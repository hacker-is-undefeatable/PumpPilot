package txbuilder

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type AutoBuilderConfig struct {
	GasLimitMultiplier float64
}

type AutoBuilder struct {
	builder *Builder
	client  ChainClient
	oracle  *FeeOracle
	cfg     AutoBuilderConfig
	nonce   NonceProvider
}

func NewAutoBuilder(builder *Builder, client ChainClient, oracle *FeeOracle, cfg AutoBuilderConfig) *AutoBuilder {
	if cfg.GasLimitMultiplier <= 0 {
		cfg.GasLimitMultiplier = 1.2
	}
	return &AutoBuilder{builder: builder, client: client, oracle: oracle, cfg: cfg}
}

func (a *AutoBuilder) SetNonceProvider(provider NonceProvider) {
	a.nonce = provider
}

func (a *AutoBuilder) Start(ctx context.Context) {
	if a.oracle == nil {
		return
	}
	go a.oracle.Start(ctx)
}

func (a *AutoBuilder) BuildBuyTx(ctx context.Context, from common.Address, pair common.Address, ethInWei, minTokensOut *big.Int) (*types.Transaction, error) {
	if a.builder == nil || a.client == nil {
		return nil, errors.New("builder and client are required")
	}
	if ethInWei == nil || minTokensOut == nil {
		return nil, errors.New("ethInWei and minTokensOut are required")
	}
	deadline := a.builder.nextDeadline()
	data, err := buildBuyData(minTokensOut, deadline)
	if err != nil {
		return nil, err
	}
	return a.buildTx(ctx, from, pair, ethInWei, data)
}

func (a *AutoBuilder) BuildSellTx(ctx context.Context, from common.Address, pair common.Address, tokenAmountIn, minRefundWei *big.Int) (*types.Transaction, error) {
	if a.builder == nil || a.client == nil {
		return nil, errors.New("builder and client are required")
	}
	if tokenAmountIn == nil || minRefundWei == nil {
		return nil, errors.New("tokenAmountIn and minRefundWei are required")
	}
	deadline := a.builder.nextDeadline()
	data, err := buildSellData(tokenAmountIn, minRefundWei, deadline)
	if err != nil {
		return nil, err
	}
	return a.buildTx(ctx, from, pair, big.NewInt(0), data)
}

func (a *AutoBuilder) BuildApproveTx(ctx context.Context, from common.Address, token common.Address, spender common.Address, amount *big.Int) (*types.Transaction, error) {
	if a.builder == nil || a.client == nil {
		return nil, errors.New("builder and client are required")
	}
	if amount == nil {
		return nil, errors.New("amount is required")
	}
	data, err := buildApproveData(spender, amount)
	if err != nil {
		return nil, err
	}
	return a.buildTx(ctx, from, token, big.NewInt(0), data)
}

func (a *AutoBuilder) buildTx(ctx context.Context, from common.Address, to common.Address, value *big.Int, data []byte) (*types.Transaction, error) {
	fees, err := a.fees(ctx)
	if err != nil {
		return nil, err
	}
	nonce, err := a.nextNonce(ctx, from)
	if err != nil {
		return nil, err
	}
	gasLimit, err := a.estimateGas(ctx, from, to, value, data, fees)
	if err != nil {
		return nil, err
	}
	params := BuildParams{
		Nonce:    nonce,
		GasLimit: gasLimit,
		Fee:      fees,
	}
	return buildDynamicTx(a.builder.ChainID, to, value, data, params)
}

func (a *AutoBuilder) nextNonce(ctx context.Context, from common.Address) (uint64, error) {
	if a.nonce != nil {
		return a.nonce.Next(ctx, from)
	}
	return a.client.PendingNonceAt(ctx, from)
}

func (a *AutoBuilder) ResetNonce(from common.Address) {
	if a.nonce != nil {
		a.nonce.Reset(from)
	}
}

func (a *AutoBuilder) ChainID() *big.Int {
	if a.builder == nil || a.builder.ChainID == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(a.builder.ChainID)
}

func (a *AutoBuilder) fees(ctx context.Context) (FeeParams, error) {
	if a.oracle == nil {
		return FeeParams{}, errors.New("fee oracle is not configured")
	}
	return a.oracle.Fees(ctx)
}

func (a *AutoBuilder) estimateGas(ctx context.Context, from common.Address, to common.Address, value *big.Int, data []byte, fees FeeParams) (uint64, error) {
	msg := ethereum.CallMsg{
		From:      from,
		To:        &to,
		Value:     value,
		Data:      data,
		GasFeeCap: fees.MaxFeePerGas,
		GasTipCap: fees.MaxPriorityFeePerGas,
	}
	gas, err := a.client.EstimateGas(ctx, msg)
	if err != nil {
		return 0, &EstimateGasError{Err: err, CallMsg: msg}
	}
	gas = applyGasMultiplier(gas, a.cfg.GasLimitMultiplier)
	return gas, nil
}

func applyGasMultiplier(gas uint64, mult float64) uint64 {
	if mult <= 0 {
		return gas
	}
	adjusted := uint64(float64(gas) * mult)
	if adjusted < gas {
		return gas
	}
	return adjusted
}

func (b *Builder) nextDeadline() uint64 {
	return uint64(b.now().Add(b.DefaultDeadline).Unix())
}

func GweiToWei(gwei float64) (*big.Int, error) {
	if gwei < 0 {
		return nil, errors.New("gwei must be non-negative")
	}
	v := new(big.Rat).SetFloat64(gwei)
	v.Mul(v, new(big.Rat).SetInt(big.NewInt(1_000_000_000)))
	out := new(big.Int)
	out.Div(v.Num(), v.Denom())
	return out, nil
}

func ParseHexBig(hexValue string) (*big.Int, error) {
	return decodeHexBig(hexValue)
}
