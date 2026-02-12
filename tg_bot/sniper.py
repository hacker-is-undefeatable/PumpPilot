# /// script
# requires-python = ">=3.14"
# dependencies = [
# "dotenv>=0.9.9",
# "python-telegram-bot>=22.6",
# "requests>=2.32.5",
# ]
# ///
import asyncio
import json
import requests
import os
from dotenv import load_dotenv

from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import (
    Application,
    CommandHandler,
    MessageHandler,
    CallbackQueryHandler,
    filters,
    ContextTypes,
)

# Load environment variables
load_dotenv()

# Backend config
BACKEND_API = os.getenv("BACKEND_API", "http://localhost:8080")
BACKEND_AUTH_TOKEN = os.getenv("BACKEND_AUTH_TOKEN", "")
OUTPUT_JSONL_PATH = os.getenv("OUTPUT_JSONL_PATH", "../backend/data/output.jsonl")
RPC_URL = os.getenv(
    "RPC_URL",
    "https://weathered-tiniest-sunset.base-mainnet.quiknode.pro/eef4e16be8050f49227e10af6d447be5175e638b/",
)

# Factory address and event topic for the pool creation event
FACTORY_ADDRESS = os.getenv(
    "FACTORY_ADDRESS", "0x07dfaec8e182c5ef79844adc70708c1c15aa60fb"
).lower()
TOKEN_CREATED_TOPIC = (
    "0x01b6aab41d4eb83cfcd6c8c59cc6c3dd697ac0110c58c23bf222e7884e44245c"
)

# Max uint256 for approve
MAX_UINT256 = str(2**256 - 1)

# Conversation states for text input flows
(
    INPUT_TRACKED_ADDRESS,
    INPUT_POSITION_SIZE,
    INPUT_SLIPPAGE_CUSTOM,
    INPUT_TOKEN_WITHDRAW_ADDR,
    INPUT_WITHDRAW_ADDRESS,
    INPUT_WITHDRAW_AMOUNT,
) = range(6)

# --------------- Helpers ---------------


def _headers():
    h = {"Content-Type": "application/json"}
    if BACKEND_AUTH_TOKEN:
        h["X-API-Key"] = BACKEND_AUTH_TOKEN
    return h


def _shorten(addr: str) -> str:
    if len(addr) > 12:
        return f"{addr[:6]}...{addr[-4:]}"
    return addr


def _wei_to_eth(wei_str: str) -> float:
    try:
        return int(wei_str) / 1e18
    except (ValueError, TypeError):
        return 0.0


# --------------- Backend API helpers ---------------


def create_wallet() -> str:
    resp = requests.post(f"{BACKEND_API}/keys", headers=_headers())
    resp.raise_for_status()
    return resp.json()["address"]


def get_eth_balance(address: str) -> str:
    resp = requests.get(
        f"{BACKEND_API}/balances",
        headers=_headers(),
        params={"address": address},
    )
    resp.raise_for_status()
    return resp.json().get("eth_wei", "0")


def get_token_balance(wallet_address: str, token: str) -> str:
    resp = requests.get(
        f"{BACKEND_API}/balances",
        headers=_headers(),
        params={"address": wallet_address, "token": token},
    )
    resp.raise_for_status()
    return resp.json().get("balance_wei", "0")


def execute_buy(wallet_address: str, pair: str, token: str, eth_in: str) -> dict:
    payload = {
        "from": wallet_address,
        "pair": pair,
        "token": token,
        "eth_in": eth_in,
        "min_tokens_out": "0",
    }
    resp = requests.post(f"{BACKEND_API}/trade/buy", headers=_headers(), json=payload)
    resp.raise_for_status()
    return resp.json()


def execute_token_transfer(
    from_addr: str, to_addr: str, token: str, amount_wei: str
) -> dict:
    payload = {
        "from": from_addr,
        "to": to_addr,
        "token": token,
        "amount_wei": amount_wei,
    }
    resp = requests.post(
        f"{BACKEND_API}/trade/transfer_token", headers=_headers(), json=payload
    )
    resp.raise_for_status()
    return resp.json()


def execute_transfer(from_addr: str, to_addr: str, eth_out: str) -> dict:
    payload = {
        "from": from_addr,
        "to": to_addr,
        "eth_out": eth_out,
    }
    resp = requests.post(
        f"{BACKEND_API}/trade/transfer", headers=_headers(), json=payload
    )
    resp.raise_for_status()
    return resp.json()


# --------------- Pool/token extraction ---------------


def _topic_to_address(topic: str) -> str:
    return "0x" + topic[-40:]


def _extract_pool_token(event: dict) -> tuple[str, str]:
    pool = event.get("pool_address", "")
    tokens = event.get("token_addresses", [])
    if pool and tokens:
        return pool, tokens[0]

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


# --------------- Token info ---------------


def get_token_symbol(token_address: str) -> str:
    try:
        resp = requests.post(
            RPC_URL,
            json={
                "jsonrpc": "2.0",
                "method": "eth_call",
                "params": [
                    {
                        "to": token_address,
                        "data": "0x95d89b41",
                    },
                    "latest",
                ],
                "id": 1,
            },
            timeout=10,
        )
        result = resp.json().get("result")
        if result and len(result) > 130:
            length = int(result[66:130], 16)
            symbol_hex = result[130 : 130 + length * 2]
            return bytes.fromhex(symbol_hex).decode("utf-8")
        return "UNKNOWN"
    except Exception:
        return "UNKNOWN"


# --------------- UI: Keyboard builders ---------------


def main_menu_keyboard(sniping_active: bool = False) -> InlineKeyboardMarkup:
    rows = [
        [
            InlineKeyboardButton("Setup Sniper", callback_data="setup_sniper"),
            InlineKeyboardButton("Portfolio", callback_data="portfolio"),
        ],
        [
            InlineKeyboardButton("Withdraw", callback_data="withdraw"),
            InlineKeyboardButton("Settings", callback_data="settings"),
        ],
    ]
    return InlineKeyboardMarkup(rows)


def active_sniper_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [
            [
                InlineKeyboardButton("Portfolio", callback_data="portfolio"),
                InlineKeyboardButton("Withdraw", callback_data="withdraw"),
            ],
            [
                InlineKeyboardButton("Main Menu", callback_data="back_menu"),
            ],
        ]
    )


def slippage_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [
            [
                InlineKeyboardButton("5%", callback_data="slip_5"),
                InlineKeyboardButton("10%", callback_data="slip_10"),
                InlineKeyboardButton("15%", callback_data="slip_15"),
                InlineKeyboardButton("Custom", callback_data="slip_custom"),
            ],
            [InlineKeyboardButton("Back", callback_data="back_menu")],
        ]
    )


def confirm_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [
            [
                InlineKeyboardButton("Confirm", callback_data="confirm_yes"),
                InlineKeyboardButton("Cancel", callback_data="confirm_no"),
            ],
        ]
    )


def back_menu_keyboard() -> InlineKeyboardMarkup:
    return InlineKeyboardMarkup(
        [[InlineKeyboardButton("Back to Menu", callback_data="back_menu")]]
    )


# --------------- UI: Dashboard message ---------------


async def send_dashboard(update_or_query, context: ContextTypes.DEFAULT_TYPE):
    """Send or edit the main dashboard message."""
    ud = context.user_data
    wallet = ud.get("address", "")

    lines = ["*PumpPilot Dashboard*", ""]
    if wallet:
        try:
            bal_wei = get_eth_balance(wallet)
            bal_eth = _wei_to_eth(bal_wei)
            lines.append(f"Wallet: `{_shorten(wallet)}`")
            lines.append(f"Balance: {bal_eth:.6f} ETH")
        except Exception:
            lines.append(f"Wallet: `{_shorten(wallet)}`")
            lines.append("Balance: unavailable")
    else:
        lines.append("No wallet yet. Use *Setup Sniper* to create one.")

    # Show active sniper info
    if ud.get("sniping_active"):
        lines.append("")
        lines.append("Sniper: ACTIVE")
        tracked = ud.get("tracked_address", "")
        lines.append(f"Tracking: `{_shorten(tracked)}`")

    text = "\n".join(lines)
    kb = main_menu_keyboard(sniping_active=bool(ud.get("sniping_active")))

    if hasattr(update_or_query, "edit_message_text"):
        try:
            await update_or_query.edit_message_text(
                text, reply_markup=kb, parse_mode="Markdown"
            )
            return
        except Exception:
            pass
    # Fallback: send new message
    if hasattr(update_or_query, "message") and update_or_query.message:
        await update_or_query.message.reply_text(
            text, reply_markup=kb, parse_mode="Markdown"
        )
    elif hasattr(update_or_query, "effective_chat"):
        await context.bot.send_message(
            update_or_query.effective_chat.id,
            text,
            reply_markup=kb,
            parse_mode="Markdown",
        )


# --------------- Handlers: /start ---------------


async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    await send_dashboard(update, context)


# --------------- Handlers: Callback queries ---------------


async def button_handler(update: Update, context: ContextTypes.DEFAULT_TYPE):
    query = update.callback_query
    await query.answer()
    data = query.data
    ud = context.user_data

    # ---- Main menu ----
    if data == "back_menu":
        await send_dashboard(query, context)
        return

    # ---- Setup sniper ----
    if data == "setup_sniper":
        # Initialize setup defaults
        ud.setdefault("slippage", 10.0)
        ud.setdefault("position_size", 0.01)
        await query.edit_message_text(
            "Enter the address to track (developer's wallet):",
            reply_markup=back_menu_keyboard(),
        )
        ud["awaiting_input"] = "tracked_address"
        return

    # ---- Slippage selection ----
    if data.startswith("slip_"):
        val = data[5:]
        if val == "custom":
            await query.edit_message_text(
                "Enter custom slippage percentage:",
                reply_markup=back_menu_keyboard(),
            )
            ud["awaiting_input"] = "slippage_custom"
            return
        ud["slippage"] = float(val)
        await query.edit_message_text(
            "Enter the withdraw address for tokens:",
            reply_markup=back_menu_keyboard(),
        )
        ud["awaiting_input"] = "token_withdraw_addr"
        return

    # ---- Confirm sniper ----
    if data == "confirm_yes":
        await _start_sniper(query, context)
        return

    if data == "confirm_no":
        await query.edit_message_text("Setup cancelled.")
        await send_dashboard(update, context)
        return

    # ---- Portfolio ----
    if data == "portfolio":
        await _show_portfolio(query, context)
        return

    # ---- Withdraw ----
    if data == "withdraw":
        wallet = ud.get("address", "")
        if not wallet:
            await query.edit_message_text(
                "No wallet found. Set up a sniper first.",
                reply_markup=back_menu_keyboard(),
            )
            return
        try:
            bal_eth = _wei_to_eth(get_eth_balance(wallet))
        except Exception:
            bal_eth = 0.0
        await query.edit_message_text(
            f"Available: {bal_eth:.6f} ETH\n\nEnter destination address:",
            reply_markup=back_menu_keyboard(),
        )
        ud["awaiting_input"] = "withdraw_address"
        return

    if data == "confirm_withdraw":
        await _execute_withdraw(query, context)
        return

    if data == "cancel_withdraw":
        ud.pop("withdraw_to", None)
        ud.pop("withdraw_amount", None)
        await send_dashboard(query, context)
        return

    # ---- Settings ----
    if data == "settings":
        await _show_settings(query, ud)
        return

    if data == "set_slippage":
        await query.edit_message_text(
            "Select slippage:", reply_markup=slippage_keyboard()
        )
        return

    if data == "set_position":
        await query.edit_message_text(
            "Enter new position size in ETH:",
            reply_markup=back_menu_keyboard(),
        )
        ud["awaiting_input"] = "position_size"
        return


# --------------- Handlers: Text input ---------------


async def text_input_handler(update: Update, context: ContextTypes.DEFAULT_TYPE):
    ud = context.user_data
    awaiting = ud.pop("awaiting_input", None)
    text = update.message.text.strip()

    if awaiting == "tracked_address":
        ud["tracked_address"] = text
        await update.message.reply_text(
            "Enter position size in ETH (e.g. 0.01):",
            reply_markup=back_menu_keyboard(),
        )
        ud["awaiting_input"] = "position_size_setup"
        return

    if awaiting == "position_size_setup":
        try:
            ud["position_size"] = float(text)
        except ValueError:
            await update.message.reply_text("Invalid number. Try again:")
            ud["awaiting_input"] = "position_size_setup"
            return
        await update.message.reply_text(
            "Select slippage:", reply_markup=slippage_keyboard()
        )
        return

    if awaiting == "slippage_custom":
        try:
            ud["slippage"] = float(text)
        except ValueError:
            await update.message.reply_text("Invalid number. Try again:")
            ud["awaiting_input"] = "slippage_custom"
            return
        await update.message.reply_text(
            "Enter the withdraw address for tokens:",
            reply_markup=back_menu_keyboard(),
        )
        ud["awaiting_input"] = "token_withdraw_addr"
        return

    if awaiting == "token_withdraw_addr":
        if not text.startswith("0x") or len(text) != 42:
            await update.message.reply_text(
                "Invalid address. Must be 0x... (42 chars). Try again:"
            )
            ud["awaiting_input"] = "token_withdraw_addr"
            return
        ud["token_withdraw_addr"] = text
        lines = [
            "*Confirm Sniper Setup*",
            "",
            f"Tracked: `{_shorten(ud.get('tracked_address', '?'))}`",
            f"Position: {ud.get('position_size', '?')} ETH",
            f"Slippage: {ud.get('slippage', '?')}%",
            f"Token Withdraw: `{_shorten(ud.get('token_withdraw_addr', '?'))}`",
        ]
        await update.message.reply_text(
            "\n".join(lines),
            reply_markup=confirm_keyboard(),
            parse_mode="Markdown",
        )
        return

    if awaiting == "withdraw_address":
        if not text.startswith("0x") or len(text) != 42:
            await update.message.reply_text(
                "Invalid address. Must be 0x... (42 chars). Try again:"
            )
            ud["awaiting_input"] = "withdraw_address"
            return
        ud["withdraw_to"] = text
        await update.message.reply_text(
            'Enter amount in ETH to withdraw (or "all"):',
            reply_markup=back_menu_keyboard(),
        )
        ud["awaiting_input"] = "withdraw_amount"
        return

    if awaiting == "withdraw_amount":
        ud["withdraw_amount"] = text
        wallet = ud.get("address", "")
        amount_display = text
        if text.lower() == "all":
            try:
                bal = _wei_to_eth(get_eth_balance(wallet))
                amount_display = f"{bal:.6f} ETH (all)"
            except Exception:
                amount_display = "all"
        else:
            amount_display = f"{text} ETH"

        await update.message.reply_text(
            f"Confirm withdrawal:\n"
            f"From: `{_shorten(wallet)}`\n"
            f"To: `{_shorten(ud['withdraw_to'])}`\n"
            f"Amount: {amount_display}",
            reply_markup=InlineKeyboardMarkup(
                [
                    [
                        InlineKeyboardButton(
                            "Confirm", callback_data="confirm_withdraw"
                        ),
                        InlineKeyboardButton("Cancel", callback_data="cancel_withdraw"),
                    ]
                ]
            ),
            parse_mode="Markdown",
        )
        return

    if awaiting == "position_size":
        try:
            ud["position_size"] = float(text)
        except ValueError:
            await update.message.reply_text("Invalid number. Try again:")
            ud["awaiting_input"] = "position_size"
            return
        await update.message.reply_text(
            f"Position size updated to {ud['position_size']} ETH.",
            reply_markup=back_menu_keyboard(),
        )
        return


# --------------- Setup flow helpers ---------------


async def _show_confirm_sniper(query, ud):
    lines = [
        "*Confirm Sniper Setup*",
        "",
        f"Tracked: `{_shorten(ud.get('tracked_address', '?'))}`",
        f"Position: {ud.get('position_size', '?')} ETH",
        f"Slippage: {ud.get('slippage', '?')}%",
        f"Token Withdraw: `{_shorten(ud.get('token_withdraw_addr', '?'))}`",
    ]
    await query.edit_message_text(
        "\n".join(lines),
        reply_markup=confirm_keyboard(),
        parse_mode="Markdown",
    )


async def _start_sniper(query, context: ContextTypes.DEFAULT_TYPE):
    ud = context.user_data
    chat_id = query.message.chat_id

    # Create wallet if needed
    if not ud.get("address"):
        try:
            await query.edit_message_text("Creating wallet...")
            address = create_wallet()
            ud["address"] = address
        except Exception as e:
            await query.edit_message_text(f"Error creating wallet: {e}")
            return

    wallet = ud["address"]
    position = ud.get("position_size", 0.01)
    required = position + 0.0003

    await query.edit_message_text(
        f"Wallet: `{wallet}`\n\n"
        f"Please fund with at least {required:.4f} ETH (position + gas).\n\n"
        f"Waiting for funds...",
        parse_mode="Markdown",
    )

    # Start funding detection loop
    asyncio.create_task(
        _wait_for_funding(
            chat_id=chat_id,
            wallet=wallet,
            required_eth=required,
            context=context,
        )
    )


async def _wait_for_funding(
    chat_id: int,
    wallet: str,
    required_eth: float,
    context: ContextTypes.DEFAULT_TYPE,
):
    """Poll wallet balance until funded, then alert and start sniping."""
    ud = context.user_data
    bot = context.bot

    while True:
        await asyncio.sleep(3)
        try:
            bal_wei = get_eth_balance(wallet)
            bal_eth = _wei_to_eth(bal_wei)
        except Exception:
            continue

        if bal_eth >= required_eth:
            await bot.send_message(
                chat_id,
                f"Wallet funded! Balance: {bal_eth:.6f} ETH\n\n"
                f"Sniping is now active.",
                reply_markup=active_sniper_keyboard(),
                parse_mode="Markdown",
            )

            ud["sniping_active"] = True

            asyncio.create_task(
                watch_jsonl_for_tokens(
                    chat_id=chat_id,
                    tracked_address=ud["tracked_address"],
                    wallet_address=wallet,
                    position_size=ud.get("position_size", 0.01),
                    bot=bot,
                )
            )
            return


# --------------- Withdraw ---------------


async def _execute_withdraw(query, context: ContextTypes.DEFAULT_TYPE):
    ud = context.user_data
    wallet = ud.get("address", "")
    to_addr = ud.get("withdraw_to", "")
    amount_str = ud.get("withdraw_amount", "0")

    if amount_str.lower() == "all":
        try:
            bal_wei = get_eth_balance(wallet)
            # Leave some for gas (0.0005 ETH)
            amount_val = _wei_to_eth(bal_wei) - 0.0005
            if amount_val <= 0:
                await query.edit_message_text(
                    "Insufficient balance for withdrawal (need gas reserve).",
                    reply_markup=back_menu_keyboard(),
                )
                return
            amount_str = f"{amount_val:.18f}"
        except Exception as e:
            await query.edit_message_text(
                f"Error fetching balance: {e}",
                reply_markup=back_menu_keyboard(),
            )
            return

    try:
        await query.edit_message_text("Processing withdrawal...")
        result = execute_transfer(wallet, to_addr, amount_str)
        tx_hash = result.get("tx_hash", "pending")
        await query.edit_message_text(
            f"Withdrawal sent!\nTx: `{tx_hash}`",
            reply_markup=back_menu_keyboard(),
            parse_mode="Markdown",
        )
    except Exception as e:
        await query.edit_message_text(
            f"Withdrawal failed: {e}",
            reply_markup=back_menu_keyboard(),
        )

    ud.pop("withdraw_to", None)
    ud.pop("withdraw_amount", None)


# --------------- Portfolio ---------------


async def _show_portfolio(query, context: ContextTypes.DEFAULT_TYPE):
    ud = context.user_data
    wallet = ud.get("address", "")

    lines = ["*Portfolio*", ""]
    if wallet:
        try:
            bal = _wei_to_eth(get_eth_balance(wallet))
            lines.append(f"Wallet: `{_shorten(wallet)}`")
            lines.append(f"ETH Balance: {bal:.6f} ETH")
        except Exception:
            lines.append(f"Wallet: `{_shorten(wallet)}`")
    lines.append("")
    lines.append("Tokens are withdrawn immediately after purchase.")
    await query.edit_message_text(
        "\n".join(lines),
        reply_markup=back_menu_keyboard(),
        parse_mode="Markdown",
    )


# --------------- Settings ---------------


async def _show_settings(query, ud):
    slippage = ud.get("slippage", 10.0)
    position = ud.get("position_size", 0.01)

    lines = [
        "*Settings*",
        "",
        f"Slippage: {slippage}%",
        f"Position Size: {position} ETH",
    ]

    await query.edit_message_text(
        "\n".join(lines),
        reply_markup=InlineKeyboardMarkup(
            [
                [
                    InlineKeyboardButton(
                        f"Slippage: {slippage}%", callback_data="set_slippage"
                    ),
                    InlineKeyboardButton(
                        f"Position: {position} ETH", callback_data="set_position"
                    ),
                ],
                [InlineKeyboardButton("Back to Menu", callback_data="back_menu")],
            ]
        ),
        parse_mode="Markdown",
    )


# --------------- JSONL file watcher ---------------


async def watch_jsonl_for_tokens(
    chat_id: int,
    tracked_address: str,
    wallet_address: str,
    position_size: float,
    bot,
):
    path = os.path.abspath(OUTPUT_JSONL_PATH)
    print(f"Watching {path} for tokens from {tracked_address}...")

    try:
        with open(path, "r") as f:
            f.seek(0, 2)
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

                pool_address, token = _extract_pool_token(event)
                if not pool_address or not token:
                    await bot.send_message(
                        chat_id,
                        f"Matched tracked address but could not extract pool/token "
                        f"from tx {event.get('tx_hash', '?')[:16]}...",
                    )
                    continue

                tx_hash = event.get("tx_hash", "unknown")
                symbol = get_token_symbol(token)

                await bot.send_message(
                    chat_id,
                    f"New token launched by tracked address!\n"
                    f"Token: {symbol} (`{_shorten(token)}`)\n"
                    f"Pool: `{_shorten(pool_address)}`\n"
                    f"Tx: `{tx_hash}`",
                    parse_mode="Markdown",
                )

                # Buy
                try:
                    result = execute_buy(
                        wallet_address, pool_address, token, str(position_size)
                    )
                    buy_tx = result.get("tx_hash", "pending")
                    await bot.send_message(
                        chat_id,
                        f"Buy executed: `{buy_tx}`",
                        parse_mode="Markdown",
                    )
                except Exception as e:
                    await bot.send_message(chat_id, f"Buy failed: {e}")
                    continue

                # Withdraw tokens
                try:
                    balance = get_token_balance(wallet_address, token)
                    if int(balance) == 0:
                        await bot.send_message(chat_id, "No tokens received?")
                        continue
                    result = execute_token_transfer(
                        wallet_address, ud.get("token_withdraw_addr"), token, balance
                    )
                    tx_hash = result.get("tx_hash", "pending")
                    await bot.send_message(
                        chat_id,
                        f"Transferred {symbol} to `{_shorten(ud['token_withdraw_addr'])}`! Tx: `{tx_hash}`",
                        parse_mode="Markdown",
                    )
                except Exception as e:
                    await bot.send_message(chat_id, f"Token withdraw failed: {e}")

        await asyncio.sleep(1)


# --------------- Main ---------------


def main():
    TOKEN = os.getenv("TELEGRAM_BOT_TOKEN")
    if not TOKEN:
        raise ValueError("TELEGRAM_BOT_TOKEN not found in .env file")

    application = Application.builder().token(TOKEN).build()

    application.add_handler(CommandHandler("start", start))
    application.add_handler(CallbackQueryHandler(button_handler))
    application.add_handler(
        MessageHandler(filters.TEXT & ~filters.COMMAND, text_input_handler)
    )

    application.run_polling()


if __name__ == "__main__":
    main()
