package app

import (
	"context"
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"log/slog"

	"pumppilot/internal/checkpoint"
	"pumppilot/internal/config"
)

func runReader(ctx context.Context, logger *slog.Logger, httpClient *ethclient.Client, cfg *config.Config, cp *checkpoint.Store, out chan<- uint64) error {
	lastProcessed := cp.Last()

	head, err := fetchHead(ctx, httpClient, cfg)
	if err != nil {
		return err
	}

	startBlock, startLatest, err := cfg.StartBlockNumber()
	if err != nil {
		return err
	}
	if lastProcessed > 0 {
		startBlock = lastProcessed + 1
		startLatest = false
	}

	if startLatest {
		if head > cfg.Ingestion.Confirmations {
			startBlock = head - cfg.Ingestion.Confirmations
		} else {
			startBlock = 0
		}
	}
	if cfg.Ingestion.ReorgReplayDepth > 0 {
		if startBlock > cfg.Ingestion.ReorgReplayDepth {
			startBlock -= cfg.Ingestion.ReorgReplayDepth
		} else {
			startBlock = 0
		}
	}

	logger.Info("reader start",
		"head", head,
		"start_block", startBlock,
		"confirmations", cfg.Ingestion.Confirmations,
		"reorg_replay_depth", cfg.Ingestion.ReorgReplayDepth,
	)

	headCh := make(chan uint64, 4)
	go pollHeads(ctx, logger, httpClient, cfg, headCh)
	go subscribeHeads(ctx, logger, cfg.RPC.WS, headCh)

	nextBlock := startBlock
	currentHead := head

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case h := <-headCh:
			if h > currentHead {
				currentHead = h
			}
		default:
			// no new head, just fall through
		}

		ready := int64(currentHead) - int64(cfg.Ingestion.Confirmations)
		if ready < 0 {
			ready = 0
		}
		for nextBlock <= uint64(ready) {
			select {
			case <-ctx.Done():
				return context.Canceled
			case out <- nextBlock:
				nextBlock++
			}
		}

		select {
		case <-ctx.Done():
			return context.Canceled
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func fetchHead(ctx context.Context, client *ethclient.Client, cfg *config.Config) (uint64, error) {
	ctxTimeout, cancel := withTimeout(ctx, cfg.Performance.RequestTimeout.Duration)
	defer cancel()
	header, err := client.HeaderByNumber(ctxTimeout, nil)
	if err != nil {
		return 0, err
	}
	return header.Number.Uint64(), nil
}

func pollHeads(ctx context.Context, logger *slog.Logger, client *ethclient.Client, cfg *config.Config, out chan<- uint64) {
	ticker := time.NewTicker(cfg.Ingestion.PollInterval.Duration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			head, err := fetchHead(ctx, client, cfg)
			if err != nil {
				logger.Warn("poll head failed", "error", err)
				continue
			}
			select {
			case out <- head:
			default:
			}
		}
	}
}

func subscribeHeads(ctx context.Context, logger *slog.Logger, wsURL string, out chan<- uint64) {
	backoff := 500 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return
		}
		rpcClient, err := rpc.DialWebsocket(ctx, wsURL, "")
		if err != nil {
			logger.Warn("ws dial failed", "error", err)
			wait(ctx, backoff)
			backoff = minDuration(backoff*2, 10*time.Second)
			continue
		}
		client := ethclient.NewClient(rpcClient)
		headers := make(chan *types.Header, 16)
		sub, err := client.SubscribeNewHead(ctx, headers)
		if err != nil {
			logger.Warn("ws subscribe failed", "error", err)
			client.Close()
			wait(ctx, backoff)
			backoff = minDuration(backoff*2, 10*time.Second)
			continue
		}
		logger.Info("ws subscribed to newHeads")
		backoff = 500 * time.Millisecond

		for {
			select {
			case <-ctx.Done():
				sub.Unsubscribe()
				client.Close()
				return
			case err := <-sub.Err():
				if err != nil && !errors.Is(err, context.Canceled) {
					logger.Warn("ws subscription error", "error", err)
				}
				sub.Unsubscribe()
				client.Close()
				wait(ctx, backoff)
				backoff = minDuration(backoff*2, 10*time.Second)
				goto reconnect
			case h := <-headers:
				if h == nil {
					continue
				}
				select {
				case out <- h.Number.Uint64():
				default:
				}
			}
		}
	reconnect:
		continue
	}
}

func wait(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
