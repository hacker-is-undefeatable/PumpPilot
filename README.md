# PumpPilot

The video can be found here: https://drive.google.com/file/d/11uWZMI66YzibA3v7prXeSwz_xbqPelRH/view?usp=sharing.
The bot can be found here: https://t.me/Robin_Sniper_Bot

PumpPilot is a specialized blockchain monitoring and trading tool designed for the Base network. It features a high-performance Go backend for real-time event ingestion and transaction management, coupled with a Python Telegram bot for user interaction and sniping operations.

## Architecture

The project consists of two main components:

1.  **Backend (Go)**:
    *   **Event Ingestion**: Monitors block headers and logs in real-time.
    *   **Decoding**: Decodes specific contract events (e.g., `PoolCreated` from a Factory contract).
    *   **TxBuilder**: Manages transaction construction, fees, and nonce management.
    *   **API**: Exposes endpoints for the frontend/bot to interact with.

2.  **Telegram Bot (Python)**:
    *   **Sniper Interface**: Allows users to input tracked addresses and manage trades.
    *   **Alerts**: Receives updates from the backend and notifies the user.
    *   **Interaction**: Handles user commands for approving tokens and executing trades.

## Prerequisites

*   [Go](https://go.dev/) (1.21+)
*   [Python](https://www.python.org/) (3.14+)
*   Access to an Ethereum-compatible RPC (HTTP and WebSocket) for the Base network.

## Setup & Configuration

### 1. Backend

Navigate to the `backend` directory:

```bash
cd backend
```

Create a configuration file based on the example:

```bash
cp config/config.example.yaml config.yaml
```

Edit `config.yaml` to include your RPC endpoints:

```yaml
rpc:
  http: "https://YOUR_BASE_RPC_URL"
  ws: "wss://YOUR_BASE_WS_URL"
```

### 2. Telegram Bot

Navigate to the `tg_bot` directory:

```bash
cd tg_bot
```

Create a `.env` file with the necessary environment variables:

```env
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
BACKEND_API=http://localhost:8080
RPC_URL=https://your_rpc_url
```

## Usage

### Running the Backend

From the `backend` directory:

```bash
go run cmd/pumppilot/main.go --config config.yaml
```

The server will start listening on port `:8080` (or as configured).

### Running the Telegram Bot

From the `tg_bot` directory:

```bash
# Install dependencies (using uv or pip)
pip install python-telegram-bot requests python-dotenv

# Run the bot
python sniper.py
```

## Features

*   **Real-time Monitoring**: Ingests blocks and logs with configurable concurrency.
*   **Reorg Handling**: Built-in protection for chain reorgs.
*   **Transaction Management**: Automatic gas estimation and nonce management.
*   **Keystore Security**: Local encrypted keystore for wallet management.

## Project Structure

```
├── backend/            # Go backend application
│   ├── cmd/            # Entry points (pumppilot, server)
│   ├── internal/       # Core business logic (ingestion, trading, txbuilder)
│   └── config/         # Configuration templates and ABIs
├── tg_bot/             # Python Telegram bot
│   └── sniper.py       # Main bot script
└── web/                # Web frontend (if applicable)
```

## Disclaimer

This software is for educational purposes only. Use at your own risk.
