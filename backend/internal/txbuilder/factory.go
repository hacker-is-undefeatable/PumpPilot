package txbuilder

import (
	"math/big"
	"time"

	"pumppilot/internal/config"
)

func NewOracleFromConfig(client ChainClient, cfg *config.Config) (*FeeOracle, error) {
	minTipWei, err := GweiToWei(cfg.Tx.MinPriorityFeeGwei)
	if err != nil {
		return nil, err
	}
	oracleCfg := FeeOracleConfig{
		RefreshInterval:   time.Duration(cfg.Tx.FeeRefreshSeconds) * time.Second,
		MaxFeeMultiplier:  cfg.Tx.MaxFeeMultiplier,
		MinPriorityFeeWei: minTipWei,
	}
	return NewFeeOracle(client, oracleCfg), nil
}

func NewAutoBuilderFromConfig(client ChainClient, cfg *config.Config) (*AutoBuilder, error) {
	builder := NewBuilder(
		bigInt(cfg.ChainID),
		time.Duration(cfg.Tx.DefaultDeadlineSeconds)*time.Second,
	)
	oracle, err := NewOracleFromConfig(client, cfg)
	if err != nil {
		return nil, err
	}
	auto := NewAutoBuilder(builder, client, oracle, AutoBuilderConfig{
		GasLimitMultiplier: cfg.Tx.GasLimitMultiplier,
	})
	auto.SetNonceProvider(NewNonceManager(client))
	return auto, nil
}

func bigInt(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}
