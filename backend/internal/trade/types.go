package trade

type BuyRequest struct {
	From            string `json:"from"`
	Pair            string `json:"pair"`
	Token           string `json:"token,omitempty"`
	TokenDecimals   *uint8 `json:"token_decimals,omitempty"`
	EthIn           string `json:"eth_in,omitempty"`
	EthInWei        string `json:"eth_in_wei,omitempty"`
	MinTokensOut    string `json:"min_tokens_out,omitempty"`
	MinTokensOutWei string `json:"min_tokens_out_wei,omitempty"`
	Simulate        bool   `json:"simulate,omitempty"`
}

type SellRequest struct {
	From             string `json:"from"`
	Pair             string `json:"pair"`
	Token            string `json:"token"`
	TokenDecimals    *uint8 `json:"token_decimals,omitempty"`
	TokenAmountIn    string `json:"token_amount_in,omitempty"`
	TokenAmountInWei string `json:"token_amount_in_wei,omitempty"`
	MinRefundEth     string `json:"min_refund_eth,omitempty"`
	MinRefundWei     string `json:"min_refund_wei,omitempty"`
	Simulate         bool   `json:"simulate,omitempty"`
}

type ApproveRequest struct {
	From          string `json:"from"`
	Token         string `json:"token"`
	Pair          string `json:"pair,omitempty"`
	Spender       string `json:"spender,omitempty"`
	TokenDecimals *uint8 `json:"token_decimals,omitempty"`
	Amount        string `json:"amount,omitempty"`
	AmountWei     string `json:"amount_wei,omitempty"`
	Simulate      bool   `json:"simulate,omitempty"`
}

type TxResult struct {
	Tx              interface{} `json:"tx,omitempty"`
	TxHash          string      `json:"tx_hash,omitempty"`
	SimulationError string      `json:"simulation_error,omitempty"`
}
