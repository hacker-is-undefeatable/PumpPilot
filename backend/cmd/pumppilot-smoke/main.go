package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"pumppilot/internal/config"
	"pumppilot/internal/decoder"
)

type rpcBlock struct {
	Number       string  `json:"number"`
	Timestamp    string  `json:"timestamp"`
	Transactions []rpcTx `json:"transactions"`
}

type rpcTx struct {
	Hash  string  `json:"hash"`
	From  string  `json:"from"`
	To    *string `json:"to"`
	Input string  `json:"input"`
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	block := flag.Uint64("block", 0, "scan a single block number and exit")
	backfill := flag.Uint64("backfill", 5, "number of recent blocks to scan before tailing")
	follow := flag.Bool("follow", true, "tail new blocks")
	fullInput := flag.Bool("full-input", false, "print full input data")
	pollInterval := flag.Duration("poll", 5*time.Second, "poll interval when ws is unavailable")
	debug := flag.Bool("debug", false, "enable debug logs")
	decodeInput := flag.Bool("decode-input", true, "decode input data using ABI")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	ctx := context.Background()

	var dec *decoder.Decoder
	if *decodeInput {
		d, derr := decoder.New(*cfg)
		if derr != nil {
			logger.Error("decoder init failed", "error", derr)
			os.Exit(1)
		}
		dec = d
	}

	rpcClient, httpClient, err := dialHTTP(ctx, cfg)
	if err != nil {
		logger.Error("http rpc dial failed", "error", err)
		os.Exit(1)
	}
	defer rpcClient.Close()
	defer httpClient.Close()

	if *block > 0 {
		logger.Info("smoke test single block", "block", *block, "factory", cfg.FactoryAddress)
		if err := scanBlock(ctx, logger, rpcClient, cfg, *block, *fullInput, dec); err != nil {
			logger.Error("scan block failed", "block", *block, "error", err)
			os.Exit(1)
		}
		return
	}

	head, err := fetchHead(ctx, httpClient)
	if err != nil {
		logger.Error("head fetch failed", "error", err)
		os.Exit(1)
	}

	start := head
	if *backfill > 0 && head >= *backfill {
		start = head - *backfill + 1
	}

	logger.Info("smoke test", "head", head, "start_block", start, "factory", cfg.FactoryAddress)

	last := uint64(0)
	for b := start; b <= head; b++ {
		if err := scanBlock(ctx, logger, rpcClient, cfg, b, *fullInput, dec); err != nil {
			logger.Warn("scan block failed", "block", b, "error", err)
			continue
		}
		last = b
	}

	if !*follow {
		return
	}

	if err := tailBlocks(ctx, logger, cfg, rpcClient, httpClient, last, *fullInput, *pollInterval, dec); err != nil {
		logger.Error("tail failed", "error", err)
		os.Exit(1)
	}
}

func dialHTTP(ctx context.Context, cfg *config.Config) (*rpc.Client, *ethclient.Client, error) {
	rpcClient, err := rpc.DialContext(ctx, cfg.RPC.HTTP)
	if err != nil {
		return nil, nil, err
	}
	rpcClient.SetHeader("User-Agent", "pumppilot-smoke")
	return rpcClient, ethclient.NewClient(rpcClient), nil
}

func fetchHead(ctx context.Context, client *ethclient.Client) (uint64, error) {
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return header.Number.Uint64(), nil
}

func scanBlock(ctx context.Context, logger *slog.Logger, rpcClient *rpc.Client, cfg *config.Config, blockNum uint64, fullInput bool, dec *decoder.Decoder) error {
	block, err := fetchBlockRaw(ctx, rpcClient, blockNum)
	if err != nil {
		return err
	}
	factory := strings.ToLower(cfg.FactoryAddress)
	logger.Debug("block fetched", "block", blockNum, "txs", len(block.Transactions))

	for _, tx := range block.Transactions {
		if tx.To == nil {
			continue
		}
		if strings.ToLower(*tx.To) != factory {
			continue
		}
		input := tx.Input
		if input == "" {
			input = "0x"
		}
		if !fullInput {
			input = shorten(input, 10, 8)
		}
		from := tx.From
		if from == "" {
			from = "?"
		}
		fmt.Printf("block=%d tx=%s from=%s to=%s input=%s\n", blockNum, tx.Hash, from, *tx.To, input)

		if dec != nil && tx.Input != "" && tx.Input != "0x" {
			inputBytes, derr := hexutil.Decode(tx.Input)
			if derr != nil {
				logger.Warn("decode input hex failed", "tx", tx.Hash, "error", derr)
				continue
			}
			method, merr := dec.DecodeInput(inputBytes)
			if merr != nil {
				logger.Warn("decode input failed", "tx", tx.Hash, "error", merr)
				continue
			}
			if method != nil {
				fmt.Printf("  method=%s args=%v\n", method.Name, method.Args)
			}
		}
	}
	return nil
}

func fetchBlockRaw(ctx context.Context, rpcClient *rpc.Client, blockNum uint64) (*rpcBlock, error) {
	var block rpcBlock
	if err := rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", hexutil.EncodeUint64(blockNum), true); err != nil {
		return nil, err
	}
	return &block, nil
}

func tailBlocks(ctx context.Context, logger *slog.Logger, cfg *config.Config, rpcClient *rpc.Client, httpClient *ethclient.Client, last uint64, fullInput bool, pollInterval time.Duration, dec *decoder.Decoder) error {
	wsClient, err := dialWS(ctx, cfg.RPC.WS)
	if err != nil {
		logger.Warn("ws dial failed, falling back to polling", "error", err)
		return pollLoop(ctx, logger, cfg, rpcClient, httpClient, last, fullInput, pollInterval, dec)
	}
	defer wsClient.Close()

	headers := make(chan *types.Header, 32)
	sub, err := wsClient.SubscribeNewHead(ctx, headers)
	if err != nil {
		logger.Warn("ws subscribe failed, falling back to polling", "error", err)
		return pollLoop(ctx, logger, cfg, rpcClient, httpClient, last, fullInput, pollInterval, dec)
	}
	logger.Info("tailing via ws newHeads")

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case err := <-sub.Err():
			return err
		case h := <-headers:
			if h == nil {
				continue
			}
			if h.Number.Uint64() <= last {
				continue
			}
			if err := scanBlock(ctx, logger, rpcClient, cfg, h.Number.Uint64(), fullInput, dec); err != nil {
				logger.Warn("scan block failed", "block", h.Number.Uint64(), "error", err)
				continue
			}
			last = h.Number.Uint64()
		}
	}
}

func pollLoop(ctx context.Context, logger *slog.Logger, cfg *config.Config, rpcClient *rpc.Client, httpClient *ethclient.Client, last uint64, fullInput bool, pollInterval time.Duration, dec *decoder.Decoder) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	logger.Info("tailing via polling", "interval", pollInterval.String())
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case <-ticker.C:
			head, err := fetchHead(ctx, httpClient)
			if err != nil {
				logger.Warn("head fetch failed", "error", err)
				continue
			}
			for b := last + 1; b <= head; b++ {
				if err := scanBlock(ctx, logger, rpcClient, cfg, b, fullInput, dec); err != nil {
					logger.Warn("scan block failed", "block", b, "error", err)
					continue
				}
				last = b
			}
		}
	}
}

func dialWS(ctx context.Context, wsURL string) (*ethclient.Client, error) {
	rpcClient, err := rpc.DialWebsocket(ctx, wsURL, "")
	if err != nil {
		return nil, err
	}
	return ethclient.NewClient(rpcClient), nil
}

func shorten(s string, prefix, suffix int) string {
	if prefix < 0 {
		prefix = 0
	}
	if suffix < 0 {
		suffix = 0
	}
	if len(s) <= prefix+suffix+3 {
		return s
	}
	return s[:prefix] + "..." + s[len(s)-suffix:]
}
