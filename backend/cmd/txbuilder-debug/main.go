package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"pumppilot/internal/config"
	"pumppilot/internal/txbuilder"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	mode := flag.String("mode", "buy", "buy|sell|approve")
	pair := flag.String("pair", "", "pair contract address (to)")
	from := flag.String("from", "", "sender address (for nonce/gas estimation)")
	token := flag.String("token", "", "token contract address (for approve or decimals fetch)")
	spender := flag.String("spender", "", "spender address for approve (defaults to pair)")
	ethIn := flag.String("eth-in", "", "ETH input (decimal, e.g. 0.01)")
	ethInWei := flag.String("eth-in-wei", "", "ETH input in wei (integer string)")
	minTokensOut := flag.String("min-tokens-out", "", "min tokens out (decimal)")
	minTokensOutWei := flag.String("min-tokens-out-wei", "", "min tokens out (base units)")
	tokenAmountIn := flag.String("token-amount-in", "", "token amount in (decimal)")
	tokenAmountInWei := flag.String("token-amount-in-wei", "", "token amount in (base units)")
	minRefund := flag.String("min-refund-eth", "", "min refund in ETH (decimal)")
	minRefundWei := flag.String("min-refund-wei", "", "min refund in wei (integer string)")
	decimalsFlag := flag.Int("token-decimals", -1, "token decimals (optional, will fetch if not set and token provided)")
	simulate := flag.Bool("simulate", false, "run eth_call with the built tx (auto mode only)")

	offline := flag.Bool("offline", false, "build without RPC (requires nonce/gas/fees flags)")
	nonce := flag.Uint64("nonce", 0, "manual nonce (offline)")
	gasLimit := flag.Uint64("gas-limit", 0, "manual gas limit (offline)")
	maxFeeGwei := flag.String("max-fee-gwei", "", "manual max fee per gas in gwei (offline)")
	priorityFeeGwei := flag.String("priority-fee-gwei", "", "manual priority fee in gwei (offline)")

	debug := flag.Bool("debug", false, "enable debug logs")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	ctx := context.Background()

	if *offline {
		if err := runOffline(ctx, logger, cfg, *mode, *pair, *from, *token, *spender,
			*ethIn, *ethInWei, *minTokensOut, *minTokensOutWei,
			*tokenAmountIn, *tokenAmountInWei, *minRefund, *minRefundWei, *decimalsFlag,
			*nonce, *gasLimit, *maxFeeGwei, *priorityFeeGwei,
		); err != nil {
			logger.Error("offline build failed", "error", err)
			os.Exit(1)
		}
		return
	}

	rpcClient, ethClient, err := dialHTTP(ctx, cfg)
	if err != nil {
		logger.Error("http rpc dial failed", "error", err)
		os.Exit(1)
	}
	defer rpcClient.Close()
	defer ethClient.Close()

	if err := runAuto(ctx, logger, cfg, ethClient, rpcClient, *mode, *pair, *from, *token, *spender,
		*ethIn, *ethInWei, *minTokensOut, *minTokensOutWei,
		*tokenAmountIn, *tokenAmountInWei, *minRefund, *minRefundWei, *decimalsFlag, *simulate,
	); err != nil {
		if *simulate {
			if estErr, ok := err.(*txbuilder.EstimateGasError); ok {
				runSimWithMsg(ctx, logger, ethClient, estErr.CallMsg)
			}
		}
		logger.Error("auto build failed", "error", err)
		os.Exit(1)
	}
}

func dialHTTP(ctx context.Context, cfg *config.Config) (*rpc.Client, *ethclient.Client, error) {
	rpcClient, err := rpc.DialContext(ctx, cfg.RPC.HTTP)
	if err != nil {
		return nil, nil, err
	}
	rpcClient.SetHeader("User-Agent", "pumppilot-txbuilder-debug")
	return rpcClient, ethclient.NewClient(rpcClient), nil
}

func runAuto(
	ctx context.Context,
	logger *slog.Logger,
	cfg *config.Config,
	ethClient *ethclient.Client,
	rpcClient *rpc.Client,
	mode string,
	pair string,
	from string,
	token string,
	spender string,
	ethIn string,
	ethInWei string,
	minTokensOut string,
	minTokensOutWei string,
	tokenAmountIn string,
	tokenAmountInWei string,
	minRefund string,
	minRefundWei string,
	decimalsFlag int,
	simulate bool,
) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	fromAddr, err := parseAddressRequired("from", from)
	if err != nil {
		return err
	}

	auto, err := txbuilder.NewAutoBuilderFromConfig(ethClient, cfg)
	if err != nil {
		return err
	}
	// Start background fee refresh (optional for one-off).
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	auto.Start(ctx)

	decimals, err := resolveDecimals(ctx, rpcClient, token, decimalsFlag)
	if err != nil {
		return err
	}

	switch mode {
	case "buy":
		pairAddr, err := parseAddressRequired("pair", pair)
		if err != nil {
			return err
		}
		ethValue, err := parseEthAmount(ethIn, ethInWei)
		if err != nil {
			return err
		}
		minOut, err := parseTokenAmount(minTokensOut, minTokensOutWei, decimals)
		if err != nil {
			return err
		}
		tx, err := auto.BuildBuyTx(ctx, fromAddr, pairAddr, ethValue, minOut)
		if err != nil {
			return err
		}
		printTx(logger, tx, "buy")
		if simulate {
			runSim(ctx, logger, ethClient, fromAddr, tx)
		}
	case "sell":
		pairAddr, err := parseAddressRequired("pair", pair)
		if err != nil {
			return err
		}
		tokenIn, err := parseTokenAmount(tokenAmountIn, tokenAmountInWei, decimals)
		if err != nil {
			return err
		}
		minRef, err := parseEthAmount(minRefund, minRefundWei)
		if err != nil {
			return err
		}
		tx, err := auto.BuildSellTx(ctx, fromAddr, pairAddr, tokenIn, minRef)
		if err != nil {
			return err
		}
		printTx(logger, tx, "sell")
		if simulate {
			runSim(ctx, logger, ethClient, fromAddr, tx)
		}
	case "approve":
		if spender == "" {
			spender = pair
		}
		spenderAddr, err := parseAddressRequired("spender", spender)
		if err != nil {
			return err
		}
		tokenAddr, err := parseAddressRequired("token", token)
		if err != nil {
			return err
		}
		amount, err := parseTokenAmount(tokenAmountIn, tokenAmountInWei, decimals)
		if err != nil {
			return err
		}
		tx, err := auto.BuildApproveTx(ctx, fromAddr, tokenAddr, spenderAddr, amount)
		if err != nil {
			return err
		}
		printTx(logger, tx, "approve")
		if simulate {
			runSim(ctx, logger, ethClient, fromAddr, tx)
		}
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
	return nil
}

func runOffline(
	ctx context.Context,
	logger *slog.Logger,
	cfg *config.Config,
	mode string,
	pair string,
	from string,
	token string,
	spender string,
	ethIn string,
	ethInWei string,
	minTokensOut string,
	minTokensOutWei string,
	tokenAmountIn string,
	tokenAmountInWei string,
	minRefund string,
	minRefundWei string,
	decimalsFlag int,
	nonce uint64,
	gasLimit uint64,
	maxFeeGwei string,
	priorityFeeGwei string,
) error {
	_ = ctx
	builder := txbuilder.NewBuilderWithClock(big.NewInt(int64(cfg.ChainID)), time.Duration(cfg.Tx.DefaultDeadlineSeconds)*time.Second, time.Now)

	if gasLimit == 0 {
		return errors.New("gas-limit is required in offline mode")
	}
	if maxFeeGwei == "" || priorityFeeGwei == "" {
		return errors.New("max-fee-gwei and priority-fee-gwei are required in offline mode")
	}
	maxFeeFloat, err := parseFloat(maxFeeGwei)
	if err != nil {
		return err
	}
	maxFeeWei, err := txbuilder.GweiToWei(maxFeeFloat)
	if err != nil {
		return err
	}
	priorityFloat, err := parseFloat(priorityFeeGwei)
	if err != nil {
		return err
	}
	priorityWei, err := txbuilder.GweiToWei(priorityFloat)
	if err != nil {
		return err
	}
	params := txbuilder.BuildParams{
		Nonce:    nonce,
		GasLimit: gasLimit,
		Fee: txbuilder.FeeParams{
			MaxFeePerGas:         maxFeeWei,
			MaxPriorityFeePerGas: priorityWei,
		},
	}

	decimals, err := resolveDecimals(context.Background(), nil, token, decimalsFlag)
	if err != nil {
		return err
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "buy":
		pairAddr, err := parseAddressRequired("pair", pair)
		if err != nil {
			return err
		}
		ethValue, err := parseEthAmount(ethIn, ethInWei)
		if err != nil {
			return err
		}
		minOut, err := parseTokenAmount(minTokensOut, minTokensOutWei, decimals)
		if err != nil {
			return err
		}
		tx, err := builder.BuildBuyTx(pairAddr, ethValue, minOut, params)
		if err != nil {
			return err
		}
		printTx(logger, tx, "buy-offline")
	case "sell":
		pairAddr, err := parseAddressRequired("pair", pair)
		if err != nil {
			return err
		}
		tokenIn, err := parseTokenAmount(tokenAmountIn, tokenAmountInWei, decimals)
		if err != nil {
			return err
		}
		minRef, err := parseEthAmount(minRefund, minRefundWei)
		if err != nil {
			return err
		}
		tx, err := builder.BuildSellTx(pairAddr, tokenIn, minRef, params)
		if err != nil {
			return err
		}
		printTx(logger, tx, "sell-offline")
	case "approve":
		if spender == "" {
			spender = pair
		}
		spenderAddr, err := parseAddressRequired("spender", spender)
		if err != nil {
			return err
		}
		tokenAddr, err := parseAddressRequired("token", token)
		if err != nil {
			return err
		}
		amount, err := parseTokenAmount(tokenAmountIn, tokenAmountInWei, decimals)
		if err != nil {
			return err
		}
		tx, err := builder.BuildApproveTx(tokenAddr, spenderAddr, amount, params)
		if err != nil {
			return err
		}
		printTx(logger, tx, "approve-offline")
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
	return nil
}

func parseAddressRequired(name string, value string) (common.Address, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return common.Address{}, fmt.Errorf("%s is required", name)
	}
	if !common.IsHexAddress(value) {
		return common.Address{}, fmt.Errorf("%s is not a valid address", name)
	}
	return common.HexToAddress(value), nil
}

func resolveDecimals(ctx context.Context, rpcClient *rpc.Client, token string, decimalsFlag int) (uint8, error) {
	if decimalsFlag >= 0 {
		return uint8(decimalsFlag), nil
	}
	if token == "" {
		return 18, nil
	}
	if rpcClient == nil {
		return 18, nil
	}
	tokenAddr, err := parseAddressRequired("token", token)
	if err != nil {
		return 0, err
	}
	return txbuilder.ReadERC20Decimals(ctx, rpcClient, tokenAddr)
}

func parseEthAmount(eth, wei string) (*big.Int, error) {
	if wei != "" {
		return parseBigInt(wei)
	}
	if eth == "" {
		return nil, errors.New("eth amount is required")
	}
	return txbuilder.ParseUnits(eth, 18)
}

func parseTokenAmount(amount, amountWei string, decimals uint8) (*big.Int, error) {
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
		return nil, fmt.Errorf("invalid integer: %s", value)
	}
	return v, nil
}

func parseFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("value is empty")
	}
	f, _ := new(big.Rat).SetString(value)
	if f == nil {
		return 0, fmt.Errorf("invalid number: %s", value)
	}
	out, _ := f.Float64()
	return out, nil
}

func printTx(logger *slog.Logger, tx *types.Transaction, label string) {
	if tx == nil {
		logger.Info("tx is nil", "label", label)
		return
	}
	logger.Info(
		"built tx",
		"label", label,
		"type", tx.Type(),
		"nonce", tx.Nonce(),
		"to", addrToHex(tx.To()),
		"value", tx.Value().String(),
		"gas", tx.Gas(),
		"max_fee_wei", tx.GasFeeCap().String(),
		"priority_fee_wei", tx.GasTipCap().String(),
		"data", hexutil.Encode(tx.Data()),
	)
}

func addrToHex(addr *common.Address) string {
	if addr == nil {
		return ""
	}
	return addr.Hex()
}

func runSim(ctx context.Context, logger *slog.Logger, client *ethclient.Client, from common.Address, tx *types.Transaction) {
	if client == nil || tx == nil {
		return
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
	out, err := client.CallContract(ctx, msg, nil)
	if err != nil {
		reason := extractRevertReason(err)
		if reason != "" {
			logger.Warn("simulation failed", "error", err, "revert_reason", reason)
		} else {
			logger.Warn("simulation failed", "error", err)
		}
		return
	}
	logger.Info("simulation ok", "result", hexutil.Encode(out))
}

func runSimWithMsg(ctx context.Context, logger *slog.Logger, client *ethclient.Client, msg ethereum.CallMsg) {
	if client == nil {
		return
	}
	out, err := client.CallContract(ctx, msg, nil)
	if err != nil {
		reason := extractRevertReason(err)
		if reason != "" {
			logger.Warn("simulation failed", "error", err, "revert_reason", reason)
		} else {
			logger.Warn("simulation failed", "error", err)
		}
		return
	}
	logger.Info("simulation ok", "result", hexutil.Encode(out))
}

func extractRevertReason(err error) string {
	if err == nil {
		return ""
	}
	if dataErr, ok := err.(interface{ ErrorData() interface{} }); ok {
		switch v := dataErr.ErrorData().(type) {
		case string:
			if b, derr := hexutil.Decode(v); derr == nil {
				if reason, rerr := abi.UnpackRevert(b); rerr == nil {
					return reason
				}
			}
		case []byte:
			if reason, rerr := abi.UnpackRevert(v); rerr == nil {
				return reason
			}
		}
	}
	return ""
}
