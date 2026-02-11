package txbuilder

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestBuildBuyTxCalldata(t *testing.T) {
	pair := common.HexToAddress("0x1111111111111111111111111111111111111111")
	now := func() time.Time { return time.Unix(1700000000, 0) }
	builder := NewBuilderWithClock(big.NewInt(8453), 120*time.Second, now)

	params := BuildParams{
		Nonce:    7,
		GasLimit: 210000,
		Fee: FeeParams{
			MaxFeePerGas:         big.NewInt(1000000000),
			MaxPriorityFeePerGas: big.NewInt(200000000),
		},
	}

	minTokensOut := big.NewInt(1000)
	deadline := uint64(1700000120)

	tx, err := builder.BuildBuyTx(pair, big.NewInt(1), minTokensOut, params)
	if err != nil {
		t.Fatalf("BuildBuyTx error: %v", err)
	}
	data := hexutil.Encode(tx.Data())
	expected := "0xd6febde8" + hex32(minTokensOut) + hex32(new(big.Int).SetUint64(deadline))
	if data != expected {
		t.Fatalf("unexpected calldata\nexpected=%s\nactual=%s", expected, data)
	}
}

func TestBuildSellTxCalldata(t *testing.T) {
	pair := common.HexToAddress("0x2222222222222222222222222222222222222222")
	now := func() time.Time { return time.Unix(1700000000, 0) }
	builder := NewBuilderWithClock(big.NewInt(8453), 60*time.Second, now)

	params := BuildParams{
		Nonce:    1,
		GasLimit: 210000,
		Fee: FeeParams{
			MaxFeePerGas:         big.NewInt(1000000000),
			MaxPriorityFeePerGas: big.NewInt(200000000),
		},
	}

	tokenAmountIn := big.NewInt(5000)
	minRefundWei := big.NewInt(123456)
	deadline := uint64(1700000060)

	tx, err := builder.BuildSellTx(pair, tokenAmountIn, minRefundWei, params)
	if err != nil {
		t.Fatalf("BuildSellTx error: %v", err)
	}
	data := hexutil.Encode(tx.Data())
	expected := "0xd3c9727c" + hex32(tokenAmountIn) + hex32(minRefundWei) + hex32(new(big.Int).SetUint64(deadline))
	if data != expected {
		t.Fatalf("unexpected calldata\nexpected=%s\nactual=%s", expected, data)
	}
}

func TestBuildApproveTxCalldata(t *testing.T) {
	token := common.HexToAddress("0x3333333333333333333333333333333333333333")
	spender := common.HexToAddress("0x4444444444444444444444444444444444444444")
	builder := NewBuilderWithClock(big.NewInt(8453), 60*time.Second, time.Now)

	params := BuildParams{
		Nonce:    2,
		GasLimit: 70000,
		Fee: FeeParams{
			MaxFeePerGas:         big.NewInt(1000000000),
			MaxPriorityFeePerGas: big.NewInt(200000000),
		},
	}

	amount := big.NewInt(1000000)

	tx, err := builder.BuildApproveTx(token, spender, amount, params)
	if err != nil {
		t.Fatalf("BuildApproveTx error: %v", err)
	}
	data := hexutil.Encode(tx.Data())
	expected := "0x095ea7b3" + hexAddress(spender) + hex32(amount)
	if data != expected {
		t.Fatalf("unexpected calldata\nexpected=%s\nactual=%s", expected, data)
	}
}

func TestParseUnits(t *testing.T) {
	v, err := ParseUnits("1.23", 6)
	if err != nil {
		t.Fatalf("ParseUnits error: %v", err)
	}
	if v.String() != "1230000" {
		t.Fatalf("unexpected value: %s", v.String())
	}

	v, err = ParseUnits("0.000001", 6)
	if err != nil {
		t.Fatalf("ParseUnits error: %v", err)
	}
	if v.String() != "1" {
		t.Fatalf("unexpected value: %s", v.String())
	}
}

func hex32(v *big.Int) string {
	b := common.LeftPadBytes(v.Bytes(), 32)
	return hexutil.Encode(b)[2:]
}

func hexAddress(addr common.Address) string {
	b := common.LeftPadBytes(addr.Bytes(), 32)
	return hexutil.Encode(b)[2:]
}
