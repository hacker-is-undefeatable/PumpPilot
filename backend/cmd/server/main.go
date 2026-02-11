package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"pumppilot/internal/api"
	"pumppilot/internal/config"
	"pumppilot/internal/keys"
	"pumppilot/internal/trade"
	"pumppilot/internal/txbuilder"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	passphrase := os.Getenv(cfg.KeyStore.PassphraseEnv)
	if passphrase == "" {
		logger.Warn("keystore passphrase env is empty", "env", cfg.KeyStore.PassphraseEnv)
	}
	keysManager, err := keys.NewManager(cfg.KeyStore.Dir, passphrase)
	if err != nil {
		logger.Error("keystore init failed", "error", err)
		os.Exit(1)
	}

	rpcClient, err := rpc.DialHTTP(cfg.RPC.HTTP)
	if err != nil {
		logger.Error("rpc dial failed", "error", err)
		os.Exit(1)
	}
	defer rpcClient.Close()
	rpcClient.SetHeader("User-Agent", "pumppilot-api")
	ethClient := ethclient.NewClient(rpcClient)
	defer ethClient.Close()

	auto, err := txbuilder.NewAutoBuilderFromConfig(ethClient, cfg)
	if err != nil {
		logger.Error("auto builder init failed", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	auto.Start(ctx)

	tradeSvc := trade.NewService(auto, ethClient, rpcClient, keysManager)
	server := api.NewServer(cfg, logger, keysManager, tradeSvc, rpcClient, ethClient)

	logger.Info("api starting", "listen", cfg.API.Listen)
	if err := server.Start(ctx); err != nil && err.Error() != "http: Server closed" {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
