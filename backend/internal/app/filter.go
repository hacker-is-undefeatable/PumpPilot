package app

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"log/slog"

	"pumppilot/internal/config"
	"pumppilot/internal/queue"
)

func runFilter(ctx context.Context, logger *slog.Logger, cfg *config.Config, in <-chan queue.TxItem, out chan<- queue.FilteredTx, blockFiltered chan<- queue.BlockFiltered) error {
	factory := common.HexToAddress(cfg.FactoryAddress)
	counts := map[uint64]int{}

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case item := <-in:
			if item.End {
				count := counts[item.BlockNumber]
				delete(counts, item.BlockNumber)
				select {
				case <-ctx.Done():
					return context.Canceled
				case blockFiltered <- queue.BlockFiltered{BlockNumber: item.BlockNumber, FilteredCount: count}:
				}
				continue
			}
			if item.Tx == nil {
				continue
			}
			if item.Tx.To == "" {
				continue
			}
			if !strings.EqualFold(item.Tx.To, factory.Hex()) {
				continue
			}
			counts[item.BlockNumber]++
			filtered := queue.FilteredTx{
				BlockNumber: item.BlockNumber,
				BlockHash:   item.BlockHash,
				Timestamp:   item.Timestamp,
				Tx:          item.Tx,
			}
			select {
			case <-ctx.Done():
				return context.Canceled
			case out <- filtered:
			}
		}
	}
}
