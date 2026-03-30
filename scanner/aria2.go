package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// DownloadProgress represents a single aria2 download's progress.
type DownloadProgress struct {
	Path      string // file path
	Completed int64  // bytes downloaded
	Total     int64  // total bytes
	Pct       int    // 0-100
	Status    string // "active", "waiting", "paused", "complete", "removed", "error"
}

// Aria2Client polls aria2 RPC for active downloads and triggers a scan when a download completes.
type Aria2Client struct {
	rpcURL     string
	token      string
	onDone     func()                       // called when a download completes
	onProgress func(progress []DownloadProgress) // called each poll with active download progress

	connected atomic.Bool
	id        int64
}

func NewAria2Client(rpcURL, token string, onDone func(), onProgress func([]DownloadProgress)) *Aria2Client {
	// aria2 JSON-RPC over HTTP and WebSocket share the same endpoint.
	// Normalize ws:// → http://, wss:// → https:// so net/http works.
	if strings.HasPrefix(rpcURL, "ws://") {
		rpcURL = "http://" + strings.TrimPrefix(rpcURL, "ws://")
	} else if strings.HasPrefix(rpcURL, "wss://") {
		rpcURL = "https://" + strings.TrimPrefix(rpcURL, "wss://")
	}
	return &Aria2Client{
		rpcURL:     rpcURL,
		token:      token,
		onDone:     onDone,
		onProgress: onProgress,
	}
}

func (c *Aria2Client) Connected() bool {
	return c.connected.Load()
}

// Run tries to connect to aria2, then polls active/waiting tasks every 5s.
// When active count drops (i.e. a download finished), it calls onDone.
func (c *Aria2Client) Run(ctx context.Context) {
	// Try initial connection
	if err := c.ping(ctx); err != nil {
		log.Printf("aria2: connect failed: %v (will retry every 30s)", err)
	} else {
		c.connected.Store(true)
		log.Printf("aria2: connected to %s", c.rpcURL)
	}

	pollTicker := time.NewTicker(5 * time.Second)
	retryTicker := time.NewTicker(30 * time.Second)
	defer pollTicker.Stop()
	defer retryTicker.Stop()

	var lastActive int
	firstPoll := true

	for {
		select {
		case <-ctx.Done():
			return
		case <-retryTicker.C:
			if !c.connected.Load() {
				if err := c.ping(ctx); err == nil {
					c.connected.Store(true)
					log.Printf("aria2: connected to %s", c.rpcURL)
					firstPoll = true
				}
			}
		case <-pollTicker.C:
			if !c.connected.Load() {
				continue
			}
			downloads, err := c.fetchActive(ctx)
			if err != nil {
				log.Printf("aria2: poll error: %v", err)
				c.connected.Store(false)
				continue
			}
			active := 0
			for _, d := range downloads {
				if d.Status == "active" || d.Status == "waiting" {
					active++
				}
			}
			if c.onProgress != nil {
				c.onProgress(downloads)
			}
			if firstPoll {
				lastActive = active
				firstPoll = false
				log.Printf("aria2: %d active downloads", active)
				continue
			}
			if active < lastActive {
				log.Printf("aria2: download completed (%d -> %d active), triggering scan", lastActive, active)
				c.onDone()
			}
			lastActive = active
		}
	}
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int64         `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Aria2Client) nextID() int64 {
	return atomic.AddInt64(&c.id, 1)
}

func (c *Aria2Client) call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	rpcParams := make([]interface{}, 0, len(params)+1)
	if c.token != "" {
		rpcParams = append(rpcParams, "token:"+c.token)
	}
	rpcParams = append(rpcParams, params...)

	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  method,
		Params:  rpcParams,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(data, &rpcResp); err != nil {
		return nil, fmt.Errorf("invalid response: %s", string(data))
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func (c *Aria2Client) ping(ctx context.Context) error {
	result, err := c.call(ctx, "aria2.getVersion")
	if err != nil {
		return err
	}
	var ver struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(result, &ver); err != nil {
		return err
	}
	log.Printf("aria2: version %s", ver.Version)
	return nil
}

type aria2Task struct {
	Status          string `json:"status"` // active, waiting, paused, complete, removed, error
	CompletedLength string `json:"completedLength"`
	TotalLength     string `json:"totalLength"`
	Files           []struct {
		Path string `json:"path"`
	} `json:"files"`
}

func (c *Aria2Client) fetchActive(ctx context.Context) ([]DownloadProgress, error) {
	fields := []string{"status", "completedLength", "totalLength", "files"}

	activeResult, err := c.call(ctx, "aria2.tellActive", fields)
	if err != nil {
		return nil, err
	}
	var tasks []aria2Task
	if err := json.Unmarshal(activeResult, &tasks); err != nil {
		return nil, err
	}

	// Waiting tasks
	waitingResult, err := c.call(ctx, "aria2.tellWaiting", 0, 1000, fields)
	if err == nil {
		var waiting []aria2Task
		if json.Unmarshal(waitingResult, &waiting) == nil {
			tasks = append(tasks, waiting...)
		}
	}

	var out []DownloadProgress
	for _, t := range tasks {
		completed := parseInt64(t.CompletedLength)
		total := parseInt64(t.TotalLength)
		pct := 0
		if total > 0 {
			pct = int(completed * 100 / total)
		}
		status := t.Status
		if status == "" {
			status = "active"
		}
		for _, f := range t.Files {
			if f.Path != "" {
				out = append(out, DownloadProgress{
					Path:      f.Path,
					Completed: completed,
					Total:     total,
					Pct:       pct,
					Status:    status,
				})
			}
		}
	}
	return out, nil
}

// TestAria2Connection tests aria2 RPC with given URL and token, returns version or error.
func TestAria2Connection(url, token string) (string, error) {
	c := &Aria2Client{rpcURL: url, token: token}
	if strings.HasPrefix(c.rpcURL, "ws://") {
		c.rpcURL = "http://" + strings.TrimPrefix(c.rpcURL, "ws://")
	} else if strings.HasPrefix(c.rpcURL, "wss://") {
		c.rpcURL = "https://" + strings.TrimPrefix(c.rpcURL, "wss://")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := c.call(ctx, "aria2.getVersion")
	if err != nil {
		return "", err
	}
	var ver struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(result, &ver); err != nil {
		return "", err
	}
	return ver.Version, nil
}

func parseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
