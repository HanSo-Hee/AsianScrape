# 🎬 Asian Drama Scraper & Telegram Bot

An automated, multi-threaded Go application designed for scraping, processing, and delivering Asian dramas to Telegram channels with multi-quality options, HLS stream containerization, and custom FileStore link generation.

---

## 👨‍💻 Author & Credits

* **Author:** Abhishek ([MrAbhi2k3](https://github.com/mrabhi2k3))
* **GitHub Repository:** [mrabhi2k3](https://github.com/mrabhi2k3)

### 🙏 Acknowledgements & Module Credits
Special thanks and credits to the open-source libraries that powered this project:
* **[Gogram](https://github.com/amarnathcjd/gogram)** by [@amarnathcjd](https://github.com/amarnathcjd) - MTProto client library for high-concurrency Telegram uploads.
* **[GoQuery](https://github.com/PuerkitoBio/goquery)** by [@PuerkitoBio](https://github.com/PuerkitoBio) - HTML document parsing and CSS selector engine for web scraping.
* **[Mongo Go Driver](https://github.com/mongodb/mongo-go-driver)** - Official MongoDB driver for persistent file storage.
* **[go-sqlite](https://github.com/glebarez/go-sqlite)** - Pure Go SQLite driver used for local fallback database operations.
* **[FFmpeg](https://ffmpeg.org/)** - Video containerization, subtitle embedding, and resolution scaling pipeline.

---

## ✨ Key Features

* 🚀 **High-Speed MTProto Uploads**: 16 concurrent worker threads per file for maximum bandwidth utilization (~10 MB/s sustained).
* 🛡️ **Flood-Wait Resilience**: Automatic detection and pause-retry logic for Telegram rate-limiting (`FLOOD_WAIT_X`).
* 🖼️ **Smart Poster Extraction**: Strictly filters out website logos (`kissasia.png`, site headers) and extracts legitimate show poster thumbnails.
* 📦 **Multi-Quality Support**: Processes and containerizes 480p, 720p, and 1080p streams with embedded English subtitles.
* 🌐 **Koyeb Deployable**: Embedded HTTP health check server on port `8080` to keep Koyeb deployments active 24/7 without sleeping.

---

## 🛠️ Environment Variables

Create a `.env` file based on `.env.example`:

```env
API_ID=123456
API_HASH=your_api_hash_here
BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrsTUVwxyZ
LOG_CHANNEL=-1001234567890
CHANNEL_ID=-1001234567890
FORCESUB_CHANNEL=-1001234567890
FORCESUB_CHANNEL_LINK=https://t.me/KDramazFlix
CHANNEL_LINK=https://t.me/KDramazFlix
SUPPORT_LINK=https://t.me/TeleRoidGroup
MONGODB_URI=mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority
PORT=8080
```

---

## 🚀 Koyeb Deployment Instructions

### Method 1: Docker Deployment (Recommended)
This repository includes an optimized multi-stage `Dockerfile` with `ffmpeg` built-in.

1. Create a new Service on **[Koyeb](https://app.koyeb.com/)**.
2. Select **GitHub** as deployment source and choose your repository.
3. In **Builder type**, select **Docker**.
4. Configure Dashboard Overrides:
   - **Dockerfile**: `Dockerfile`
   - **Build command**: *(Leave blank - auto-handled by Dockerfile)*
   - **Run command**: *(Leave blank - auto-handled by Dockerfile)*
   - **Work directory**: *(Leave blank / `/app`)*
5. Configure Environment Variables & Health Checks:
   - **Port**: `8080`
   - **Health check path**: `/health` (HTTP)
   - Add all environment variables listed in `.env.example`.

---

### Method 2: Buildpack Deployment (Alternative)
If using Koyeb's Buildpack builder instead of Docker:
- **Build command**: `go build -o app main.go`
- **Run command**: `./app`
- **Work directory**: `/`
- **Port**: `8080`
- **Health check path**: `/health`

---

Thank You! ❤️
