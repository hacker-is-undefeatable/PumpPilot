package trade

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"pumppilot/internal/keys"
	"pumppilot/internal/txbuilder"
)

type Service struct {
	auto      *txbuilder.AutoBuilder
	client    *ethclient.Client
	rpcClient *rpc.Client
	keys      *keys.Manager
}

func NewService(auto *txbuilder.AutoBuilder, client *ethclient.Client, rpcClient *rpc.Client, keys *keys.Manager) *Service {
	return &Service{auto: auto, client: client, rpcClient: rpcClient, keys: keys}
}

func (s *Service) Buy(ctx context.Context, req BuyRequest) (*TxResult, error) {
	from, err := parseAddress(req.From)
	if err != nil {
		return nil, err
	}
	pair, err := parseAddress(req.Pair)
	if err != nil {
		return nil, err
	}
	ethValue, err := parseEthAmount(req.EthIn, req.EthInWei)
	if err != nil {
		return nil, err
	}
	decimals, err := s.resolveDecimals(ctx, req.Token, req.TokenDecimals)
	if err != nil {
		return nil, err
	}
	minOut, err := parseTokenAmount(req.MinTokensOut, req.MinTokensOutWei, decimals)
	if err != nil {
		return nil, err
	}
	tx, err := s.auto.BuildBuyTx(ctx, from, pair, ethValue, minOut)
	if err != nil {
		return nil, err
	}
	return s.signAndSend(ctx, from, tx, req.Simulate)
}

func (s *Service) Sell(ctx context.Context, req SellRequest) (*TxResult, error) {
	from, err := parseAddress(req.From)
	if err != nil {
		return nil, err
	}
	pair, err := parseAddress(req.Pair)
	if err != nil {
		return nil, err
	}
	decimals, err := s.resolveDecimals(ctx, req.Token, req.TokenDecimals)
	if err != nil {
		return nil, err
	}
	tokenIn, err := parseTokenAmount(req.TokenAmountIn, req.TokenAmountInWei, decimals)
	if err != nil {
		return nil, err
	}
	minRefund, err := parseEthAmount(req.MinRefundEth, req.MinRefundWei)
	if err != nil {
		return nil, err
	}
	tx, err := s.auto.BuildSellTx(ctx, from, pair, tokenIn, minRefund)
	if err != nil {
		return nil, err
	}
	return s.signAndSend(ctx, from, tx, req.Simulate)
}

func (s *Service) Approve(ctx context.Context, req ApproveRequest) (*TxResult, error) {
	from, err := parseAddress(req.From)
	if err != nil {
		return nil, err
	}
	spenderAddr := req.Spender
	if strings.TrimSpace(spenderAddr) == "" {
		spenderAddr = req.Pair
	}
	spender, err := parseAddress(spenderAddr)
	if err != nil {
		return nil, err
	}
	token, err := parseAddress(req.Token)
	if err != nil {
		return nil, err
	}
	decimals, err := s.resolveDecimals(ctx, req.Token, req.TokenDecimals)
	if err != nil {
		return nil, err
	}
	amount, err := parseTokenAmount(req.Amount, req.AmountWei, decimals)
	if err != nil {
		return nil, err
	}
	tx, err := s.auto.BuildApproveTx(ctx, from, token, spender, amount)
	if err != nil {
		return nil, err
	}
	return s.signAndSend(ctx, from, tx, req.Simulate)
}

func (s *Service) signAndSend(ctx context.Context, from common.Address, tx *types.Transaction, simulate bool) (*TxResult, error) {
	if tx == nil {
		return nil, errors.New("transaction is nil")
	}
	if s.auto == nil {
		return nil, errors.New("auto builder not configured")
	}
	if simulate {
		if err := simulateTx(ctx, s.client, from, tx); err != nil {
			return &TxResult{Tx: TxSummary(tx), SimulationError: err.Error()}, nil
		}
	}
	if s.keys == nil {
		return nil, errors.New("keystore not configured")
	}
	chainID := s.auto.ChainID()
	signed, err := s.keys.SignTransaction(from, tx, chainID)
	if err != nil {
		s.auto.ResetNonce(from)
		return nil, err
	}
	if err := s.client.SendTransaction(ctx, signed); err != nil {
		s.auto.ResetNonce(from)
		return nil, err
	}
	return &TxResult{Tx: TxSummary(signed), TxHash: signed.Hash().Hex()}, nil
}

func (s *Service) resolveDecimals(ctx context.Context, token string, override *uint8) (uint8, error) {
	if override != nil {
		return *override, nil
	}
	if token == "" || s.rpcClient == nil {
		return 18, nil
	}
	addr, err := parseAddress(token)
	if err != nil {
		return 0, err
	}
	return txbuilder.ReadERC20Decimals(ctx, s.rpcClient, addr)
}

func parseAddress(value string) (common.Address, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return common.Address{}, errors.New("address is required")
	}
	if !common.IsHexAddress(value) {
		return common.Address{}, errors.New("invalid address")
	}
	return common.HexToAddress(value), nil
}

func parseEthAmount(eth string, wei string) (*big.Int, error) {
	if wei != "" {
		return parseBigInt(wei)
	}
	if eth == "" {
		return nil, errors.New("eth amount is required")
	}
	return txbuilder.ParseUnits(eth, 18)
}

func parseTokenAmount(amount string, amountWei string, decimals uint8) (*big.Int, error) {
	if amountWei != "" {
		return parseBigInt(amountWei)
	}
	if amount == "" {
		return nil, errors.New("token amount is required")
	}
	return txbuilder.ParseUnits(amount, decimals)
}

func parseBigInt(value string) (*big.Int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("value is empty")
	}
	if strings.HasPrefix(value, "0x") {
		return txbuilder.ParseHexBig(value)
	}
	v, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return nil, errors.New("invalid integer")
	}
	return v, nil
}

func simulateTx(ctx context.Context, client *ethclient.Client, from common.Address, tx *types.Transaction) error {
	if client == nil {
		return errors.New("client is nil")
	}
	msg := ethereum.CallMsg{
		From:      from,
		To:        tx.To(),
		Value:     tx.Value(),
		Data:      tx.Data(),
		Gas:       tx.Gas(),
		GasFeeCap: tx.GasFeeCap(),
		GasTipCap: tx.GasTipCap(),
	}
	_, err := client.CallContract(ctx, msg, nil)
	return err
}

func TxSummary(tx *types.Transaction) map[string]interface{} {
	if tx == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"hash":             tx.Hash().Hex(),
		"type":             tx.Type(),
		"nonce":            tx.Nonce(),
		"to":               addrToHex(tx.To()),
		"value":            tx.Value().String(),
		"gas":              tx.Gas(),
		"max_fee_wei":      tx.GasFeeCap().String(),
		"priority_fee_wei": tx.GasTipCap().String(),
		"data":             hexutil.Encode(tx.Data()),
	}
}

func addrToHex(addr *common.Address) string {
	if addr == nil {
		return ""
	}
	return addr.Hex()
}
