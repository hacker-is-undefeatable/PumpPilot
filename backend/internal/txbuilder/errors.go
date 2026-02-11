package txbuilder

import "github.com/ethereum/go-ethereum"

type EstimateGasError struct {
	Err     error
	CallMsg ethereum.CallMsg
}

func (e *EstimateGasError) Error() string {
	if e == nil {
		return "estimate gas failed"
	}
	if e.Err == nil {
		return "estimate gas failed"
	}
	return "estimate gas failed: " + e.Err.Error()
}

func (e *EstimateGasError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
