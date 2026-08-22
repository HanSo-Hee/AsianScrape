# Author: MrAbhi2k3
# GitHub: https://github.com/MrAbhi2k3
#
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

import asyncio
import base64
import gc
import hashlib
import logging
import math
import os
import random
import re
import time
import requests
import cloudscraper
import cryptg
import aiohttp
from bs4 import BeautifulSoup
from telethon import TelegramClient, events, Button
from telethon.errors import UserNotParticipantError
from telethon.tl.functions.channels import GetParticipantRequest
from telethon.tl.functions.messages import ExportChatInviteRequest
from telethon.tl import functions, types
from urllib.parse import urljoin
from . import config
from .database import Database

logging.basicConfig(format="[%(levelname) 5s/%(asctime)s] %(name)s: %(message)s", level=logging.INFO)
logger = logging.getLogger("Asianscraper")

client = TelegramClient("bot_session", config.API_ID, config.API_HASH)
db = Database(config.MONGO_SRV)
bot_username = ""
forcesub_invite_link = ""

async def check_user_sub(user_id):
    if not config.FORCESUB_CHANNEL:
        return True
    try:
        await client(GetParticipantRequest(config.FORCESUB_CHANNEL, user_id))
        return True
    except UserNotParticipantError:
        return False
    except Exception:
        return True

def get_hash(link):
    return hashlib.sha256(link.encode("utf-8")).hexdigest()[:16]

def get_progress_bar(current, total):
    if not total:
        return "Unknown size"
    percentage = (current / total) * 100
    completed = int(percentage / 10)
    bar = "■" * completed + "□" * (10 - completed)
    return f"[{bar}] {percentage:.1f}% ({current / 1024 / 1024:.1f}MB/{total / 1024 / 1024:.1f}MB)"

class UploadProgress:
    def __init__(self, filename, log_msg_id):
        self.filename = filename
        self.log_msg_id = log_msg_id
        self.last_update = 0

    async def callback(self, current, total):
        now = time.time()
        if now - self.last_update >= 15 or current == total:
            self.last_update = now
            bar = get_progress_bar(current, total)
            logger.info(f"Upload Progress for {self.filename}: {current}/{total} bytes")
            try:
                await client.edit_message(
                    config.LOG_CHANNEL,
                    self.log_msg_id,
                    f"**Uploading Episode to Channel...**\n\n**File:** `{self.filename}`\n{bar}"
                )
            except Exception:
                pass

async def download_file_with_progress(url, filename, log_msg_id):
    logger.info(f"Starting download from URL: {url} -> File: {filename}")
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    }
    async with aiohttp.ClientSession() as session:
        async with session.get(url, headers=headers, timeout=120) as r:
            total_size = int(r.headers.get("Content-Length", 0))
            current_size = 0
            last_update = 0
            with open(filename, "wb") as f:
                async for chunk in r.content.iter_chunked(1024 * 1024):
                    f.write(chunk)
                    current_size += len(chunk)
                    now = time.time()
                    if now - last_update >= 15 or current_size == total_size:
                        last_update = now
                        bar = get_progress_bar(current_size, total_size)
                        logger.info(f"Download Progress for {filename}: {current_size}/{total_size} bytes")
                        try:
                            await client.edit_message(
                                config.LOG_CHANNEL,
                                log_msg_id,
                                f"**Downloading Episode...**\n\n**File:** `{filename}`\n{bar}"
                            )
                        except Exception:
                            pass
    if os.path.exists(filename) and os.path.getsize(filename) < 1024:
        raise Exception(f"Downloaded file {filename} is invalid or empty ({os.path.getsize(filename)} bytes)")
    logger.info(f"Download finished: {filename}")

async def fast_upload_file(client, filepath, progress_callback=None):
    part_size = 512 * 1024
    file_size = os.path.getsize(filepath)
    part_count = math.ceil(file_size / part_size)
    file_id = random.randint(0, 2**63 - 1)
    is_big = file_size > 10 * 1024 * 1024
    
    queue = asyncio.Queue(maxsize=4)
    uploaded_bytes = 0
    progress_lock = asyncio.Lock()

    async def worker():
        nonlocal uploaded_bytes
        while True:
            item = await queue.get()
            if item is None:
                queue.task_done()
                break
            part_index, data = item
            try:
                if is_big:
                    await client(functions.upload.SaveBigFilePartRequest(
                        file_id=file_id,
                        file_part=part_index,
                        file_total_parts=part_count,
                        bytes=data
                    ))
                else:
                    await client(functions.upload.SaveFilePartRequest(
                        file_id=file_id,
                        file_part=part_index,
                        bytes=data
                    ))
                async with progress_lock:
                    uploaded_bytes += len(data)
                    if progress_callback:
                        await progress_callback(uploaded_bytes, file_size)
            except Exception as e:
                logger.error(f"Error uploading part {part_index}: {e}")
            finally:
                queue.task_done()

    workers = [asyncio.create_task(worker()) for _ in range(4)]

    with open(filepath, "rb") as f:
        for part_index in range(part_count):
            data = f.read(part_size)
            await queue.put((part_index, data))

    await queue.join()
    for _ in range(4):
        await queue.put(None)
    await asyncio.gather(*workers)

    if is_big:
        return types.InputFileBig(
            id=file_id,
            parts=part_count,
            name=os.path.basename(filepath)
        )
    else:
        md5 = hashlib.md5()
        with open(filepath, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                md5.update(chunk)
        return types.InputFile(
            id=file_id,
            parts=part_count,
            name=os.path.basename(filepath),
            md5_checksum=md5.hexdigest()
        )

def resolve_downloadwella_link(url):
    if "downloadwella.com" not in url:
        return url
    try:
        scraper = cloudscraper.create_scraper()
        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        }
        r = scraper.get(url, headers=headers, timeout=12)
        if r.status_code != 200:
            return url
        soup = BeautifulSoup(r.text, "html.parser")
        form = soup.find("form")
        if not form:
            return url
        payload = {}
        for inp in form.find_all("input"):
            name = inp.get("name")
            val = inp.get("value", "")
            if name:
                payload[name] = val
        r2 = scraper.post(url, data=payload, headers=headers, timeout=15)
        if r2.status_code != 200:
            return url
        soup2 = BeautifulSoup(r2.text, "html.parser")
        for a in soup2.find_all("a"):
            href = a.get("href", "")
            if href.endswith(".mkv") or href.endswith(".mp4") or (href and ".mkv" in href):
                return href
    except Exception:
        pass
    return url

def clean_filename(filename):
    base, ext = os.path.splitext(filename)
    cleaned = re.sub(r'\(Episodes?\s*[\d\s&,-]+\s*Added\)', '', base, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(Episodes?\s*[\d\s&,-]+\s*Complete\)', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(Complete\)', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?dramakey\.com\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?kissasia\.biz\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?dramacool\.sh\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?dramacool\.bg\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?moviesflixers_dl\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\[.*?\]', '', cleaned)
    cleaned = re.sub(r'[\._-]', ' ', cleaned)
    cleaned = re.sub(r'\s+', ' ', cleaned).strip()
    return f"{cleaned} [@KDramazFlix]{ext}"

def clean_show_title(title):
    cleaned = re.sub(r'\(Episodes?\s*[\d\s&,-]+\s*Added\)', '', title, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(Episodes?\s*[\d\s&,-]+\s*Complete\)', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(Complete\)', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?dramakey\.com\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?kissasia\.biz\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?dramacool\.sh\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\(?dramacool\.bg\)?', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'\|\s*(Chinese|Korean|Japanese|Thai|Asian)?\s*Drama', '', cleaned, flags=re.IGNORECASE)
    cleaned = re.sub(r'[\._-]', ' ', cleaned)
    cleaned = re.sub(r'\s+', ' ', cleaned).strip()
    return cleaned

def get_base_show_link(url):
    base = url.split("?")[0].split("#")[0]
    base = re.sub(r'-episode-\d+/?$', '/', base)
    return base

def extract_episode_number(title):
    match = re.search(r"episode\s*(\d+)", title, re.IGNORECASE) or re.search(r"ep\s*(\d+)", title, re.IGNORECASE) or re.search(r"ep\.\s*(\d+)", title, re.IGNORECASE) or re.search(r"[Ee](\d+)", title)
    if match:
        return int(match.group(1))
    return 1

async def download_subtitle(url, filepath):
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    }
    try:
        async with aiohttp.ClientSession() as session:
            async with session.get(url, headers=headers, timeout=30) as r:
                if r.status == 200:
                    with open(filepath, "wb") as f:
                        f.write(await r.read())
                    return True
    except Exception as e:
        logger.error(f"Failed to download subtitle: {e}")
    return False

def unwrap_video_url(video_url):
    if "cdnvideo.autos/media/" in video_url or "cdnvideo" in video_url:
        match = re.search(r'/media/([A-Za-z0-9+/=]+)(?:\.mp4)?', video_url)
        if match:
            b64_str = match.group(1)
            missing_padding = len(b64_str) % 4
            if missing_padding:
                b64_str += '=' * (4 - missing_padding)
            try:
                decoded = base64.b64decode(b64_str).decode('utf-8')
                if decoded.startswith("http"):
                    return decoded
            except Exception:
                pass
    return video_url

async def get_video_resolution(filepath):
    try:
        proc = await asyncio.create_subprocess_exec(
            "ffprobe", "-v", "error", "-select_streams", "v:0",
            "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", filepath,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE
        )
        stdout, _ = await proc.communicate()
        if proc.returncode == 0:
            parts = stdout.decode().strip().split('x')
            if len(parts) == 2:
                return int(parts[0]), int(parts[1])
    except Exception:
        pass
    return None, None

async def process_video_quality(input_file, sub_file, output_file, target_height):
    args = ["ffmpeg", "-y", "-i", input_file]
    if sub_file and os.path.exists(sub_file):
        logger.info(f"Fast muxing subtitles from {sub_file} using stream copy...")
        args.extend(["-i", sub_file, "-map", "0:v", "-map", "0:a", "-map", "1:s", "-c:v", "copy", "-c:a", "copy", "-c:s", "srt", "-metadata:s:s:0", "language=eng"])
    else:
        logger.info("Fast stream copying video file...")
        args.extend(["-c:v", "copy", "-c:a", "copy"])
            
    args.append(output_file)

    logger.info(f"Executing FFmpeg stream copy: {args}")
    proc = await asyncio.create_subprocess_exec(
        *args,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE
    )
    stdout, stderr = await proc.communicate()
    if proc.returncode != 0:
        err_str = stderr.decode()
        logger.error(f"FFmpeg failed with exit code {proc.returncode}. Error: {err_str}")
        raise Exception(f"FFmpeg failed: {err_str}")

async def download_and_upload_document(item, quality, direct_link, show_title, ep_num):
    direct_link = unwrap_video_url(direct_link)
    if "downloadwella.com" in direct_link:
        logger.info(f"Resolving downloadwella link: {direct_link}")
        resolved = resolve_downloadwella_link(direct_link)
        if resolved != direct_link:
            logger.info(f"Successfully resolved to direct file link: {resolved}")
            direct_link = resolved

    subtitle_url = item.get("subtitle_url", "")
    orig_name = direct_link.split("/")[-1].split("?")[0]
    _, orig_ext = os.path.splitext(orig_name)
    if not orig_ext:
        orig_ext = ".mp4"
    
    if subtitle_url:
        final_ext = ".mkv"
    else:
        final_ext = orig_ext
        
    local_filename = clean_filename(f"{show_title} E{ep_num:02d} {quality}{final_ext}")
    
    temp_input = f"temp_input_{quality}_{ep_num}{orig_ext}"
    temp_sub = f"temp_subtitle_{ep_num}.vtt" if subtitle_url else None

    logger.info(f"Processing download and upload of {local_filename}")
    log_msg = await client.send_message(
        config.LOG_CHANNEL,
        f"**Initializing file download...**\n\n**File:** `{local_filename}`"
    )

    try:
        await download_file_with_progress(direct_link, temp_input, log_msg.id)
        
        if subtitle_url:
            await client.edit_message(
                config.LOG_CHANNEL,
                log_msg.id,
                f"**Downloading Subtitles...**\n\n**File:** `{local_filename}`"
            )
            await download_subtitle(subtitle_url, temp_sub)
            
        await client.edit_message(
            config.LOG_CHANNEL,
            log_msg.id,
            f"**Processing video for {quality}...**\n\n**File:** `{local_filename}`"
        )
        
        target_height = 720
        if "480" in quality:
            target_height = 480
        elif "1080" in quality:
            target_height = 1080
            
        await process_video_quality(temp_input, temp_sub if (temp_sub and os.path.exists(temp_sub)) else None, local_filename, target_height)
        
        progress = UploadProgress(local_filename, log_msg.id)
        
        subtitles = item.get("subtitles", "")
        sub_line = f"🌐 **Subtitles Merged:** {subtitles}\n" if subtitles else ""
        
        caption = (
            f"🎥 **{show_title} - Episode {ep_num:02d}**\n"
            f"{sub_line}"
            f"💿 **Quality:** {quality}\n\n"
            f"📤 **Uploaded BY:** @MoviesFlixers_DL"
        )
        
        thumb_path = "bot/logo.png" if os.path.exists("bot/logo.png") else None
        
        logger.info(f"Uploading file concurrently: {local_filename} to Telegram LOG_CHANNEL (thumb={thumb_path})")
        
        input_file = await fast_upload_file(client, local_filename, progress_callback=progress.callback)
        
        msg = await client.send_file(
            config.LOG_CHANNEL,
            file=input_file,
            caption=caption,
            force_document=True,
            thumb=thumb_path
        )
        
        logger.info(f"Successfully uploaded: {local_filename} with message ID {msg.id}")
        await client.edit_message(
            config.LOG_CHANNEL,
            log_msg.id,
            f"**Successfully Processed!**\n\n**File:** `{local_filename}`\nStatus: Uploaded"
        )
        
        return msg.id
    except Exception as e:
        logger.error(f"Processing failed for file: {local_filename}. Error: {str(e)}")
        await client.edit_message(
            config.LOG_CHANNEL,
            log_msg.id,
            f"**Failed to Process!**\n\n**File:** `{local_filename}`\nError: {str(e)}"
        )
    finally:
        for f in [temp_input, temp_sub, local_filename]:
            if f and os.path.exists(f):
                try:
                    os.remove(f)
                except Exception:
                    pass
        gc.collect()
    return None
def get_show_page_and_source(url):
    url = url.split("#")[0]
    if "dramakey.com" in url:
        return url, "DramaKey"
    elif "kissasia" in url:
        return url.split("?")[0], "KissAsia"
    elif "dramacool" in url:
        if "/drama-detail/" in url:
            return url, "DramaCool"
        else:
            return url, "DramaCool_Episode"
    return None, None

def get_dramacool_show_url_from_episode(ep_url):
    try:
        scraper = cloudscraper.create_scraper()
        r = scraper.get(ep_url, headers={
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        }, timeout=15)
        if r.status_code == 200:
            soup = BeautifulSoup(r.text, "html.parser")
            for a in soup.find_all("a", href=True):
                href = a["href"]
                if "/drama-detail/" in href:
                    if not href.startswith("http"):
                        href = urljoin(ep_url, href)
                    return href
    except Exception:
        pass
    return None

async def scrape_show_data(url, source):
    scraper = cloudscraper.create_scraper()
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    }

    def fetch_soup(target_url):
        try:
            r = scraper.get(target_url, headers=headers, timeout=15)
            if r.status_code == 200:
                return BeautifulSoup(r.text, "html.parser")
        except Exception:
            pass
        return None

    def detect_qualities(links_list):
        qualities_dict = {}
        for text, href in links_list:
            combined = (href + " " + text).lower()
            if "1080" in combined:
                q = "1080p"
            elif "720" in combined:
                q = "720p"
            elif "480" in combined:
                q = "480p"
            else:
                if "480p" not in qualities_dict:
                    q = "480p"
                elif "720p" not in qualities_dict:
                    q = "720p"
                else:
                    q = "1080p"
            qualities_dict[q] = href
        return qualities_dict

    def clean_title_str(t):
        t = re.sub(r'\(\d{4}\)', '', t)
        t = re.sub(r'[\W_]+', ' ', t)
        return t.strip().lower()

    if source == "DramaKey":
        soup = fetch_soup(url)
        if not soup:
            return None
        title_el = soup.find("h1") or soup.select_one(".entry-title")
        title = title_el.text.strip() if title_el else "Unknown Drama"
        
        img_el = soup.select_one(".wp-post-image") or soup.find("img")
        img_url = ""
        if img_el:
            img_url = img_el.get("src") or img_el.get("data-src") or ""
        if not img_url:
            og_img = soup.find("meta", property="og:image")
            if og_img:
                img_url = og_img.get("content", "")
        if img_url and not img_url.startswith("http"):
            img_url = urljoin(url, img_url)

        ep_links = {}
        for a in soup.find_all("a"):
            href = a.get("href", "")
            text = a.text.strip()
            if "downloadwella" in href or "gofile" in href or "mega.nz" in href or "drive.google" in href:
                if href and not href.startswith("http"):
                    href = urljoin(url, href)
                filename = href.split("/")[-1]
                ep_match = re.search(r"[Ee](\d+)", filename) or re.search(r"episode[s]?[-_\s]?(\d+)", filename, re.IGNORECASE) or re.search(r"ep[-_\s]?(\d+)", filename, re.IGNORECASE)
                if ep_match:
                    ep_num = int(ep_match.group(1))
                    ep_key = f"Episode {ep_num:02d}"
                else:
                    ep_key = "Episode 01"
                
                if ep_key not in ep_links:
                    ep_links[ep_key] = []
                ep_links[ep_key].append((text, href))

        episodes_list = []
        for ep_key, links in sorted(ep_links.items()):
            ep_num = int(ep_key.split(" ")[-1])
            qualities = detect_qualities(links)
            
            subtitles = ""
            for text, href in links:
                if "sub" in text.lower() or "eng" in text.lower() or "sub" in href.lower():
                    subtitles = "English"
                    break

            episodes_list.append({
                "episode": ep_num,
                "title": f"{title} - {ep_key}",
                "link": f"{url}#{ep_key.replace(' ', '_')}",
                "qualities": qualities,
                "subtitles": subtitles
            })
        return {
            "title": title,
            "img_url": img_url,
            "episodes": episodes_list
        }

    elif source == "KissAsia":
        soup = fetch_soup(url)
        if not soup:
            return None
        title_el = soup.find("h1") or soup.select_one(".wp-block-post-title") or soup.select_one(".entry-title")
        title = title_el.text.strip() if title_el else "Unknown Drama"
        
        img_el = soup.select_one("img.wp-post-image") or soup.find("img")
        img_url = ""
        if img_el:
            img_url = img_el.get("src") or img_el.get("data-src") or ""
        if not img_url:
            og_img = soup.find("meta", property="og:image")
            if og_img:
                img_url = og_img.get("content", "")
        if img_url and not img_url.startswith("http"):
            img_url = urljoin(url, img_url)

        play_div = soup.find(id="Play") or soup.select_one("[data-post-id]")
        post_id = play_div.get("data-post-id") if play_div else None
        
        matched_entry = None
        if post_id:
            try:
                feed_r = scraper.get(f"https://www.blogger.com/feeds/4927638411765974267/posts/default/{post_id}?alt=json", timeout=12)
                if feed_r.status_code == 200:
                    matched_entry = feed_r.json().get("entry", {})
            except Exception:
                pass

        if not matched_entry:
            blogger_feed = []
            try:
                feed_r = scraper.get("https://www.blogger.com/feeds/4927638411765974267/posts/default?alt=json", timeout=12)
                if feed_r.status_code == 200:
                    blogger_feed = feed_r.json().get("feed", {}).get("entry", [])
            except Exception:
                pass

            cleaned_show = clean_title_str(title)
            for entry in blogger_feed:
                b_title = entry.get("title", {}).get("$t", "")
                if "-" in b_title:
                    b_show = b_title.split("-", 1)[1]
                else:
                    b_show = b_title
                cleaned_b = clean_title_str(b_show)
                if cleaned_b in cleaned_show or cleaned_show in cleaned_b:
                    matched_entry = entry
                    break

        episodes_list = []
        if matched_entry:
            content = matched_entry.get("content", {}).get("$t", "")
            lines = [line.strip() for line in content.split(";") if line.strip()]
            for idx, line in enumerate(lines):
                if "<img" in line.lower():
                    img_match = re.search(r'src="([^"]+)"', line)
                    if img_match and not img_url:
                        img_url = img_match.group(1)
                    continue
                
                link_match = re.search(r'(https?://[^\s|]+)', line)
                if link_match:
                    video_url = link_match.group(1)
                    subtitles = ""
                    subtitle_url = ""
                    sub_parts = line.split("|")
                    if len(sub_parts) >= 2:
                        subtitles = sub_parts[1].strip()
                    if len(sub_parts) >= 3:
                        sub_urls = [s.strip() for s in sub_parts[2].split(",") if s.strip()]
                        if sub_urls:
                            subtitle_url = sub_urls[0]
                        
                    ep_num = idx + 1
                    ep_match = re.search(r"[Ee](\d+)", line) or re.search(r"episode[s]?[-_\s]?(\d+)", line, re.IGNORECASE) or re.search(r"ep[-_\s]?(\d+)", line, re.IGNORECASE)
                    if ep_match:
                        ep_num = int(ep_match.group(1))
                    
                    if not subtitles and ("sub" in line.lower() or "eng" in line.lower()):
                        subtitles = "English"

                    episodes_list.append({
                        "episode": ep_num,
                        "title": f"{title} - Episode {ep_num:02d}",
                        "link": f"{url}?episode={ep_num}",
                        "qualities": {"720p": video_url},
                        "subtitles": "English" if "eng" in subtitles.lower() else subtitles,
                        "subtitle_url": subtitle_url
                    })
        
        if not episodes_list:
            raw_links = []
            for a in soup.find_all("a"):
                href = a.get("href", "")
                text = a.text.strip()
                if "gofile" in href or "mega.nz" in href or "drive.google" in href or "download" in href:
                    if href and not href.startswith("http"):
                        href = urljoin(url, href)
                    raw_links.append((text, href))
            
            ep_links = {}
            for text, href in raw_links:
                filename = href.split("/")[-1]
                ep_match = re.search(r"[Ee](\d+)", filename) or re.search(r"episode[s]?[-_\s]?(\d+)", filename, re.IGNORECASE) or re.search(r"ep[-_\s]?(\d+)", filename, re.IGNORECASE)
                if ep_match:
                    ep_num = int(ep_match.group(1))
                    ep_key = f"Episode {ep_num:02d}"
                else:
                    ep_key = "Episode 01"
                if ep_key not in ep_links:
                    ep_links[ep_key] = []
                ep_links[ep_key].append((text, href))
            
            for ep_key, links in sorted(ep_links.items()):
                ep_num = int(ep_key.split(" ")[-1])
                qualities = detect_qualities(links)
                
                subtitles = ""
                for text, href in links:
                    if "sub" in text.lower() or "eng" in text.lower() or "sub" in href.lower():
                        subtitles = "English"
                        break

                episodes_list.append({
                    "episode": ep_num,
                    "title": f"{title} - {ep_key}",
                    "link": f"{url}#{ep_key.replace(' ', '_')}",
                    "qualities": qualities,
                    "subtitles": subtitles
                })

        return {
            "title": title,
            "img_url": img_url,
            "episodes": episodes_list
        }

    elif source == "DramaCool":
        soup = fetch_soup(url)
        if not soup:
            return None
        title_el = soup.find("h1") or soup.select_one(".title") or soup.select_one("h1.title")
        title = title_el.text.strip() if title_el else "Unknown Drama"
        
        img_el = soup.select_one(".img.thumb img") or soup.select_one(".thumb img") or soup.find("img")
        img_url = ""
        if img_el:
            img_url = img_el.get("src") or img_el.get("data-src") or img_el.get("data-original") or ""
        if not img_url:
            og_img = soup.find("meta", property="og:image")
            if og_img:
                img_url = og_img.get("content", "")
        if img_url and not img_url.startswith("http"):
            img_url = urljoin(url, img_url)

        episode_pages = []
        episode_list_container = soup.select_one("ul.list-episode-item") or soup.select_one("ul.all-episode") or soup
        for a in episode_list_container.find_all("a", href=True):
            href = a["href"]
            if "-episode-" in href:
                if not href.startswith("http"):
                    href = urljoin(url, href)
                if href not in episode_pages:
                    episode_pages.append(href)

        if not episode_pages:
            episode_pages = [url]

        episodes_list = []
        for ep_url in episode_pages:
            ep_num = extract_episode_number(ep_url.split("/")[-1])
            ep_soup = fetch_soup(ep_url)
            raw_links = []
            if ep_soup:
                for a_tag in ep_soup.find_all("a"):
                    href = a_tag.get("href", "")
                    text = a_tag.text.strip()
                    if "gofile" in href or "mega" in href or "download" in href:
                        if href and not href.startswith("http"):
                            href = urljoin(ep_url, href)
                        raw_links.append((text, href))
            
            if raw_links:
                qualities = detect_qualities(raw_links)
                
                subtitles = ""
                for text, href in raw_links:
                    if "sub" in text.lower() or "eng" in text.lower() or "sub" in href.lower():
                        subtitles = "English"
                        break

                episodes_list.append({
                    "episode": ep_num,
                    "title": f"{title} - Episode {ep_num:02d}",
                    "link": ep_url,
                    "qualities": qualities,
                    "subtitles": subtitles
                })

        episodes_list.sort(key=lambda x: x["episode"])
        return {
            "title": title,
            "img_url": img_url,
            "episodes": episodes_list
        }
    return None

async def process_user_url(event, url):
    try:
        show_url, source = get_show_page_and_source(url)
        if not show_url:
            await event.respond("Could not resolve a valid show URL or unsupported domain.")
            return

        status_msg = await event.respond("Fetching details and preparing scrape...")

        if source == "DramaCool_Episode":
            await status_msg.edit("Fetching episode page to locate main show page...")
            show_url_resolved = get_dramacool_show_url_from_episode(show_url)
            if show_url_resolved:
                show_url = show_url_resolved
                source = "DramaCool"
            else:
                show_url = url
                source = "DramaCool"

        await status_msg.edit(f"Scraping {source} show details from:\n`{show_url}`")
        show_data = await scrape_show_data(show_url, source)
        if not show_data or not show_data.get("episodes"):
            await status_msg.edit("Failed to scrape show details or no episodes found.")
            return

        show_title = show_data["title"]
        img_url = show_data["img_url"]
        episodes = show_data["episodes"]
        
        await status_msg.edit(f"Found show: **{show_title}** with {len(episodes)} episodes.\nStarting download & upload process...")

        base_link = show_url
        show_id = get_hash(base_link)
        existing = db.get_file_qualities(show_id)
        if existing:
            qualities = existing["qualities"]
            show_title = existing["title"]
        else:
            qualities = {}
            show_title = clean_show_title(show_title)

        target_qualities = ["480p", "720p", "1080p"]

        def get_quality_priority(q):
            if q == "480p":
                return 1
            elif q == "720p":
                return 2
            elif q == "1080p":
                return 3
            return 4

        updated = False
        for q in target_qualities:
            await status_msg.edit(f"Processing uploads for quality: **{q}**...")
            for ep_item in episodes:
                ep_num = ep_item["episode"]
                qualities_map = ep_item.get("qualities", {})
                
                if q in qualities_map:
                    q_url = qualities_map[q]
                else:
                    available_keys = list(qualities_map.keys())
                    if not available_keys:
                        continue
                    q_url = qualities_map[available_keys[0]]
                
                already_uploaded = False
                if q in qualities:
                    for ep_data in qualities[q]:
                        if ep_data["episode"] == ep_num:
                            already_uploaded = True
                            break
                if already_uploaded:
                    continue

                subtitles = ep_item.get("subtitles", "")
                if subtitles:
                    qualities["_subtitles"] = subtitles

                msg_id = await download_and_upload_document(ep_item, q, q_url, show_title, ep_num)
                if msg_id:
                    if q not in qualities:
                        qualities[q] = []
                    qualities[q].append({"episode": ep_num, "msg_id": msg_id})
                    db.save_file_qualities(show_id, show_title, qualities)
                    db.mark_posted(ep_item["link"], ep_item["title"])
                    updated = True

        if updated or not existing:
            db.save_file_qualities(show_id, show_title, qualities)
            
            subtitles = qualities.get("_subtitles", "")
            sub_line = f"🌐 **Subtitles Merged:** {subtitles}\n" if subtitles else ""
            
            caption = (
                f"🎬 **NEW DRAMA SHOW UPDATES** 🎬\n\n"
                f"📝 **Title:** {show_title}\n"
                f"{sub_line}"
                f"ℹ️ **Source:** {source}\n\n"
                f"📤 **Uploaded BY:** @MoviesFlixers_DL"
            )
            
            buttons = []
            row = []
            for q in sorted([k for k in qualities.keys() if not k.startswith("_")], key=get_quality_priority):
                start_url = f"https://t.me/{bot_username}?start=batch_{show_id}_{q}"
                row.append(Button.url(f"📥 {q}", start_url))
                if len(row) == 3:
                    buttons.append(row)
                    row = []
            if row:
                buttons.append(row)
                
            channel_msg_id = qualities.get("_channel_msg_id")
            local_img_path = ""
            if img_url and not channel_msg_id:
                try:
                    img_r = requests.get(img_url, timeout=15)
                    if img_r.status_code == 200:
                        local_img_path = f"poster_{show_id}.jpg"
                        with open(local_img_path, "wb") as img_f:
                            img_f.write(img_r.content)
                except Exception:
                    pass
                    
            try:
                if channel_msg_id:
                    await client.edit_message(
                        config.CHANNEL_ID,
                        channel_msg_id,
                        caption,
                        buttons=buttons,
                        link_preview=False
                    )
                else:
                    if local_img_path and os.path.exists(local_img_path):
                        msg = await client.send_file(
                            config.CHANNEL_ID,
                            file=local_img_path,
                            caption=caption,
                            buttons=buttons,
                            link_preview=False
                        )
                        qualities["_channel_msg_id"] = msg.id
                        db.save_file_qualities(show_id, show_title, qualities)
                        try:
                            os.remove(local_img_path)
                        except Exception:
                            pass
                    else:
                        msg = await client.send_message(
                            config.CHANNEL_ID,
                            caption,
                            buttons=buttons,
                            link_preview=False
                        )
                        qualities["_channel_msg_id"] = msg.id
                        db.save_file_qualities(show_id, show_title, qualities)
                db.mark_posted(base_link, show_title)
                await status_msg.edit("Successfully processed all episodes and posted updates!")
            except Exception as post_err:
                logger.error(f"Failed to post show card: {str(post_err)}")
                await status_msg.edit(f"Error posting show card: {str(post_err)}")
                if local_img_path and os.path.exists(local_img_path):
                    try:
                        os.remove(local_img_path)
                    except Exception:
                        pass
        else:
            await status_msg.edit("All episodes and qualities for this show are already uploaded.")
    except Exception as e:
        logger.error(f"Error processing URL: {str(e)}")
        await event.respond(f"An error occurred: {str(e)}")

async def delete_messages_after_delay(chat_id, message_ids, delay=1800):
    await asyncio.sleep(delay)
    try:
        await client.delete_messages(chat_id, message_ids)
    except Exception as e:
        logger.error(f"Failed to delete batch messages: {str(e)}")

@client.on(events.NewMessage(pattern="/start"))
async def start_handler(event):
    text = event.text.strip()
    user_id = event.sender_id
    logger.info(f"Received /start from user ID: {user_id}")
    
    if " " in text:
        param = text.split(" ")[1]
        if param.startswith("batch_"):
            parts = param.split("_")
            if len(parts) >= 3:
                show_id = parts[1]
                quality = parts[2]
                logger.info(f"User requesting batch ID: {show_id} | Quality: {quality}")
                
                is_subbed = await check_user_sub(user_id)
                if not is_subbed:
                    logger.info(f"User ID: {user_id} is not subscribed to ForceSub channel.")
                    buttons = [
                        [Button.url("Join Channel", forcesub_invite_link)],
                        [Button.url("Try Again", f"https://t.me/{bot_username}?start={param}")]
                    ]
                    await event.respond(
                        "You must join our channel to get the download files. Please join and try again.",
                        buttons=buttons
                    )
                    return
                    
                file_data = db.get_file_qualities(show_id)
                if file_data and quality in file_data["qualities"]:
                    episodes = file_data["qualities"][quality]
                    if not episodes:
                        await event.respond("No episodes found for this quality.")
                        return
                        
                    loading_msg = await event.respond("**Preparing your batch files... Please wait.**")
                    episodes = sorted(episodes, key=lambda x: x["episode"])
                    sent_msg_ids = []
                    
                    subtitles = file_data["qualities"].get("_subtitles", "")
                    sub_line = f"🌐 **Subtitles Merged:** {subtitles}\n" if subtitles else ""
                    
                    for ep in episodes:
                        msg_id = ep.get("msg_id")
                        if msg_id:
                            try:
                                original_msg = await client.get_messages(config.LOG_CHANNEL, ids=msg_id)
                                if original_msg and original_msg.media:
                                    caption = (
                                        f"🎥 **{file_data['title']}** - Episode {ep['episode']:02d}\n"
                                        f"{sub_line}"
                                        f"💿 **Quality:** {quality}\n\n"
                                        f"📤 **Uploaded BY:** @MoviesFlixers_DL"
                                    )
                                    sent_msg = await client.send_file(
                                        event.chat_id,
                                        file=original_msg.media,
                                        caption=caption
                                    )
                                    sent_msg_ids.append(sent_msg.id)
                            except Exception as copy_err:
                                logger.error(f"Failed to copy batch message ID {msg_id}: {str(copy_err)}")
                                
                    await client.delete_messages(event.chat_id, [loading_msg.id])
                    
                    if sent_msg_ids:
                        delete_warn_msg = await event.respond(
                            "**Your files will be deleted after 30 Mins forward and save it.**",
                            buttons=[
                                [Button.url("Backup Channel", "https://t.me/KDramazFlix")]
                            ]
                        )
                        sent_msg_ids.append(delete_warn_msg.id)
                        client.loop.create_task(delete_messages_after_delay(event.chat_id, sent_msg_ids, delay=1800))
                else:
                    await event.respond("Batch files not found or link has expired.")
                return
            
    await event.respond(
        "Hello! I am the Asian Drama Auto Uploader Bot. "
        "I automatically monitor drama pages and manage files."
    )

@client.on(events.NewMessage(incoming=True))
async def message_handler(event):
    text = event.text.strip() if event.text else ""
    if not text or text.startswith("/"):
        return
    url_match = re.search(r'(https?://[^\s]+)', text)
    if url_match:
        url = url_match.group(1)
        if any(domain in url for domain in ["dramakey.com", "kissasia", "dramacool"]):
            client.loop.create_task(process_user_url(event, url))

async def start_bot():
    global bot_username, forcesub_invite_link
    logger.info("Starting Telethon Client bot...")
    await client.start(bot_token=config.BOT_TOKEN)
    me = await client.get_me()
    bot_username = me.username
    logger.info(f"Bot successfully started as @{bot_username}")
    
    if config.FORCESUB_CHANNEL:
        try:
            logger.info(f"Querying join invite link for ForceSub channel: {config.FORCESUB_CHANNEL}")
            invite = await client(ExportChatInviteRequest(config.FORCESUB_CHANNEL))
            forcesub_invite_link = invite.link
            logger.info(f"ForceSub invite link generated: {forcesub_invite_link}")
        except Exception as invite_err:
            logger.warning(f"Failed to auto-export invite link: {str(invite_err)}")
            forcesub_invite_link = config.FORCESUB_CHANNEL_LINK or ""
            
    await client.run_until_disconnected()
