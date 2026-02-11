# /// script
# requires-python = ">=3.14"
# dependencies = [
#     "dotenv>=0.9.9",
#     "requests>=2.32.5",
#     "telegram>=0.0.1",
#     "websockets>=16.0",
# ]
# ///
import asyncio
import websockets
import json
import requests
import time
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

# Constants
ROBINPUMP_WS = "ws://localhost:3000/api/data"
ROBINPUMP_API = "http://localhost:3000/api/trade"

# Conversation states
SLIPPAGE, TRACKED_ADDRESS, WAIT_TIME, POSITION_SIZE, CONFIRM = range(5)


async def start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    await update.message.reply_text(
        "Welcome to the RobinPump Fun Sniping Bot on Base! Let's create a new sniping setup.\n"
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
        wait_time = int(update.message.text)
        context.user_data["wait_time"] = wait_time if wait_time > 0 else None
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
        await update.message.reply_text("Creating new Lightning wallet...")
        try:
            private_key, public_key = create_lightning_wallet()
            context.user_data["private_key"] = private_key
            context.user_data["public_key"] = public_key
            await update.message.reply_text(
                f"Please fund the address {public_key} with at least {context.user_data['position_size'] + 0.01} ETH (including fees). Reply 'funded' when done."
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
        subscribe_to_new_tokens(
            update.message.chat_id,
            context.user_data["tracked_address"],
            context.user_data["private_key"],
            context.user_data["slippage"],
            context.user_data["wait_time"],
            context.user_data["position_size"],
            context.bot,
            context.user_data["public_key"],
        )
    )


def create_lightning_wallet():
    response = requests.post("http://localhost:3000/api/lightning/create")
    if response.status_code == 200:
        data = response.json()
        private_key = data["private_key"]  # Assume hex or string, handle accordingly
        public_key = data["public_key"]
        return private_key, public_key
    else:
        raise Exception("Failed to create wallet: " + response.text)


def execute_trade(
    action, mint, amount, private_key, slippage, priority_fee=500000
):  # Adjusted for gas
    payload = {
        "action": action,
        "mint": mint,
        "amount": amount,
        "denominatedInEth": True if action == "buy" else False,
        "priorityFee": priority_fee,
        "slippage": slippage,
        "privateKey": private_key,
    }
    headers = {"Content-Type": "application/json"}
    response = requests.post(ROBINPUMP_API, headers=headers, json=payload)
    if response.status_code == 200:
        print(f"{action.capitalize()} executed successfully for mint {mint}.")
        return response.json()
    else:
        print(f"Failed to {action}: {response.text}")
        return None


def get_token_balance(public_key, mint):
    payload = {"public_key": public_key, "mint": mint}
    headers = {"Content-Type": "application/json"}
    response = requests.post(
        "http://localhost:3000/api/balance", headers=headers, json=payload
    )
    if response.status_code == 200:
        data = response.json()
        return data.get("balance", 0)
    else:
        print(f"Failed to get balance: {response.text}")
        return 0


async def subscribe_to_new_tokens(
    chat_id,
    tracked_address,
    private_key,
    slippage,
    wait_time,
    position_size,
    bot,
    public_key,
):
    async with websockets.connect(ROBINPUMP_WS) as websocket:
        payload = {"method": "subscribeNewToken"}
        await websocket.send(json.dumps(payload))
        print("Subscribed to new token events...")

        while True:
            message = await websocket.recv()
            data = json.loads(message)
            if "tx" in data and "from" in data["tx"]:
                creator = data["tx"]["from"]
                mint = data.get("mint")  # Assuming mint is in data
                if creator.lower() == tracked_address.lower():
                    await bot.send_message(
                        chat_id, f"New token launched by tracked address: Mint {mint}"
                    )
                    # Buy
                    result = execute_trade(
                        "buy", mint, position_size, private_key, slippage
                    )
                    if result:
                        await bot.send_message(chat_id, "Buy executed successfully.")

                    if wait_time:
                        await bot.send_message(
                            chat_id, f"Waiting {wait_time} seconds before selling..."
                        )
                        await asyncio.sleep(wait_time)

                        # Get token balance to sell all
                        balance = get_token_balance(public_key, mint)
                        if balance > 0:
                            result = execute_trade(
                                "sell", mint, balance, private_key, slippage
                            )
                            if result:
                                await bot.send_message(
                                    chat_id, "Sell executed successfully."
                                )
                        else:
                            await bot.send_message(chat_id, "No tokens to sell.")


def main():
    TOKEN = os.getenv("TELEGRAM_BOT_TOKEN")
    if not TOKEN:
        raise ValueError("TELEGRAM_BOT_TOKEN not found in .env file")

    application = Application.builder() .token(TOKEN).build()

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
