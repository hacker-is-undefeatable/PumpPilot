package txbuilder

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	selectorBuy     = mustSelector("0xd6febde8")
	selectorSell    = mustSelector("0xd3c9727c")
	selectorApprove = mustSelector("0x095ea7b3")
)

type FeeParams struct {
	MaxFeePerGas         *big.Int
	MaxPriorityFeePerGas *big.Int
}

type BuildParams struct {
	Nonce    uint64
	GasLimit uint64
	Fee      FeeParams
}

type Builder struct {
	ChainID         *big.Int
	DefaultDeadline time.Duration
	now             func() time.Time
}

func NewBuilder(chainID *big.Int, defaultDeadline time.Duration) *Builder {
	if defaultDeadline <= 0 {
		defaultDeadline = 120 * time.Second
	}
	return &Builder{
		ChainID:         new(big.Int).Set(chainID),
		DefaultDeadline: defaultDeadline,
		now:             time.Now,
	}
}

func NewBuilderWithClock(chainID *big.Int, defaultDeadline time.Duration, now func() time.Time) *Builder {
	b := NewBuilder(chainID, defaultDeadline)
	if now != nil {
		b.now = now
	}
	return b
}

func (b *Builder) BuildBuyTx(pair common.Address, ethInWei, minTokensOut *big.Int, p BuildParams) (*types.Transaction, error) {
	if ethInWei == nil || minTokensOut == nil {
		return nil, errors.New("ethInWei and minTokensOut are required")
	}
	deadline := uint64(b.now().Add(b.DefaultDeadline).Unix())
	data, err := buildBuyData(minTokensOut, deadline)
	if err != nil {
		return nil, err
	}
	return buildDynamicTx(b.ChainID, pair, ethInWei, data, p)
}

func (b *Builder) BuildSellTx(pair common.Address, tokenAmountIn, minRefundWei *big.Int, p BuildParams) (*types.Transaction, error) {
	if tokenAmountIn == nil || minRefundWei == nil {
		return nil, errors.New("tokenAmountIn and minRefundWei are required")
	}
	deadline := uint64(b.now().Add(b.DefaultDeadline).Unix())
	data, err := buildSellData(tokenAmountIn, minRefundWei, deadline)
	if err != nil {
		return nil, err
	}
	return buildDynamicTx(b.ChainID, pair, big.NewInt(0), data, p)
}

func (b *Builder) BuildApproveTx(token common.Address, spender common.Address, amount *big.Int, p BuildParams) (*types.Transaction, error) {
	if amount == nil {
		return nil, errors.New("amount is required")
	}
	data, err := buildApproveData(spender, amount)
	if err != nil {
		return nil, err
	}
	return buildDynamicTx(b.ChainID, token, big.NewInt(0), data, p)
}

func buildBuyData(minTokensOut *big.Int, deadline uint64) ([]byte, error) {
	arg0, err := encodeUint256(minTokensOut)
	if err != nil {
		return nil, fmt.Errorf("minTokensOut: %w", err)
	}
	arg1, err := encodeUint256(new(big.Int).SetUint64(deadline))
	if err != nil {
		return nil, fmt.Errorf("deadline: %w", err)
	}
	data := append([]byte{}, selectorBuy...)
	data = append(data, arg0...)
	data = append(data, arg1...)
	return data, nil
}

func buildSellData(tokenAmountIn, minRefundWei *big.Int, deadline uint64) ([]byte, error) {
	arg0, err := encodeUint256(tokenAmountIn)
	if err != nil {
		return nil, fmt.Errorf("tokenAmountIn: %w", err)
	}
	arg1, err := encodeUint256(minRefundWei)
	if err != nil {
		return nil, fmt.Errorf("minRefundWei: %w", err)
	}
	arg2, err := encodeUint256(new(big.Int).SetUint64(deadline))
	if err != nil {
		return nil, fmt.Errorf("deadline: %w", err)
	}
	data := append([]byte{}, selectorSell...)
	data = append(data, arg0...)
	data = append(data, arg1...)
	data = append(data, arg2...)
	return data, nil
}

func buildApproveData(spender common.Address, amount *big.Int) ([]byte, error) {
	arg0 := encodeAddress(spender)
	arg1, err := encodeUint256(amount)
	if err != nil {
		return nil, fmt.Errorf("amount: %w", err)
	}
	data := append([]byte{}, selectorApprove...)
	data = append(data, arg0...)
	data = append(data, arg1...)
	return data, nil
}

func buildDynamicTx(chainID *big.Int, to common.Address, value *big.Int, data []byte, p BuildParams) (*types.Transaction, error) {
	if chainID == nil {
		return nil, errors.New("chainID is required")
	}
	if value == nil {
		return nil, errors.New("value is required")
	}
	if p.GasLimit == 0 {
		return nil, errors.New("gasLimit is required")
	}
	if p.Fee.MaxFeePerGas == nil || p.Fee.MaxPriorityFeePerGas == nil {
		return nil, errors.New("maxFeePerGas and maxPriorityFeePerGas are required")
	}
	if p.Fee.MaxFeePerGas.Sign() < 0 || p.Fee.MaxPriorityFeePerGas.Sign() < 0 {
		return nil, errors.New("fee values must be non-negative")
	}
	if value.Sign() < 0 {
		return nil, errors.New("value must be non-negative")
	}
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     p.Nonce,
		Gas:       p.GasLimit,
		GasFeeCap: p.Fee.MaxFeePerGas,
		GasTipCap: p.Fee.MaxPriorityFeePerGas,
		To:        &to,
		Value:     value,
		Data:      data,
	}), nil
}

func encodeUint256(v *big.Int) ([]byte, error) {
	if v == nil {
		return nil, errors.New("value is nil")
	}
	if v.Sign() < 0 {
		return nil, errors.New("value must be non-negative")
	}
	return common.LeftPadBytes(v.Bytes(), 32), nil
}

func encodeAddress(addr common.Address) []byte {
	return common.LeftPadBytes(addr.Bytes(), 32)
}

func mustSelector(hex string) []byte {
	b, err := hexutil.Decode(hex)
	if err != nil {
		panic(err)
	}
	if len(b) != 4 {
		panic("selector must be 4 bytes")
	}
	return b
}
