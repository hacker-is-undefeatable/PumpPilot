package app

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"log/slog"

	"pumppilot/internal/config"
	"pumppilot/internal/decoder"
	"pumppilot/internal/queue"
	"pumppilot/internal/util"
)

func runEnrichers(ctx context.Context, logger *slog.Logger, client *ethclient.Client, cfg *config.Config, dec *decoder.Decoder, in <-chan queue.FilteredTx, out chan<- queue.EnrichedTx, blockAck chan<- uint64) error {
	workers := cfg.Performance.ReceiptFetchConcurrency
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go enrichWorker(ctx, logger, client, cfg, dec, in, out, blockAck, i)
	}
	<-ctx.Done()
	return context.Canceled
}

func enrichWorker(ctx context.Context, logger *slog.Logger, client *ethclient.Client, cfg *config.Config, dec *decoder.Decoder, in <-chan queue.FilteredTx, out chan<- queue.EnrichedTx, blockAck chan<- uint64, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case item := <-in:
			if item.Tx == nil {
				continue
			}
			enriched := queue.EnrichedTx{
				Chain:           cfg.Chain,
				ChainID:         cfg.ChainID,
				BlockNumber:     item.BlockNumber,
				BlockHash:       item.BlockHash.Hex(),
				BlockTimestamp:  item.Timestamp,
				TxHash:          item.Tx.Hash,
				From:            item.Tx.From,
				To:              item.Tx.To,
				Nonce:           item.Tx.Nonce,
				ValueWei:        item.Tx.ValueWei,
				Gas:             item.Tx.Gas,
				GasPriceWei:     item.Tx.GasPriceWei,
				MaxFeePerGasWei: item.Tx.MaxFeePerGasWei,
				MaxPriorityFee:  item.Tx.MaxPriorityFeeWei,
				Input:           item.Tx.Input,
			}
			enriched.Errors = append(enriched.Errors, item.Tx.ParseErrors...)

			if item.Tx.Type > 255 {
				enriched.Type = 255
				enriched.Errors = append(enriched.Errors, "tx_type_overflow")
			} else {
				enriched.Type = uint8(item.Tx.Type)
			}

			if item.Tx.Hash == "" {
				enriched.Errors = append(enriched.Errors, "missing_tx_hash")
			} else {
				receipt, rerr := fetchReceiptWithRetry(ctx, client, cfg, common.HexToHash(item.Tx.Hash))
				if rerr != nil {
					logger.Error("receipt fetch failed", "tx", item.Tx.Hash, "error", rerr, "worker", workerID)
					enriched.Errors = append(enriched.Errors, "receipt: "+rerr.Error())
				} else if receipt != nil {
					enriched.Receipt = &queue.ReceiptInfo{
						Status:            receipt.Status,
						CumulativeGasUsed: receipt.CumulativeGasUsed,
						GasUsed:           receipt.GasUsed,
						TxIndex:           receipt.TransactionIndex,
						LogsCount:         len(receipt.Logs),
					}
					if receipt.EffectiveGasPrice != nil {
						enriched.Receipt.EffectiveGasPrice = receipt.EffectiveGasPrice.String()
					}
					if receipt.ContractAddress != (common.Address{}) {
						enriched.Receipt.ContractAddress = receipt.ContractAddress.Hex()
					}
					if cfg.Decoding.DecodeLogs {
						logs, pool, tokens, derr := dec.DecodeLogs(receipt.Logs)
						if derr != nil {
							enriched.Errors = append(enriched.Errors, "decode_logs: "+derr.Error())
						} else {
							enriched.DecodedLogs = logs
							enriched.PoolAddress = pool
							enriched.TokenAddresses = tokens
						}
					}
				}
			}

			if cfg.Decoding.DecodeInput && item.Tx.Input != "" {
				inputBytes, derr := hexutil.Decode(item.Tx.Input)
				if derr != nil {
					enriched.Errors = append(enriched.Errors, "decode_input_hex: "+derr.Error())
				} else {
					method, err := dec.DecodeInput(inputBytes)
					if err != nil {
						enriched.Errors = append(enriched.Errors, "decode_input: "+err.Error())
					} else {
						enriched.Method = method
					}
				}
			}

			select {
			case <-ctx.Done():
				return
			case out <- enriched:
			}

			select {
			case <-ctx.Done():
				return
			case blockAck <- item.BlockNumber:
			}
		}
	}
}

func fetchReceiptWithRetry(ctx context.Context, client *ethclient.Client, cfg *config.Config, hash common.Hash) (*types.Receipt, error) {
	var receipt *types.Receipt
	err := util.Retry(ctx, cfg.Performance.RetryMax, cfg.Performance.RetryBackoff.Duration, func() error {
		ctxTimeout, cancel := withTimeout(ctx, cfg.Performance.RequestTimeout.Duration)
		defer cancel()
		r, err := client.TransactionReceipt(ctxTimeout, hash)
		if err != nil {
			return err
		}
		receipt = r
		return nil
	})
	return receipt, err
}
