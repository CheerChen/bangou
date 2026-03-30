package checker

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/CheerChen/bangou/committed"
)

func CheckLink(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Run(ctx context.Context, s committed.Store, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			check(ctx, s)
		}
	}
}

func check(ctx context.Context, s committed.Store) {
	outputs, err := s.ListAllOutputs(ctx)
	if err != nil {
		log.Printf("checker list outputs: %v", err)
		return
	}
	for _, o := range outputs {
		alive := CheckLink(o.LinkPath)
		if alive == o.Alive {
			continue
		}
		if err := s.SetOutputAlive(ctx, o.ID, alive); err != nil {
			log.Printf("checker update output(%d): %v", o.ID, err)
			continue
		}
		if !alive {
			log.Printf("orphaned: %s -> %s", o.Number, o.LinkPath)
		}
	}
}
