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

// Aria2Client polls aria2 RPC for active downloads and triggers a scan when a download completes.
type Aria2Client struct {
	rpcURL string
	token  string
	onDone func() // called when a download completes

	connected atomic.Bool
	id        int64
}

func NewAria2Client(rpcURL, token string, onDone func()) *Aria2Client {
	// aria2 JSON-RPC over HTTP and WebSocket share the same endpoint.
	// Normalize ws:// → http://, wss:// → https:// so net/http works.
	if strings.HasPrefix(rpcURL, "ws://") {
		rpcURL = "http://" + strings.TrimPrefix(rpcURL, "ws://")
	} else if strings.HasPrefix(rpcURL, "wss://") {
		rpcURL = "https://" + strings.TrimPrefix(rpcURL, "wss://")
	}
	return &Aria2Client{
		rpcURL: rpcURL,
		token:  token,
		onDone: onDone,
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
			active, err := c.countActive(ctx)
			if err != nil {
				log.Printf("aria2: poll error: %v", err)
				c.connected.Store(false)
				continue
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

func (c *Aria2Client) countActive(ctx context.Context) (int, error) {
	activeResult, err := c.call(ctx, "aria2.tellActive", []string{"gid"})
	if err != nil {
		return 0, err
	}
	var active []json.RawMessage
	if err := json.Unmarshal(activeResult, &active); err != nil {
		return 0, err
	}

	waitingResult, err := c.call(ctx, "aria2.tellWaiting", 0, 1000, []string{"gid"})
	if err != nil {
		return len(active), nil // waiting query failed, just return active count
	}
	var waiting []json.RawMessage
	if err := json.Unmarshal(waitingResult, &waiting); err != nil {
		return len(active), nil
	}

	return len(active) + len(waiting), nil
}
