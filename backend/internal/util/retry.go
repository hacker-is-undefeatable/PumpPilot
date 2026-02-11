package util

import (
	"context"
	"time"
)

func Retry(ctx context.Context, max int, backoff time.Duration, fn func() error) error {
	var err error
	for attempt := 0; attempt <= max; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = fn()
		if err == nil {
			return nil
		}
		if attempt == max {
			break
		}
		wait := backoff * time.Duration(1<<attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return err
}
