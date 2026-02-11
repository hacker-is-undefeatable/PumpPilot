package txbuilder

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"time"
)

type FeeOracleConfig struct {
	RefreshInterval   time.Duration
	MaxFeeMultiplier  float64
	MinPriorityFeeWei *big.Int
}

type FeeOracle struct {
	client ChainClient
	cfg    FeeOracleConfig

	mu       sync.RWMutex
	baseFee  *big.Int
	tipCap   *big.Int
	lastSync time.Time
}

func NewFeeOracle(client ChainClient, cfg FeeOracleConfig) *FeeOracle {
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 5 * time.Second
	}
	if cfg.MaxFeeMultiplier <= 0 {
		cfg.MaxFeeMultiplier = 2.0
	}
	return &FeeOracle{client: client, cfg: cfg}
}

func (o *FeeOracle) Start(ctx context.Context) {
	ticker := time.NewTicker(o.cfg.RefreshInterval)
	defer ticker.Stop()

	_ = o.Refresh(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = o.Refresh(ctx)
		}
	}
}

func (o *FeeOracle) Refresh(ctx context.Context) error {
	baseFee, err := o.fetchBaseFee(ctx)
	if err != nil {
		return err
	}
	tip, err := o.client.SuggestGasTipCap(ctx)
	if err != nil {
		return err
	}
	if o.cfg.MinPriorityFeeWei != nil && tip.Cmp(o.cfg.MinPriorityFeeWei) < 0 {
		tip = new(big.Int).Set(o.cfg.MinPriorityFeeWei)
	}
	o.mu.Lock()
	o.baseFee = baseFee
	o.tipCap = tip
	o.lastSync = time.Now()
	o.mu.Unlock()
	return nil
}

func (o *FeeOracle) Fees(ctx context.Context) (FeeParams, error) {
	baseFee, tip, err := o.snapshot(ctx)
	if err != nil {
		return FeeParams{}, err
	}
	maxFee := new(big.Int).Add(mulFloat(baseFee, o.cfg.MaxFeeMultiplier), tip)
	return FeeParams{
		MaxFeePerGas:         maxFee,
		MaxPriorityFeePerGas: tip,
	}, nil
}

func (o *FeeOracle) snapshot(ctx context.Context) (*big.Int, *big.Int, error) {
	o.mu.RLock()
	baseFee := o.baseFee
	tip := o.tipCap
	o.mu.RUnlock()

	if baseFee != nil && tip != nil {
		return new(big.Int).Set(baseFee), new(big.Int).Set(tip), nil
	}
	if err := o.Refresh(ctx); err != nil {
		return nil, nil, err
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.baseFee == nil || o.tipCap == nil {
		return nil, nil, errors.New("fee oracle unavailable")
	}
	return new(big.Int).Set(o.baseFee), new(big.Int).Set(o.tipCap), nil
}

func (o *FeeOracle) fetchBaseFee(ctx context.Context) (*big.Int, error) {
	header, err := o.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	if header.BaseFee != nil {
		return new(big.Int).Set(header.BaseFee), nil
	}
	// Fallback for non-EIP-1559 chains: approximate using gas price.
	price, err := o.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}
	return price, nil
}

func mulFloat(v *big.Int, f float64) *big.Int {
	if v == nil {
		return big.NewInt(0)
	}
	if f == 1.0 {
		return new(big.Int).Set(v)
	}
	r := new(big.Rat).SetInt(v)
	r.Mul(r, new(big.Rat).SetFloat64(f))
	out := new(big.Int)
	out.Div(r.Num(), r.Denom())
	return out
}
