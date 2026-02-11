package decoder

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"pumppilot/internal/config"
	"pumppilot/internal/queue"
)

type Decoder struct {
	hasABI        bool
	abi           abi.ABI
	methodFilter  map[string]struct{}
	eventMappings map[common.Hash]eventMapping
	logAddrs      map[common.Address]struct{}
	decodeAllLogs bool
}

type eventMapping struct {
	Event       abi.Event
	PoolField   string
	TokenFields []string
}

func New(cfg config.Config) (*Decoder, error) {
	d := &Decoder{
		methodFilter:  map[string]struct{}{},
		eventMappings: map[common.Hash]eventMapping{},
		logAddrs:      map[common.Address]struct{}{},
	}
	if len(cfg.Decoding.MethodFilter) > 0 {
		for _, name := range cfg.Decoding.MethodFilter {
			d.methodFilter[strings.TrimSpace(name)] = struct{}{}
		}
	}
	for _, addr := range cfg.Decoding.LogAddresses {
		if addr == "" {
			continue
		}
		d.logAddrs[common.HexToAddress(addr)] = struct{}{}
	}
	if cfg.Decoding.ABIPath == "" {
		if cfg.Decoding.AllowMissing {
			return d, nil
		}
		return nil, errors.New("decoding.abi_path is required")
	}
	b, err := os.ReadFile(cfg.Decoding.ABIPath)
	if err != nil {
		if cfg.Decoding.AllowMissing {
			return d, nil
		}
		return nil, err
	}
	parsed, err := abi.JSON(strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	d.abi = parsed
	d.hasABI = true
	if len(cfg.Decoding.EventMappings) == 0 {
		d.decodeAllLogs = true
		return d, nil
	}
	for _, m := range cfg.Decoding.EventMappings {
		event, ok := d.abi.Events[m.Event]
		if !ok {
			return nil, fmt.Errorf("event %q not found in ABI", m.Event)
		}
		d.eventMappings[event.ID] = eventMapping{
			Event:       event,
			PoolField:   m.PoolField,
			TokenFields: m.TokenFields,
		}
	}
	return d, nil
}

func (d *Decoder) DecodeInput(data []byte) (*queue.DecodedMethod, error) {
	if !d.hasABI || len(data) < 4 {
		return nil, nil
	}
	method, err := d.abi.MethodById(data[:4])
	if err != nil {
		return nil, nil
	}
	if len(d.methodFilter) > 0 {
		if _, ok := d.methodFilter[method.Name]; !ok {
			return nil, nil
		}
	}
	args := map[string]interface{}{}
	if err := method.Inputs.UnpackIntoMap(args, data[4:]); err != nil {
		return nil, err
	}
	return &queue.DecodedMethod{
		Name: method.Name,
		Args: normalizeMap(args),
	}, nil
}

func (d *Decoder) DecodeLogs(logs []*types.Log) ([]queue.DecodedLog, string, []string, error) {
	if !d.hasABI {
		return nil, "", nil, nil
	}
	decoded := make([]queue.DecodedLog, 0)
	var pool string
	var tokens []string
	seenTokens := map[string]struct{}{}

	for _, l := range logs {
		if len(d.logAddrs) > 0 {
			if _, ok := d.logAddrs[l.Address]; !ok {
				continue
			}
		}
		if len(l.Topics) == 0 {
			continue
		}
		mapping, ok := d.eventMappings[l.Topics[0]]
		if !ok && !d.decodeAllLogs {
			continue
		}
		if !ok && d.decodeAllLogs {
			// Attempt to match any ABI event by ID
			for _, ev := range d.abi.Events {
				if ev.ID == l.Topics[0] {
					mapping = eventMapping{Event: ev}
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		args := map[string]interface{}{}
		if err := mapping.Event.Inputs.UnpackIntoMap(args, l.Data); err != nil {
			return decoded, pool, tokens, err
		}
		if err := abi.ParseTopicsIntoMap(args, mapping.Event.Inputs, l.Topics[1:]); err != nil {
			return decoded, pool, tokens, err
		}

		decoded = append(decoded, queue.DecodedLog{
			Event:   mapping.Event.Name,
			Address: l.Address.Hex(),
			Args:    normalizeMap(args),
			Topics:  topicsToHex(l.Topics),
			Data:    "0x" + hex.EncodeToString(l.Data),
		})

		if mapping.PoolField != "" {
			if v, ok := args[mapping.PoolField]; ok {
				if addr := normalizeAddress(v); addr != "" {
					pool = addr
				}
			}
		}
		for _, field := range mapping.TokenFields {
			if v, ok := args[field]; ok {
				if addr := normalizeAddress(v); addr != "" {
					if _, exists := seenTokens[addr]; !exists {
						seenTokens[addr] = struct{}{}
						tokens = append(tokens, addr)
					}
				}
			}
		}
	}

	return decoded, pool, tokens, nil
}

func topicsToHex(topics []common.Hash) []string {
	out := make([]string, 0, len(topics))
	for _, t := range topics {
		out = append(out, t.Hex())
	}
	return out
}

func normalizeMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = normalizeValue(v)
	}
	return out
}

func normalizeValue(v interface{}) interface{} {
	switch t := v.(type) {
	case common.Address:
		return t.Hex()
	case *common.Address:
		if t == nil {
			return ""
		}
		return t.Hex()
	case common.Hash:
		return t.Hex()
	case *big.Int:
		if t == nil {
			return "0"
		}
		return t.String()
	case []byte:
		return "0x" + hex.EncodeToString(t)
	case [32]byte:
		return "0x" + hex.EncodeToString(t[:])
	case []common.Address:
		out := make([]string, 0, len(t))
		for _, a := range t {
			out = append(out, a.Hex())
		}
		return out
	case []*big.Int:
		out := make([]string, 0, len(t))
		for _, n := range t {
			if n == nil {
				out = append(out, "0")
				continue
			}
			out = append(out, n.String())
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(t))
		for _, v := range t {
			out = append(out, normalizeValue(v))
		}
		return out
	default:
		return t
	}
}

func normalizeAddress(v interface{}) string {
	switch t := v.(type) {
	case common.Address:
		return t.Hex()
	case *common.Address:
		if t == nil {
			return ""
		}
		return t.Hex()
	default:
		return ""
	}
}
