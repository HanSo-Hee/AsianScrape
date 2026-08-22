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

import re
import cloudscraper
from bs4 import BeautifulSoup
import urllib3
from urllib.parse import urljoin

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

class BaseScraper:
    def __init__(self):
        self.scraper = cloudscraper.create_scraper()
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        }

    def fetch_soup(self, url):
        try:
            r = self.scraper.get(url, headers=self.headers, timeout=15)
            if r.status_code == 200:
                return BeautifulSoup(r.text, "html.parser")
        except Exception:
            pass
        return None

    def clean_title(self, title):
        title = re.sub(r'\(\d{4}\)', '', title)
        title = re.sub(r'[\W_]+', ' ', title)
        return title.strip().lower()

    def detect_qualities(self, links_list):
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

class DramakeyScraper(BaseScraper):
    def __init__(self, url="https://dramakey.com/"):
        super().__init__()
        self.url = url

    def get_latest(self):
        soup = self.fetch_soup(self.url)
        results = []
        if not soup:
            return results
        posts = soup.select(".eael-grid-post")[:10]
        for post in posts:
            title_a = post.select_one(".eael-entry-title a")
            img_el = post.select_one("img")
            if title_a:
                link = title_a.get("href")
                if link and not link.startswith("http"):
                    link = urljoin(self.url, link)
                show_title = title_a.text.strip()
                img_url = ""
                if img_el:
                    img_url = img_el.get("src") or img_el.get("data-src") or ""
                if img_url and not img_url.startswith("http"):
                    img_url = urljoin(self.url, img_url)
                
                sub_soup = self.fetch_soup(link)
                if not sub_soup:
                    continue
                
                ep_links = {}
                for a in sub_soup.find_all("a"):
                    href = a.get("href", "")
                    text = a.text.strip()
                    if "downloadwella" in href or "gofile" in href or "mega.nz" in href or "drive.google" in href:
                        if href and not href.startswith("http"):
                            href = urljoin(link, href)
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
                    results.append({
                        "title": f"{show_title} - {ep_key}",
                        "link": f"{link}#{ep_key.replace(' ', '_')}",
                        "img_url": img_url,
                        "source": "DramaKey",
                        "qualities": self.detect_qualities(links)
                    })
        return results

class KissasiaScraper(BaseScraper):
    def __init__(self, url="https://kissasia.biz/"):
        super().__init__()
        self.url = url

    def get_latest(self):
        soup = self.fetch_soup(self.url)
        results = []
        if not soup:
            return results
        posts = soup.select(".wp-block-post")[:10]
        
        blogger_feed = []
        try:
            feed_r = self.scraper.get("https://www.blogger.com/feeds/4927638411765974267/posts/default?alt=json", timeout=12)
            if feed_r.status_code == 200:
                blogger_feed = feed_r.json().get("feed", {}).get("entry", [])
        except Exception:
            pass
            
        for post in posts:
            title_a = post.select_one(".wp-block-post-title a")
            img_el = post.select_one("img.wp-post-image") or post.select_one("img")
            if title_a:
                link = title_a.get("href")
                if link and not link.startswith("http"):
                    link = urljoin(self.url, link)
                show_title = title_a.text.strip()
                img_url = ""
                if img_el:
                    img_url = img_el.get("src") or img_el.get("data-src") or ""
                if img_url and not img_url.startswith("http"):
                    img_url = urljoin(self.url, img_url)
                
                cleaned_show = self.clean_title(show_title)
                matched_entry = None
                for entry in blogger_feed:
                    b_title = entry.get("title", {}).get("$t", "")
                    if "-" in b_title:
                        b_show = b_title.split("-", 1)[1]
                    else:
                        b_show = b_title
                    cleaned_b = self.clean_title(b_show)
                    if cleaned_b in cleaned_show or cleaned_show in cleaned_b:
                        matched_entry = entry
                        break
                        
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
                            sub_parts = line.split("|")
                            if len(sub_parts) >= 2:
                                subtitles = sub_parts[1].strip()
                                
                            ep_num = idx + 1
                            ep_match = re.search(r"[Ee](\d+)", line) or re.search(r"episode[s]?[-_\s]?(\d+)", line, re.IGNORECASE) or re.search(r"ep[-_\s]?(\d+)", line, re.IGNORECASE)
                            if ep_match:
                                ep_num = int(ep_match.group(1))
                                
                            results.append({
                                "title": f"{show_title} - Episode {ep_num:02d}",
                                "link": f"{link}?episode={ep_num}",
                                "img_url": img_url,
                                "source": "KissAsia",
                                "subtitles": subtitles,
                                "qualities": {
                                    "720p": video_url
                                }
                            })
                else:
                    sub_soup = self.fetch_soup(link)
                    if sub_soup:
                        raw_links = []
                        for a in sub_soup.find_all("a"):
                            href = a.get("href", "")
                            text = a.text.strip()
                            if "gofile" in href or "mega.nz" in href or "drive.google" in href or "download" in href:
                                if href and not href.startswith("http"):
                                    href = urljoin(link, href)
                                raw_links.append((text, href))
                        if raw_links:
                            results.append({
                                "title": show_title,
                                "link": link,
                                "img_url": img_url,
                                "source": "KissAsia",
                                "qualities": self.detect_qualities(raw_links)
                            })
        return results

class DramacoolScraper(BaseScraper):
    def __init__(self, url="https://dramacool.sh/"):
        super().__init__()
        self.url = url

    def get_latest(self):
        soup = self.fetch_soup(self.url)
        results = []
        if not soup:
            return results
        items = soup.select("ul.box li")[:10]
        for item in items:
            a = item if item.name == "a" else item.find("a")
            if not a:
                continue
            link = a.get("href")
            if link and not link.startswith("http"):
                link = urljoin(self.url, link)
            title_el = item.find("h3") or item.find(class_="title") or a
            title = title_el.text.strip() if title_el else ""
            ep_num = item.find(class_="ep") or item.find(class_="ep sub")
            if ep_num:
                title = f"{title} - {ep_num.text.strip()}"
            img_el = item.find("img")
            img_url = ""
            if img_el:
                img_url = img_el.get("src") or img_el.get("data-src") or img_el.get("data-original") or ""
            if img_url and not img_url.startswith("http"):
                img_url = urljoin(self.url, img_url)
            
            raw_links = []
            if link:
                sub_soup = self.fetch_soup(link)
                if sub_soup:
                    for a_tag in sub_soup.find_all("a"):
                        href = a_tag.get("href", "")
                        text = a_tag.text.strip()
                        if "gofile" in href or "mega" in href or "download" in href:
                            if href and not href.startswith("http"):
                                href = urljoin(link, href)
                            raw_links.append((text, href))
                results.append({
                    "title": title,
                    "link": link,
                    "img_url": img_url,
                    "source": "DramaCool",
                    "qualities": self.detect_qualities(raw_links)
                })
        return results
