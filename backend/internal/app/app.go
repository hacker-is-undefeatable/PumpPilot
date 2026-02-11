package app

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/sync/errgroup"
	"log/slog"

	"pumppilot/internal/checkpoint"
	"pumppilot/internal/config"
	"pumppilot/internal/decoder"
	"pumppilot/internal/queue"
)

type App struct {
	cfg    *config.Config
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *App {
	return &App{cfg: cfg, logger: logger}
}

func (a *App) Run(ctx context.Context) error {
	rpcClient, httpClient, err := dialHTTP(a.cfg, a.logger)
	if err != nil {
		return err
	}
	defer rpcClient.Close()
	defer httpClient.Close()

	dec, err := decoder.New(*a.cfg)
	if err != nil {
		return err
	}

	cp := checkpoint.New(a.cfg.Checkpoint.Path)
	_, _ = cp.Load()

	blockNumCh := make(chan uint64, a.cfg.Performance.QueueSize)
	queue1 := make(chan queue.TxItem, a.cfg.Performance.QueueSize)
	queue2 := make(chan queue.FilteredTx, a.cfg.Performance.QueueSize)
	queue3 := make(chan queue.EnrichedTx, a.cfg.Performance.QueueSize)
	blockFilteredCh := make(chan queue.BlockFiltered, a.cfg.Performance.QueueSize)
	blockAckCh := make(chan uint64, a.cfg.Performance.QueueSize)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return runReader(gctx, a.logger, httpClient, a.cfg, cp, blockNumCh)
	})

	g.Go(func() error {
		return runBlockFetchers(gctx, a.logger, rpcClient, a.cfg, blockNumCh, queue1)
	})

	g.Go(func() error {
		return runFilter(gctx, a.logger, a.cfg, queue1, queue2, blockFilteredCh)
	})

	g.Go(func() error {
		return runEnrichers(gctx, a.logger, httpClient, a.cfg, dec, queue2, queue3, blockAckCh)
	})

	g.Go(func() error {
		return runEvaluator(gctx, a.logger, a.cfg, queue3)
	})

	g.Go(func() error {
		return runTracker(gctx, a.logger, a.cfg, cp, blockFilteredCh, blockAckCh)
	})

	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
	return nil
}

func dialHTTP(cfg *config.Config, logger *slog.Logger) (*rpc.Client, *ethclient.Client, error) {
	httpClient := &http.Client{
		Timeout: cfg.Performance.RequestTimeout.Duration,
	}
	rpcClient, err := rpc.DialHTTPWithClient(cfg.RPC.HTTP, httpClient)
	if err != nil {
		return nil, nil, err
	}
	rpcClient.SetHeader("User-Agent", "pumppilot")
	logger.Info("rpc http connected")
	return rpcClient, ethclient.NewClient(rpcClient), nil
}

func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}
