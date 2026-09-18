/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package tgbot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"asianscraper/config"
	"asianscraper/downloader"
	"asianscraper/scraper"

	"github.com/amarnathcjd/gogram/telegram"
)

func downloadAndUploadDocument(ctx context.Context, client *telegram.Client, item scraper.Episode, quality string, directLink string, showTitle string, epNum int, imgURL string, audio string) (int32, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	directLink = scraper.UnwrapVideoURL(directLink)
	if strings.Contains(directLink, "downloadwella.com") {
		resolved := scraper.ResolveDownloadwellaLink(directLink)
		if resolved != directLink {
			directLink = resolved
		}
	}

	origName := directLink
	if parts := strings.Split(directLink, "/"); len(parts) > 0 {
		origName = strings.Split(parts[len(parts)-1], "?")[0]
	}

	origExt := filepath.Ext(origName)
	if origExt == "" {
		origExt = ".mp4"
	}

	tempInput := fmt.Sprintf("temp_input_%s_%d%s", quality, epNum, origExt)
	var tempFiles []string
	tempFiles = append(tempFiles, tempInput)

	var subInputs []downloader.SubtitleInput
	if len(item.SubtitleTracks) > 0 {
		for idx, track := range item.SubtitleTracks {
			tSub := fmt.Sprintf("temp_sub_%d_%d.vtt", epNum, idx)
			tempFiles = append(tempFiles, tSub)
			errSub := downloader.DownloadSubtitle(ctx, track.URL, tSub)
			if errSub == nil {
				subInputs = append(subInputs, downloader.SubtitleInput{
					Path:     tSub,
					Language: track.Language,
				})
			}
		}
	} else if item.SubtitleURL != "" {
		tSub := fmt.Sprintf("temp_subtitle_%d.vtt", epNum)
		tempFiles = append(tempFiles, tSub)
		errSub := downloader.DownloadSubtitle(ctx, item.SubtitleURL, tSub)
		if errSub == nil {
			subInputs = append(subInputs, downloader.SubtitleInput{
				Path:     tSub,
				Language: "English",
			})
		}
	}

	finalExt := origExt
	if len(subInputs) > 0 {
		finalExt = ".mkv"
	}

	localFilename := scraper.CleanFilename(fmt.Sprintf("%s E%02d %s%s", showTitle, epNum, quality, finalExt))
	tempFiles = append(tempFiles, localFilename)

	defer func() {
		for _, f := range tempFiles {
			if f != "" {
				_ = os.Remove(f)
			}
		}
	}()

	statusMsg, err := client.SendMessage(config.Global.LogChannel, fmt.Sprintf("<b>Initializing file download...</b>\n\n<b>File:</b> <code>%s</code>", localFilename))
	if err != nil {
		return 0, err
	}
	logMsgID := statusMsg.ID

	if fi, err := os.Stat(localFilename); err != nil || fi.Size() < 1024 {
		if fiIn, errIn := os.Stat(tempInput); errIn != nil || fiIn.Size() < 1024 {
			err = downloader.DownloadFileWithProgress(ctx, directLink, tempInput, func(current, total int64, speedBps float64) {
				bar := getProgressBar(current, total, speedBps)
				msgText := fmt.Sprintf("<b>Downloading Episode...</b>\n\n<b>File:</b> <code>%s</code>\n%s", localFilename, bar)
				_, _ = client.EditMessage(config.Global.LogChannel, logMsgID, msgText)
			})
			if err != nil {
				return 0, fmt.Errorf("download video failed: %w", err)
			}
		}

		if ctx.Err() != nil {
			return 0, ctx.Err()
		}

		_, _ = client.EditMessage(config.Global.LogChannel, logMsgID, fmt.Sprintf("<b>Processing video for %s...</b>\n\n<b>File:</b> <code>%s</code>", quality, localFilename))

		targetHeight := 720
		if strings.Contains(quality, "480") {
			targetHeight = 480
		} else if strings.Contains(quality, "1080") {
			targetHeight = 1080
		}

		err = downloader.ProcessVideoQualityMultiSub(ctx, tempInput, subInputs, localFilename, targetHeight)
		if err != nil {
			return 0, fmt.Errorf("ffmpeg processing failed: %w", err)
		}
	}

	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	subLine := ""
	var subLangs []string
	if len(item.SubtitleTracks) > 0 {
		for _, tr := range item.SubtitleTracks {
			if tr.Language != "" {
				subLangs = append(subLangs, tr.Language)
			}
		}
	} else if len(subInputs) > 0 {
		for _, sIn := range subInputs {
			if sIn.Language != "" {
				subLangs = append(subLangs, sIn.Language)
			}
		}
	}

	if len(subLangs) > 0 {
		subLine = fmt.Sprintf("🌐 <b>Subtitles:</b> <code>%s</code>\n", strings.Join(subLangs, ", "))
	} else if item.Subtitles != "" {
		subLine = fmt.Sprintf("🌐 <b>Subtitles:</b> <code>%s</code>\n", item.Subtitles)
	}

	audioLine := ""
	if audio != "" && !strings.EqualFold(audio, "unknown") {
		audioLine = fmt.Sprintf("🔊 <b>Audio:</b> <code>%s</code>\n", audio)
	}

	caption := fmt.Sprintf("📁 <b>File:</b> <code>%s</code>\n🎬 <b>Title:</b> <code>%s</code>\n🔢 <b>Episode:</b> <code>E%02d</code>\n💿 <b>Quality:</b> <code>%s</code>\n%s%s⚡ <b>Uploaded By:</b> @KDramaZFlix", localFilename, showTitle, epNum, quality, audioLine, subLine)

	var sentMsg *telegram.NewMessage
	var uploadErr error
	for uAttempt := 1; uAttempt <= 5; uAttempt++ {
		uploadStart := time.Now()
		opt := &telegram.MediaOptions{
			Caption:  caption,
			MimeType: "application/octet-stream",
			Attributes: []telegram.DocumentAttribute{
				&telegram.DocumentAttributeFilename{
					FileName: localFilename,
				},
			},
			Upload: &telegram.UploadOptions{
				Threads: 16,
				ProgressCallback: func(p *telegram.ProgressInfo) {
					elapsed := time.Since(uploadStart).Seconds()
					var speed float64
					if elapsed > 0 {
						speed = float64(p.Current) / elapsed
					}
					bar := getProgressBar(p.Current, p.TotalSize, speed)
					msgText := fmt.Sprintf("<b>Uploading Document File...</b>\n\n<b>File:</b> <code>%s</code>\n%s", localFilename, bar)
					_, _ = client.EditMessage(config.Global.LogChannel, logMsgID, msgText)
				},
			},
		}

		if fi, err := os.Stat("logo.png"); err == nil && fi.Size() > 0 {
			opt.Thumb = "logo.png"
		}

		sentMsg, uploadErr = client.SendMedia(config.Global.LogChannel, localFilename, opt)
		if uploadErr == nil && sentMsg != nil {
			break
		}

		wTime := parseFloodWait(uploadErr)
		if wTime > 0 {
			msgText := fmt.Sprintf("⏳ <b>Telegram FloodWait Limit Detected!</b>\nSleeping for %d seconds before resuming upload for:\n<code>%s</code>", wTime+1, localFilename)
			_, _ = client.EditMessage(config.Global.LogChannel, logMsgID, msgText)
			time.Sleep(time.Duration(wTime+1) * time.Second)
		} else {
			time.Sleep(3 * time.Second)
		}
	}

	if uploadErr != nil || sentMsg == nil {
		return 0, fmt.Errorf("upload to telegram failed: %w", uploadErr)
	}

	_, _ = client.DeleteMessages(config.Global.LogChannel, []int32{logMsgID}, true)
	return sentMsg.ID, nil
}

func deleteMessagesAfterDelay(client *telegram.Client, chatID int64, sentMsgIDs []int32, delay time.Duration) {
	time.Sleep(delay)
	_, _ = client.DeleteMessages(chatID, sentMsgIDs, true)
}
