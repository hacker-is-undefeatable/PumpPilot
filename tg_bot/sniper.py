# /// script
# requires-python = ">=3.14"
# dependencies = [
#     "dotenv>=0.9.9",
#     "python-telegram-bot>=22.6",
#     "requests>=2.32.5",
# ]
# ///
import asyncio
import json
import requests
import os
from dotenv import load_dotenv

from telegram import Update
from telegram.ext import (
    Application,
    CommandHandler,
    MessageHandler,
    filters,
    ContextTypes,
    ConversationHandler,
)

# Load environment variables
load_dotenv()

# Backend config
BACKEND_API = os.getenv("BACKEND_API", "http://localhost:8080")
BACKEND_AUTH_TOKEN = os.getenv("BACKEND_AUTH_TOKEN", "")
OUTPUT_JSONL_PATH = os.getenv("OUTPUT_JSONL_PATH", "../backend/data/output.jsonl")
RPC_URL = os.getenv("RPC_URL", "https://weathered-tiniest-sunset.base-mainnet.quiknode.pro/eef4e16be8050f49227e10af6d447be5175e638b/")

# Factory address and event topic for the pool creation event
FACTORY_ADDRESS = os.getenv("FACTORY_ADDRESS", "0x07dfaec8e182c5ef79844adc70708c1c15aa60fb").lower()
TOKEN_CREATED_TOPIC = "0x01b6aab41d4eb83cfcd6c8c59cc6c3dd697ac0110c58c23bf222e7884e44245c"

# Max uint256 for approve
MAX_UINT256 = str(2**256 - 1)

# Conversation states
SLIPPAGE, TRACKED_ADDRESS, WAIT_TIME, POSITION_SIZE, CONFIRM = range(5)


def _headers():
    h = {"Content-Type": "application/json"}
    if BACKEND_AUTH_TOKEN:
        h["X-API-Key"] = BACKEND_AUTH_TOKEN
    return h


async def start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    await update.message.reply_text(
        "Welcome to PumpPilot Sniping Bot on Base! Let's create a new sniping setup.\n"
        "Enter slippage percentage (e.g., 10 for 10%):"
    )
    return SLIPPAGE


async def slippage(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    try:
        context.user_data["slippage"] = float(update.message.text)
        await update.message.reply_text(
            "Enter the address to track (developer's public key):"
        )
        return TRACKED_ADDRESS
    except ValueError:
        await update.message.reply_text("Invalid slippage. Please enter a number:")
        return SLIPPAGE


async def tracked_address(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    context.user_data["tracked_address"] = update.message.text
    await update.message.reply_text(
        "Enter wait time in seconds before selling (optional, enter 0 for none):"
    )
    return WAIT_TIME


async def wait_time(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    try:
        wt = int(update.message.text)
        context.user_data["wait_time"] = wt if wt > 0 else None
        await update.message.reply_text("Enter position size in ETH:")
        return POSITION_SIZE
    except ValueError:
        await update.message.reply_text(
            "Invalid wait time. Please enter a number or 0:"
        )
        return WAIT_TIME


async def position_size(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    try:
        context.user_data["position_size"] = float(update.message.text)
        await update.message.reply_text("Confirm setup? (yes/no)")
        return CONFIRM
    except ValueError:
        await update.message.reply_text("Invalid position size. Please enter a number:")
        return POSITION_SIZE


async def confirm(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    if update.message.text.lower() == "yes":
        await update.message.reply_text("Creating wallet via backend...")
        try:
            address = create_wallet()
            context.user_data["address"] = address
            await update.message.reply_text(
                f"Please fund the address {address} with at least "
                f"{context.user_data['position_size'] + 0.0003} ETH (including gas). "
                f"Reply 'funded' when done."
            )
            return ConversationHandler.END
        except Exception as e:
            await update.message.reply_text(f"Error creating wallet: {str(e)}")
            return ConversationHandler.END
    else:
        await update.message.reply_text("Setup cancelled.")
        return ConversationHandler.END


async def funded(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    await update.message.reply_text("Starting sniping...")
    asyncio.create_task(
        watch_jsonl_for_tokens(
            chat_id=update.message.chat_id,
            tracked_address=context.user_data["tracked_address"],
            wallet_address=context.user_data["address"],
            slippage=context.user_data["slippage"],
            wait_time=context.user_data["wait_time"],
            position_size=context.user_data["position_size"],
            bot=context.bot,
        )
    )


# --------------- Backend API helpers ---------------


def create_wallet() -> str:
    resp = requests.post(f"{BACKEND_API}/keys", headers=_headers())
    resp.raise_for_status()
    return resp.json()["address"]


def execute_buy(wallet_address: str, pair: str, token: str, eth_in: str) -> dict:
    payload = {
        "from": wallet_address,
        "pair": pair,
        "token": token,
        "eth_in": eth_in,
        "min_tokens_out": "0",
    }
    resp = requests.post(
        f"{BACKEND_API}/trade/buy", headers=_headers(), json=payload
    )
    resp.raise_for_status()
    return resp.json()


def execute_approve(wallet_address: str, pair: str, token: str) -> dict:
    payload = {
        "from": wallet_address,
        "token": token,
        "pair": pair,
        "amount_wei": MAX_UINT256,
    }
    resp = requests.post(
        f"{BACKEND_API}/trade/approve", headers=_headers(), json=payload
    )
    resp.raise_for_status()
    return resp.json()


def execute_sell(
    wallet_address: str, pair: str, token: str, token_amount_in_wei: str
) -> dict:
    payload = {
        "from": wallet_address,
        "pair": pair,
        "token": token,
        "token_amount_in_wei": token_amount_in_wei,
        "min_refund_eth": "0",
    }
    resp = requests.post(
        f"{BACKEND_API}/trade/sell", headers=_headers(), json=payload
    )
    resp.raise_for_status()
    return resp.json()


def get_token_balance(wallet_address: str, token: str) -> str:
    resp = requests.get(
        f"{BACKEND_API}/balances",
        headers=_headers(),
        params={"address": wallet_address, "token": token},
    )
    resp.raise_for_status()
    return resp.json().get("balance_wei", "0")


# --------------- Pool/token extraction ---------------


def _topic_to_address(topic: str) -> str:
    """Convert a 32-byte hex topic to a checksumless 0x address."""
    return "0x" + topic[-40:]


def _extract_pool_token(event: dict) -> tuple[str, str]:
    """Extract pool and token addresses from a JSONL event.

    First checks enriched fields (pool_address, token_addresses).
    Falls back to fetching the tx receipt from RPC and parsing raw logs.
    """
    # Try enriched fields first
    pool = event.get("pool_address", "")
    tokens = event.get("token_addresses", [])
    if pool and tokens:
        return pool, tokens[0]

    # Fetch receipt from RPC and parse raw logs
    tx_hash = event.get("tx_hash")
    if not tx_hash:
        return "", ""

    try:
        resp = requests.post(
            RPC_URL,
            json={
                "jsonrpc": "2.0",
                "method": "eth_getTransactionReceipt",
                "params": [tx_hash],
                "id": 1,
            },
            timeout=10,
        )
        receipt = resp.json().get("result", {})
        for log in receipt.get("logs", []):
            topics = log.get("topics", [])
            addr = log.get("address", "").lower()
            if (
                addr == FACTORY_ADDRESS
                and len(topics) >= 3
                and topics[0] == TOKEN_CREATED_TOPIC
            ):
                pool = _topic_to_address(topics[1])
                token = _topic_to_address(topics[2])
                return pool, token
    except Exception as e:
        print(f"RPC receipt fetch failed: {e}")

    return "", ""


# --------------- JSONL file watcher ---------------


async def watch_jsonl_for_tokens(
    chat_id: int,
    tracked_address: str,
    wallet_address: str,
    slippage: float,
    wait_time: int | None,
    position_size: float,
    bot,
):
    path = os.path.abspath(OUTPUT_JSONL_PATH)
    print(f"Watching {path} for tokens from {tracked_address}...")

    # Start from end of file if it exists
    try:
        with open(path, "r") as f:
            f.seek(0, 2)  # seek to end
            pos = f.tell()
    except FileNotFoundError:
        pos = 0

    while True:
        try:
            with open(path, "r") as f:
                f.seek(pos)
                new_data = f.read()
                pos = f.tell()
        except FileNotFoundError:
            await asyncio.sleep(1)
            continue

        if new_data:
            for line in new_data.strip().split("\n"):
                line = line.strip()
                if not line:
                    continue
                try:
                    event = json.loads(line)
                except json.JSONDecodeError:
                    continue

                creator = event.get("from", "")
                if creator.lower() != tracked_address.lower():
                    continue

                # Parse pool/token from raw receipt logs
                pool_address, token = _extract_pool_token(event)
                if not pool_address or not token:
                    await bot.send_message(
                        chat_id,
                        f"Matched tracked address but could not extract pool/token from tx {event.get('tx_hash', '?')[:16]}...",
                    )
                    continue

                tx_hash = event.get("tx_hash", "unknown")

                await bot.send_message(
                    chat_id,
                    f"New token launched by tracked address!\n"
                    f"Pool: {pool_address}\n"
                    f"Token: {token}\n"
                    f"Tx: {tx_hash}",
                )

                # Buy
                try:
                    result = execute_buy(
                        wallet_address, pool_address, token, str(position_size)
                    )
                    await bot.send_message(
                        chat_id,
                        f"Buy executed: {result.get('tx_hash', 'pending')}",
                    )
                except Exception as e:
                    await bot.send_message(chat_id, f"Buy failed: {e}")
                    continue

                # Wait then sell
                if wait_time:
                    await bot.send_message(
                        chat_id, f"Waiting {wait_time}s before selling..."
                    )
                    await asyncio.sleep(wait_time)

                    balance = get_token_balance(wallet_address, token)
                    if balance and balance != "0":
                        try:
                            execute_approve(wallet_address, pool_address, token)
                            result = execute_sell(
                                wallet_address, pool_address, token, balance
                            )
                            await bot.send_message(
                                chat_id,
                                f"Sell executed: {result.get('tx_hash', 'pending')}",
                            )
                        except Exception as e:
                            await bot.send_message(chat_id, f"Sell failed: {e}")
                    else:
                        await bot.send_message(chat_id, "No tokens to sell.")

        await asyncio.sleep(1)


def main():
    TOKEN = os.getenv("TELEGRAM_BOT_TOKEN")
    if not TOKEN:
        raise ValueError("TELEGRAM_BOT_TOKEN not found in .env file")

    application = Application.builder().token(TOKEN).build()

    conv_handler = ConversationHandler(
        entry_points=[CommandHandler("start", start)],
        states={
            SLIPPAGE: [MessageHandler(filters.TEXT & ~filters.COMMAND, slippage)],
            TRACKED_ADDRESS: [
                MessageHandler(filters.TEXT & ~filters.COMMAND, tracked_address)
            ],
            WAIT_TIME: [MessageHandler(filters.TEXT & ~filters.COMMAND, wait_time)],
            POSITION_SIZE: [
                MessageHandler(filters.TEXT & ~filters.COMMAND, position_size)
            ],
            CONFIRM: [MessageHandler(filters.TEXT & ~filters.COMMAND, confirm)],
        },
        fallbacks=[],
    )

    application.add_handler(conv_handler)
    application.add_handler(
        MessageHandler(filters.TEXT & filters.Regex(r"^(funded)$"), funded)
    )

    application.run_polling()


if __name__ == "__main__":
    main()
