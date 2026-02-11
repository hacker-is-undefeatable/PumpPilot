package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a scalar")
	}
	if value.Value == "" {
		d.Duration = 0
		return nil
	}
	if value.Tag == "!!int" {
		var v int64
		if err := value.Decode(&v); err != nil {
			return err
		}
		d.Duration = time.Duration(v) * time.Millisecond
		return nil
	}
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

type Config struct {
	Chain          string `yaml:"chain"`
	ChainID        uint64 `yaml:"chain_id"`
	FactoryAddress string `yaml:"factory_address"`

	RPC struct {
		HTTP string `yaml:"http"`
		WS   string `yaml:"ws"`
	} `yaml:"rpc"`

	Ingestion struct {
		StartBlock       string   `yaml:"start_block"`
		Confirmations    uint64   `yaml:"confirmations"`
		ReorgReplayDepth uint64   `yaml:"reorg_replay_depth"`
		PollInterval     Duration `yaml:"poll_interval"`
	} `yaml:"ingestion"`

	Performance struct {
		BlockFetchConcurrency   int      `yaml:"block_fetch_concurrency"`
		ReceiptFetchConcurrency int      `yaml:"receipt_fetch_concurrency"`
		RequestTimeout          Duration `yaml:"request_timeout"`
		RetryMax                int      `yaml:"retry_max"`
		RetryBackoff            Duration `yaml:"retry_backoff"`
		QueueSize               int      `yaml:"queue_size"`
	} `yaml:"performance"`

	Decoding struct {
		ABIPath       string         `yaml:"abi_path"`
		EventMappings []EventMapping `yaml:"event_mappings"`
		MethodFilter  []string       `yaml:"method_filter"`
		LogAddresses  []string       `yaml:"log_addresses"`
		DecodeInput   bool           `yaml:"decode_input"`
		DecodeLogs    bool           `yaml:"decode_logs"`
		AllowMissing  bool           `yaml:"allow_missing_abi"`
	} `yaml:"decoding"`

	Tx struct {
		DefaultDeadlineSeconds uint64  `yaml:"default_deadline_seconds"`
		GasLimitMultiplier     float64 `yaml:"gas_limit_multiplier"`
		MaxFeeMultiplier       float64 `yaml:"max_fee_multiplier"`
		MinPriorityFeeGwei     float64 `yaml:"min_priority_fee_gwei"`
		FeeRefreshSeconds      uint64  `yaml:"fee_refresh_seconds"`
	} `yaml:"tx"`

	KeyStore struct {
		Dir                string `yaml:"dir"`
		PassphraseEnv      string `yaml:"passphrase_env"`
		AllowPrivateExport bool   `yaml:"allow_private_export"`
	} `yaml:"keystore"`

	API struct {
		Listen    string `yaml:"listen"`
		AuthToken string `yaml:"auth_token"`
	} `yaml:"api"`

	Checkpoint struct {
		Path string `yaml:"path"`
	} `yaml:"checkpoint"`

	Output struct {
		JSONLPath string `yaml:"jsonl_path"`
	} `yaml:"output"`
}

type EventMapping struct {
	Event       string   `yaml:"event"`
	PoolField   string   `yaml:"pool_field"`
	TokenFields []string `yaml:"token_fields"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Chain == "" {
		c.Chain = "base"
	}
	if c.ChainID == 0 {
		switch strings.ToLower(c.Chain) {
		case "base":
			c.ChainID = 8453
		}
	}
	if c.Ingestion.Confirmations == 0 {
		c.Ingestion.Confirmations = 2
	}
	if c.Ingestion.ReorgReplayDepth == 0 {
		c.Ingestion.ReorgReplayDepth = 5
	}
	if c.Ingestion.PollInterval.Duration == 0 {
		c.Ingestion.PollInterval = Duration{Duration: 5 * time.Second}
	}
	if c.Performance.BlockFetchConcurrency == 0 {
		c.Performance.BlockFetchConcurrency = 1
	}
	if c.Performance.ReceiptFetchConcurrency == 0 {
		c.Performance.ReceiptFetchConcurrency = 8
	}
	if c.Performance.RequestTimeout.Duration == 0 {
		c.Performance.RequestTimeout = Duration{Duration: 15 * time.Second}
	}
	if c.Performance.RetryMax == 0 {
		c.Performance.RetryMax = 3
	}
	if c.Performance.RetryBackoff.Duration == 0 {
		c.Performance.RetryBackoff = Duration{Duration: 500 * time.Millisecond}
	}
	if c.Performance.QueueSize == 0 {
		c.Performance.QueueSize = 2000
	}
	if c.Tx.DefaultDeadlineSeconds == 0 {
		c.Tx.DefaultDeadlineSeconds = 120
	}
	if c.Tx.GasLimitMultiplier == 0 {
		c.Tx.GasLimitMultiplier = 1.2
	}
	if c.Tx.MaxFeeMultiplier == 0 {
		c.Tx.MaxFeeMultiplier = 2.0
	}
	if c.Tx.FeeRefreshSeconds == 0 {
		c.Tx.FeeRefreshSeconds = 5
	}
	if c.KeyStore.Dir == "" {
		c.KeyStore.Dir = "data/keystore"
	}
	if c.KeyStore.PassphraseEnv == "" {
		c.KeyStore.PassphraseEnv = "PUMPPILOT_KEYSTORE_PASSPHRASE"
	}
	if c.API.Listen == "" {
		c.API.Listen = ":8080"
	}
	if c.Checkpoint.Path == "" {
		c.Checkpoint.Path = "data/checkpoint.json"
	}
	if c.Output.JSONLPath == "" {
		c.Output.JSONLPath = "data/output.jsonl"
	}
}

func (c *Config) validate() error {
	if c.FactoryAddress == "" {
		return fmt.Errorf("factory_address is required")
	}
	if c.RPC.HTTP == "" {
		return fmt.Errorf("rpc.http is required")
	}
	if c.RPC.WS == "" {
		return fmt.Errorf("rpc.ws is required")
	}
	if c.Ingestion.StartBlock == "" {
		c.Ingestion.StartBlock = "latest"
	}
	if c.Performance.BlockFetchConcurrency < 1 {
		return fmt.Errorf("block_fetch_concurrency must be >= 1")
	}
	if c.Performance.ReceiptFetchConcurrency < 1 {
		return fmt.Errorf("receipt_fetch_concurrency must be >= 1")
	}
	return nil
}

func (c *Config) StartBlockNumber() (uint64, bool, error) {
	if strings.ToLower(c.Ingestion.StartBlock) == "latest" {
		return 0, true, nil
	}
	v, err := strconv.ParseUint(c.Ingestion.StartBlock, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid start_block %q", c.Ingestion.StartBlock)
	}
	return v, false, nil
}
