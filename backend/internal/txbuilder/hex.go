package txbuilder

import (
	"errors"
	"math/big"
	"strings"
)

func decodeHexBig(value string) (*big.Int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("hex value is empty")
	}
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		value = value[2:]
	}
	value = strings.TrimLeft(value, "0")
	if value == "" {
		return big.NewInt(0), nil
	}
	if len(value)%2 == 1 {
		value = "0" + value
	}
	v, ok := new(big.Int).SetString(value, 16)
	if !ok {
		return nil, errors.New("invalid hex number")
	}
	return v, nil
}
