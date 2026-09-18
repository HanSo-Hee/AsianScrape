/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package scraper

import (
	"net/http"
	"time"
)

type SubtitleTrack struct {
	Language string `json:"language"`
	URL      string `json:"url"`
}

type Episode struct {
	Episode        int               `json:"episode"`
	Title          string            `json:"title"`
	Link           string            `json:"link"`
	Qualities      map[string]string `json:"qualities"`
	Subtitles      string            `json:"subtitles"`
	SubtitleURL    string            `json:"subtitle_url"`
	SubtitleTracks []SubtitleTrack   `json:"subtitle_tracks"`
}

type ShowData struct {
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	Audio         string    `json:"audio"`
	TotalEpisodes int       `json:"total_episodes"`
	ImgURL        string    `json:"img_url"`
	Episodes      []Episode `json:"episodes"`
}

var client = &http.Client{
	Timeout: 20 * time.Second,
}
