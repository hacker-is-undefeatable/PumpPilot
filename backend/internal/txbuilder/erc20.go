package txbuilder

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	selectorBalanceOf = mustSelector("0x70a08231")
	selectorDecimals  = mustSelector("0x313ce567")
)

func BuildBalanceOfCallData(owner common.Address) []byte {
	data := append([]byte{}, selectorBalanceOf...)
	data = append(data, encodeAddress(owner)...)
	return data
}

func BuildDecimalsCallData() []byte {
	return append([]byte{}, selectorDecimals...)
}

func ReadERC20Balance(ctx context.Context, rpcClient *rpc.Client, token common.Address, owner common.Address) (*big.Int, error) {
	if rpcClient == nil {
		return nil, errors.New("rpc client is nil")
	}
	data := BuildBalanceOfCallData(owner)
	call := map[string]string{
		"to":   token.Hex(),
		"data": hexutil.Encode(data),
	}
	var out string
	if err := rpcClient.CallContext(ctx, &out, "eth_call", call, "latest"); err != nil {
		return nil, err
	}
	return decodeHexBig(out)
}

func ReadERC20Decimals(ctx context.Context, rpcClient *rpc.Client, token common.Address) (uint8, error) {
	if rpcClient == nil {
		return 0, errors.New("rpc client is nil")
	}
	call := map[string]string{
		"to":   token.Hex(),
		"data": hexutil.Encode(BuildDecimalsCallData()),
	}
	var out string
	if err := rpcClient.CallContext(ctx, &out, "eth_call", call, "latest"); err != nil {
		return 0, err
	}
	v, err := decodeHexBig(out)
	if err != nil {
		return 0, err
	}
	if v.Sign() < 0 || v.BitLen() > 8 {
		return 0, fmt.Errorf("decimals out of range: %s", v.String())
	}
	return uint8(v.Uint64()), nil
}

func ParseUnits(amount string, decimals uint8) (*big.Int, error) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return nil, errors.New("amount is empty")
	}
	if strings.HasPrefix(amount, "-") {
		return nil, errors.New("amount must be non-negative")
	}
	parts := strings.SplitN(amount, ".", 2)
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if intPart == "" {
		intPart = "0"
	}
	if len(fracPart) > int(decimals) {
		return nil, fmt.Errorf("too many decimal places: %d > %d", len(fracPart), decimals)
	}
	fracPart = fracPart + strings.Repeat("0", int(decimals)-len(fracPart))
	combined := intPart + fracPart
	combined = strings.TrimLeft(combined, "0")
	if combined == "" {
		return big.NewInt(0), nil
	}
	v, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, errors.New("invalid number format")
	}
	return v, nil
}
