package queue

import "github.com/ethereum/go-ethereum/common"

type TxItem struct {
	BlockNumber uint64
	BlockHash   common.Hash
	Timestamp   uint64
	Tx          *RawTx
	End         bool
}

type FilteredTx struct {
	BlockNumber uint64
	BlockHash   common.Hash
	Timestamp   uint64
	Tx          *RawTx
}

type BlockFiltered struct {
	BlockNumber   uint64
	FilteredCount int
}

type EnrichedTx struct {
	Chain           string            `json:"chain"`
	ChainID         uint64            `json:"chain_id"`
	BlockNumber     uint64            `json:"block_number"`
	BlockHash       string            `json:"block_hash"`
	BlockTimestamp  uint64            `json:"block_timestamp"`
	TxHash          string            `json:"tx_hash"`
	From            string            `json:"from"`
	To              string            `json:"to"`
	Nonce           uint64            `json:"nonce"`
	ValueWei        string            `json:"value_wei"`
	Gas             uint64            `json:"gas"`
	GasPriceWei     string            `json:"gas_price_wei,omitempty"`
	MaxFeePerGasWei string            `json:"max_fee_per_gas_wei,omitempty"`
	MaxPriorityFee  string            `json:"max_priority_fee_wei,omitempty"`
	Type            uint8             `json:"type"`
	Input           string            `json:"input"`
	Method          *DecodedMethod    `json:"method,omitempty"`
	Receipt         *ReceiptInfo      `json:"receipt,omitempty"`
	DecodedLogs     []DecodedLog      `json:"decoded_logs,omitempty"`
	PoolAddress     string            `json:"pool_address,omitempty"`
	TokenAddresses  []string          `json:"token_addresses,omitempty"`
	Errors          []string          `json:"errors,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
}

type RawTx struct {
	Hash              string   `json:"hash"`
	From              string   `json:"from"`
	To                string   `json:"to"`
	Nonce             uint64   `json:"nonce"`
	ValueWei          string   `json:"value_wei"`
	Gas               uint64   `json:"gas"`
	GasPriceWei       string   `json:"gas_price_wei,omitempty"`
	MaxFeePerGasWei   string   `json:"max_fee_per_gas_wei,omitempty"`
	MaxPriorityFeeWei string   `json:"max_priority_fee_wei,omitempty"`
	Type              uint64   `json:"type"`
	Input             string   `json:"input"`
	ParseErrors       []string `json:"parse_errors,omitempty"`
}

type DecodedMethod struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type DecodedLog struct {
	Event   string                 `json:"event"`
	Address string                 `json:"address"`
	Args    map[string]interface{} `json:"args"`
	Topics  []string               `json:"topics"`
	Data    string                 `json:"data"`
}

type ReceiptInfo struct {
	Status            uint64 `json:"status"`
	CumulativeGasUsed uint64 `json:"cumulative_gas_used"`
	GasUsed           uint64 `json:"gas_used"`
	EffectiveGasPrice string `json:"effective_gas_price_wei,omitempty"`
	ContractAddress   string `json:"contract_address,omitempty"`
	TxIndex           uint   `json:"transaction_index"`
	LogsCount         int    `json:"logs_count"`
}
