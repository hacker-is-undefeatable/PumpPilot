package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"log/slog"

	"pumppilot/internal/config"
	"pumppilot/internal/queue"
	"pumppilot/internal/util"
)

type rpcBlock struct {
	Number       string  `json:"number"`
	Hash         string  `json:"hash"`
	Timestamp    string  `json:"timestamp"`
	Transactions []rpcTx `json:"transactions"`
}

type rpcTx struct {
	Hash                 string  `json:"hash"`
	From                 string  `json:"from"`
	To                   *string `json:"to"`
	Nonce                string  `json:"nonce"`
	Value                string  `json:"value"`
	Gas                  string  `json:"gas"`
	GasPrice             string  `json:"gasPrice"`
	MaxFeePerGas         string  `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string  `json:"maxPriorityFeePerGas"`
	Type                 string  `json:"type"`
	Input                string  `json:"input"`
}

func runBlockFetchers(ctx context.Context, logger *slog.Logger, rpcClient *rpc.Client, cfg *config.Config, in <-chan uint64, out chan<- queue.TxItem) error {
	workers := cfg.Performance.BlockFetchConcurrency
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go blockFetcher(ctx, logger, rpcClient, cfg, in, out, i)
	}
	<-ctx.Done()
	return context.Canceled
}

func blockFetcher(ctx context.Context, logger *slog.Logger, rpcClient *rpc.Client, cfg *config.Config, in <-chan uint64, out chan<- queue.TxItem, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case num := <-in:
			block, err := fetchBlockWithRetry(ctx, rpcClient, cfg, num)
			if err != nil {
				logger.Error("fetch block failed", "block", num, "error", err, "worker", workerID)
				continue
			}
			pushBlock(ctx, logger, block, out, num)
		}
	}
}

func fetchBlockWithRetry(ctx context.Context, rpcClient *rpc.Client, cfg *config.Config, num uint64) (*rpcBlock, error) {
	var block rpcBlock
	err := util.Retry(ctx, cfg.Performance.RetryMax, cfg.Performance.RetryBackoff.Duration, func() error {
		ctxTimeout, cancel := withTimeout(ctx, cfg.Performance.RequestTimeout.Duration)
		defer cancel()
		if err := rpcClient.CallContext(ctxTimeout, &block, "eth_getBlockByNumber", hexutil.EncodeUint64(num), true); err != nil {
			return err
		}
		return nil
	})
	return &block, err
}

func pushBlock(ctx context.Context, logger *slog.Logger, block *rpcBlock, out chan<- queue.TxItem, requested uint64) {
	blockNumber := requested
	blockHash := common.HexToHash(block.Hash)
	blockTime := uint64(0)

	if block.Number != "" {
		if n, err := hexutil.DecodeUint64(block.Number); err == nil {
			blockNumber = n
		} else {
			logger.Warn("block number decode failed", "block", requested, "value", block.Number, "error", err)
		}
	}
	if block.Timestamp != "" {
		if ts, err := hexutil.DecodeUint64(block.Timestamp); err == nil {
			blockTime = ts
		} else {
			logger.Warn("block timestamp decode failed", "block", requested, "value", block.Timestamp, "error", err)
		}
	}

	logger.Debug("block fetched", "block", blockNumber, "txs", len(block.Transactions))

	for _, tx := range block.Transactions {
		raw, ok := parseRawTx(tx, logger, blockNumber)
		if !ok {
			continue
		}
		item := queue.TxItem{
			BlockNumber: blockNumber,
			BlockHash:   blockHash,
			Timestamp:   blockTime,
			Tx:          raw,
		}
		select {
		case <-ctx.Done():
			return
		case out <- item:
		}
	}

	end := queue.TxItem{
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		Timestamp:   blockTime,
		End:         true,
	}
	select {
	case <-ctx.Done():
		return
	case out <- end:
	}
}

func parseRawTx(tx rpcTx, logger *slog.Logger, blockNumber uint64) (*queue.RawTx, bool) {
	errs := make([]string, 0)

	toAddr := ""
	if tx.To != nil {
		toAddr = strings.TrimSpace(*tx.To)
	}
	input := strings.TrimSpace(tx.Input)
	if input == "" {
		input = "0x"
	}

	nonce := decodeUint64(tx.Nonce, &errs, "nonce")
	gas := decodeUint64(tx.Gas, &errs, "gas")
	txType := decodeUint64(tx.Type, &errs, "type")

	value := decodeBigString(tx.Value, &errs, "value", true)
	gasPrice := decodeBigString(tx.GasPrice, &errs, "gasPrice", false)
	maxFee := decodeBigString(tx.MaxFeePerGas, &errs, "maxFeePerGas", false)
	maxPriority := decodeBigString(tx.MaxPriorityFeePerGas, &errs, "maxPriorityFeePerGas", false)

	if tx.Hash == "" {
		logger.Warn("tx missing hash", "block", blockNumber)
		return nil, false
	}

	return &queue.RawTx{
		Hash:              tx.Hash,
		From:              tx.From,
		To:                toAddr,
		Nonce:             nonce,
		ValueWei:          value,
		Gas:               gas,
		GasPriceWei:       gasPrice,
		MaxFeePerGasWei:   maxFee,
		MaxPriorityFeeWei: maxPriority,
		Type:              txType,
		Input:             input,
		ParseErrors:       errs,
	}, true
}

func decodeUint64(value string, errs *[]string, field string) uint64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	v, err := hexutil.DecodeUint64(value)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: %v", field, err))
		return 0
	}
	return v
}

func decodeBigString(value string, errs *[]string, field string, allowZero bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		if allowZero {
			return "0"
		}
		return ""
	}
	v, err := hexutil.DecodeBig(value)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: %v", field, err))
		if allowZero {
			return "0"
		}
		return ""
	}
	return v.String()
}
