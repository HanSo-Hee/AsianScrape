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
import asyncio
from bot.core import start_bot
from aiohttp import web

async def health_check(request):
    return web.Response(text="Bot is running!", status=200)

async def start_web_server():
    port = os.getenv("PORT")
    if port:
        app = web.Application()
        app.router.add_get('/', health_check)
        app.router.add_get('/health', health_check)
        runner = web.AppRunner(app)
        await runner.setup()
        site = web.TCPSite(runner, '0.0.0.0', int(port))
        await site.start()
        print(f"Web server started on port {port} for Render health checks.")

async def main():
    await asyncio.gather(
        start_web_server(),
        start_bot()
    )

if __name__ == "__main__":
    asyncio.run(main())

