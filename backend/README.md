# PumpPilot

Minimal, reliable pipeline for streaming Base factory transactions, decoding inputs and logs, and emitting enriched JSONL.

## What it does
- Subscribes to new block headers (WSS) with HTTP polling fallback
- Fetches full blocks via HTTP (`eth_getBlockByNumber` with full txs)
- Filters txs where `to == factory_address`
- Fetches receipts for logs and status
- Decodes input and logs using ABI
- Extracts pool + token addresses from configured event fields
- Writes enriched JSONL to `data/output.jsonl`

## Configuration
Edit `config.yaml`.
- `rpc.http` and `rpc.ws` are your QuickNode endpoints
- `decoding.abi_path` should point to the factory ABI JSON file
- `decoding.event_mappings` defines which event fields map to pool/token

Example mapping:

```yaml
# config.yaml

decoding:
  abi_path: "config/factory_abi.json"
  decode_input: true
  decode_logs: true
  allow_missing_abi: false
  event_mappings:
    - event: "PoolCreated"
      pool_field: "pool"
      token_fields: ["token", "token0", "token1"]
```

## Run

```bash
cd backend
go run ./cmd/pumppilot -config config.yaml
```

## Smoke Test (latest factory txs)

```bash
cd backend
go run ./cmd/pumppilot-smoke -config config.yaml -backfill 5 -follow true
```

Single-block replay:

```bash
cd backend
go run ./cmd/pumppilot-smoke -config config.yaml -block 40887432 -debug -decode-input
```

Add `-debug` to print block fetch details.

## Output
Each line is a JSON object. Key fields:
- `block_number`, `tx_hash`, `from`, `to`, `input`
- `method` (decoded call)
- `receipt` (status, gas used, logs count)
- `decoded_logs` (decoded events)
- `pool_address`, `token_addresses` (from event mappings)

## Notes
- Input data is already included in full block tx objects. Receipts are only used for status and logs.
- Reorg handling: blocks are processed after `confirmations` and the last `reorg_replay_depth` blocks are reprocessed on startup.
- If ABI is missing, decoding is skipped but streaming continues.

## Tx Builder (buy/sell/approve)
The adapter for pair-style contracts lives in `backend/internal/txbuilder`.

Key helpers:
- `BuildBuyTx(pair, ethInWei, minTokensOut, params)`
- `BuildSellTx(pair, tokenAmountIn, minRefundWei, params)`
- `BuildApproveTx(token, pair, tokenAmountIn, params)`
- `ReadERC20Balance(ctx, rpcClient, token, owner)`
- `ReadERC20Decimals(ctx, rpcClient, token)`
- `ParseUnits("1.23", decimals)` to get base units

Deadlines are internal: `Builder` uses a default (config `tx.default_deadline_seconds`).

## Auto Tx Builder (nonce + gas + fees)
`AutoBuilder` wraps the adapter with QuickNode lookups:
- `PendingNonceAt` for nonce
- `EstimateGas` for gas limit (with a multiplier)
- `SuggestGasTipCap` + latest base fee for EIP-1559 fees

Use `NewAutoBuilderFromConfig` to wire it from config and call `Start(ctx)` to keep fees fresh.

## Testing the Tx Builder

### 1) Live (RPC) build with auto fees/nonce/gas
```bash
cd backend
go run ./cmd/txbuilder-debug \\
  -config config.yaml \\
  -mode buy \\
  -from 0xYourWallet \\
  -pair 0xPairAddress \\
  -eth-in 0.01 \\
  -min-tokens-out 1000 \\
  -token 0xTokenAddress
```

Add `-simulate` to run an `eth_call` with the built tx.

### 2) Offline build (manual nonce/gas/fees)
```bash
cd backend
go run ./cmd/txbuilder-debug \\
  -config config.yaml \\
  -mode sell \\
  -pair 0xPairAddress \\
  -token 0xTokenAddress \\
  -token-amount-in 500 \\
  -min-refund-eth 0.005 \\
  -offline \\
  -nonce 12 \\
  -gas-limit 250000 \\
  -max-fee-gwei 20 \\
  -priority-fee-gwei 2
```

### 3) Unit tests
```bash
cd backend
go test ./internal/txbuilder -run Test
```

## API Server

### Run
```bash
cd backend
export PUMPPILOT_KEYSTORE_PASSPHRASE="change-me"
go run ./cmd/server -config config.yaml
```

If `api.auth_token` is set, include `X-API-Key` (or `Authorization: Bearer <token>`).

### Endpoints
- `GET /health`
- `GET /keys` (list addresses)
- `POST /keys` (create new key)
- `POST /keys/export` (export keystore JSON, or private key if enabled)
- `GET /balances?address=0x..&token=0x..` (token optional for ETH)
- `POST /trade/buy`
- `POST /trade/sell`
- `POST /trade/approve`

### Trade Request Examples

**Buy**
```json
{
  "from": "0xYourWallet",
  "pair": "0xPairAddress",
  "eth_in": "0.01",
  "min_tokens_out": "1000",
  "token": "0xTokenAddress",
  "simulate": true
}
```

**Sell**
```json
{
  "from": "0xYourWallet",
  "pair": "0xPairAddress",
  "token": "0xTokenAddress",
  "token_amount_in": "500",
  "min_refund_eth": "0.005",
  "simulate": true
}
```

**Approve**
```json
{
  "from": "0xYourWallet",
  "token": "0xTokenAddress",
  "pair": "0xPairAddress",
  "amount": "1000000"
}
```

### Security Notes
- Keys are stored in `data/keystore/` using geth-compatible encrypted JSON files.
- Set `keystore.passphrase_env` to control which env var supplies the encryption passphrase.
- Private-key export is disabled by default (`keystore.allow_private_export=false`).
