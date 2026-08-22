# Author: MrAbhi2k3
# GitHub: https://github.com/MrAbhi2k3
#
# This file is part of the AutoAnime distribution.
#    Copyright (c) 2026 TeleroidGroup
#
#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU General Public License as published by
#    the Free Software Foundation, version 3.
#
#    This program is distributed in the hope that it will be useful, but
#    WITHOUT ANY WARRANTY; without even the implied warranty of
#    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
#    General Public License for more details.
#
# License can be found in <
# https://github.com/TeleroidGroup/AutoAnimeBot/blob/main/LICENSE > .
#
# if you are using this following code then don't forgot to give proper
# credit to t.me/TeleroidGroup (github.com/TeleroidGroup)

import os
import sys
from dotenv import load_dotenv

load_dotenv()

# Required environment variables
API_ID = os.getenv("API_ID")
API_HASH = os.getenv("API_HASH")
BOT_TOKEN = os.getenv("BOT_TOKEN")
MONGO_SRV = os.getenv("MONGO_SRV")

missing_vars = []
if not API_ID:
    missing_vars.append("API_ID")
if not API_HASH:
    missing_vars.append("API_HASH")
if not BOT_TOKEN:
    missing_vars.append("BOT_TOKEN")
if not MONGO_SRV:
    missing_vars.append("MONGO_SRV")

if missing_vars:
    print(f"Error: Missing required environment variables: {', '.join(missing_vars)}")
    print("Please set them in your environment or a .env file.")
    sys.exit(1)

try:
    API_ID = int(API_ID)
except ValueError:
    print("Error: API_ID must be an integer.")
    sys.exit(1)

# Optional / configuration variables with defaults
CHANNEL_ID = os.getenv("CHANNEL_ID")
try:
    if CHANNEL_ID:
        CHANNEL_ID = int(CHANNEL_ID)
except ValueError:
    pass

LOG_CHANNEL = os.getenv("LOG_CHANNEL")
try:
    if LOG_CHANNEL:
        LOG_CHANNEL = int(LOG_CHANNEL)
except ValueError:
    pass
if not LOG_CHANNEL:
    LOG_CHANNEL = CHANNEL_ID

BACKUP_CHANNEL = os.getenv("BACKUP_CHANNEL")
try:
    if BACKUP_CHANNEL:
        BACKUP_CHANNEL = int(BACKUP_CHANNEL)
except ValueError:
    pass

FORCESUB_CHANNEL = os.getenv("FORCESUB_CHANNEL")
try:
    if FORCESUB_CHANNEL:
        FORCESUB_CHANNEL = int(FORCESUB_CHANNEL)
except ValueError:
    pass

FORCESUB_CHANNEL_LINK = os.getenv("FORCESUB_CHANNEL_LINK", "")
CLOUDFLARE_BASE_URL = os.getenv("CLOUDFLARE_BASE_URL", "")
BUTTON_UPLOAD = os.getenv("BUTTON_UPLOAD", "True").lower() == "true"

try:
    CHECK_INTERVAL = int(os.getenv("CHECK_INTERVAL", "600"))
except ValueError:
    CHECK_INTERVAL = 600

