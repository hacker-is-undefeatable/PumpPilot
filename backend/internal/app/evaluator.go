package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"pumppilot/internal/config"
	"pumppilot/internal/queue"
)

func runEvaluator(ctx context.Context, logger *slog.Logger, cfg *config.Config, in <-chan queue.EnrichedTx) error {
	var file *os.File
	if cfg.Output.JSONLPath == "-" {
		file = os.Stdout
	} else {
		dir := filepath.Dir(cfg.Output.JSONLPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(cfg.Output.JSONLPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		file = f
		defer f.Close()
	}

	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case item := <-in:
			if err := enc.Encode(item); err != nil {
				logger.Error("output encode failed", "error", err)
			}
		}
	}
}
