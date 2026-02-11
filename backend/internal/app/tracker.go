package app

import (
	"context"
	"log/slog"

	"pumppilot/internal/checkpoint"
	"pumppilot/internal/config"
	"pumppilot/internal/queue"
)

type blockState struct {
	expected int
	done     int
}

func runTracker(ctx context.Context, logger *slog.Logger, cfg *config.Config, cp *checkpoint.Store, filtered <-chan queue.BlockFiltered, ack <-chan uint64) error {
	last := cp.Last()
	next := last + 1
	states := map[uint64]*blockState{}
	completed := map[uint64]bool{}

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case f := <-filtered:
			if f.BlockNumber <= last {
				continue
			}
			st := states[f.BlockNumber]
			if st == nil {
				st = &blockState{}
				states[f.BlockNumber] = st
			}
			st.expected = f.FilteredCount
			if st.expected == 0 && st.done == 0 {
				completed[f.BlockNumber] = true
				delete(states, f.BlockNumber)
			}
		case b := <-ack:
			if b <= last {
				continue
			}
			st := states[b]
			if st == nil {
				st = &blockState{}
				states[b] = st
			}
			st.done++
			if st.expected > 0 && st.done >= st.expected {
				completed[b] = true
				delete(states, b)
			}
		}

		for completed[next] {
			if err := cp.Save(next); err != nil {
				logger.Error("checkpoint save failed", "block", next, "error", err)
				break
			}
			logger.Info("checkpoint advanced", "block", next)
			delete(completed, next)
			last = next
			next++
		}
	}
}
