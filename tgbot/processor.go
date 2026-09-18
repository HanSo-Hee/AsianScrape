/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package tgbot

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"asianscraper/config"
	"asianscraper/db"
	"asianscraper/downloader"
	"asianscraper/scraper"

	"github.com/amarnathcjd/gogram/telegram"
)

func ProcessUserURL(client *telegram.Client, replyToChatID int64, replyToMsgID int32, urlStr string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	RegisterTask(replyToChatID, cancel)
	defer UnregisterTask(replyToChatID)

	cancelMarkup := telegram.NewKeyboard().AddRow(
		telegram.Button.Data("❌ Cancel", fmt.Sprintf("cancel_%d", replyToChatID)),
	).Build()

	sendOpt := &telegram.SendOptions{
		ReplyMarkup: cancelMarkup,
	}
	if replyToMsgID > 0 {
		sendOpt.ReplyTo = &telegram.InputReplyToMessage{ReplyToMsgID: replyToMsgID}
	}

	statusMsg, err := client.SendMessage(replyToChatID, "<b>Processing link... Please wait.</b>", sendOpt)
	if err != nil {
		log.Printf("Failed to send status message: %v", err)
		return
	}
	statusMsgID := statusMsg.ID

	cleanURL, source := scraper.GetShowPageAndSource(urlStr)
	showData, err := scraper.ScrapeShowData(cleanURL, source)
	if err != nil || showData == nil || len(showData.Episodes) == 0 {
		_, _ = client.EditMessage(replyToChatID, statusMsgID, fmt.Sprintf("❌ <b>Scraping failed:</b> %v", err))
		return
	}

	if ctx.Err() != nil {
		_, _ = client.EditMessage(replyToChatID, statusMsgID, "❌ <b>Process cancelled by user.</b>")
		return
	}

	showTitle := showData.Title
	showID := scraper.CleanShowTitle(showTitle)
	imgURL := showData.ImgURL
	episodes := showData.Episodes

	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].Episode < episodes[j].Episode
	})

	_, qualities, found := db.Global.GetFileQualities(showID)
	if !found {
		qualities = make(map[string]interface{})
	}

	audio := showData.Audio
	if audio == "" {
		if a, ok := qualities["_audio"].(string); ok {
			audio = a
		}
	}
	if audio != "" {
		qualities["_audio"] = audio
	}

	targetQualities := []string{"480p", "720p", "1080p"}
	updated := false
	var allDeliverableMsgIDs []int32

	for _, epItem := range episodes {
		epNum := epItem.Episode
		for _, q := range targetQualities {
			qURL, exists := epItem.Qualities[q]

			var existingMsgID int32 = 0
			if epListRaw, ok := qualities[q].([]interface{}); ok {
				for _, epRaw := range epListRaw {
					if epMap, okMap := epRaw.(map[string]interface{}); okMap {
						if intEp, okEp := epMap["episode"].(float64); okEp && int(intEp) == epNum {
							if mID, okM := epMap["msg_id"].(float64); okM {
								existingMsgID = int32(mID)
							} else if mID, okM := epMap["msg_id"].(int32); okM {
								existingMsgID = mID
							} else if mID, okM := epMap["msg_id"].(int); okM {
								existingMsgID = int32(mID)
							}
							break
						}
					}
				}
			}

			if existingMsgID > 0 {
				allDeliverableMsgIDs = append(allDeliverableMsgIDs, existingMsgID)
				continue
			}

			if !exists || qURL == "" {
				continue
			}

			if len(epItem.SubtitleTracks) > 0 {
				var langs []string
				for _, tr := range epItem.SubtitleTracks {
					if tr.Language != "" {
						langs = append(langs, tr.Language)
					}
				}
				if len(langs) > 0 {
					qualities["_subtitles"] = strings.Join(langs, ", ")
				} else if epItem.Subtitles != "" {
					qualities["_subtitles"] = epItem.Subtitles
				}
			} else if epItem.Subtitles != "" {
				qualities["_subtitles"] = epItem.Subtitles
			}

			var msgID int32
			var uploadErr error
			for attempt := 1; attempt <= 5; attempt++ {
				if ctx.Err() != nil {
					_, _ = client.EditMessage(replyToChatID, statusMsgID, "❌ <b>Process cancelled by user.</b>")
					return
				}
				msgID, uploadErr = downloadAndUploadDocument(ctx, client, epItem, q, qURL, showTitle, epNum, imgURL, audio)
				if uploadErr == nil && msgID > 0 {
					break
				}
				if ctx.Err() != nil {
					_, _ = client.EditMessage(replyToChatID, statusMsgID, "❌ <b>Process cancelled by user.</b>")
					return
				}
				log.Printf("Attempt %d/5 failed for %s E%02d %s: %v. Retrying...", attempt, showTitle, epNum, q, uploadErr)
				waitTime := parseFloodWait(uploadErr)
				if waitTime > 0 {
					log.Printf("FloodWait detected! Sleeping for %d seconds before retrying...", waitTime+1)
					time.Sleep(time.Duration(waitTime+1) * time.Second)
				} else {
					time.Sleep(3 * time.Second)
				}
			}

			if uploadErr == nil && msgID > 0 {
				var epList []interface{}
				if existingList, ok := qualities[q].([]interface{}); ok {
					epList = existingList
				}
				epList = append(epList, map[string]interface{}{"episode": epNum, "msg_id": msgID})
				qualities[q] = epList
				db.Global.SaveFileQualities(showID, showTitle, qualities)
				db.Global.MarkPosted(epItem.Link, epItem.Title)
				updated = true
				allDeliverableMsgIDs = append(allDeliverableMsgIDs, msgID)
			} else {
				log.Printf("Failed to process episode %d (%s) after 5 attempts: %v", epNum, q, uploadErr)
			}
		}
	}

	channelMsgIDRaw, hasMsg := qualities["_channel_msg_id"]
	var channelMsgID int32
	if hasMsg {
		if f, ok := channelMsgIDRaw.(float64); ok {
			channelMsgID = int32(f)
		} else if i, ok := channelMsgIDRaw.(int32); ok {
			channelMsgID = i
		} else if i, ok := channelMsgIDRaw.(int); ok {
			channelMsgID = int32(i)
		}
	}

	if showData.Status != "" {
		qualities["_status"] = showData.Status
	}

	if updated || !found || channelMsgID == 0 {
		db.Global.SaveFileQualities(showID, showTitle, qualities)

		subtitles, _ := qualities["_subtitles"].(string)
		subLine := ""
		if subtitles != "" {
			subLine = fmt.Sprintf("🌐 <b>Subtitles:</b> <code>%s</code>\n", subtitles)
		}

		totalCount := showData.TotalEpisodes
		if totalCount == 0 {
			totalCount = len(episodes)
		}

		epLine := ""
		if len(episodes) == 1 {
			epLine = fmt.Sprintf("🔢 <b>Episode:</b> <code>E%02d</code>\n", episodes[0].Episode)
		} else if len(episodes) > 1 {
			firstEp := episodes[0].Episode
			lastEp := episodes[len(episodes)-1].Episode
			if totalCount > 0 {
				epLine = fmt.Sprintf("🔢 <b>Episodes:</b> <code>E%02d - E%02d</code> (Total %d Episodes)\n", firstEp, lastEp, totalCount)
			} else {
				epLine = fmt.Sprintf("🔢 <b>Episodes:</b> <code>E%02d - E%02d</code>\n", firstEp, lastEp)
			}
		}

		statusVal, _ := qualities["_status"].(string)
		if statusVal == "" {
			statusVal = "Ongoing"
		}
		statusQuote := fmt.Sprintf("<blockquote>Status: %s</blockquote>\n", statusVal)

		audioLine := ""
		if audio != "" && !strings.EqualFold(audio, "unknown") {
			audioLine = fmt.Sprintf("🔊 <b>Audio:</b> <code>%s</code>\n", audio)
		}

		caption := fmt.Sprintf("🎬 <b>NEW DRAMA RELEASED</b> 🎬\n\n📌 <b>Title:</b> <code>%s</code>\n%s%s%s%s\n👇 <b>Download Episodes via Buttons Below:</b>\n\n⚡ <b>Uploaded By:</b> @KDramaZFlix", showTitle, epLine, audioLine, subLine, statusQuote)

		cleanID := regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(showID, "_")
		kb := telegram.NewKeyboard()
		sortedQs := []string{"480p", "720p", "1080p"}
		var buttons []telegram.KeyboardButton
		for _, q := range sortedQs {
			if _, ok := qualities[q]; ok {
				param := fmt.Sprintf("batch_%s_%s", cleanID, q)
				startURL := ""
				baseURL := config.Global.BotURL
				if baseURL == "" {
					baseURL = config.Global.CloudflareBaseURL
				}
				if baseURL != "" {
					baseURL = strings.TrimRight(baseURL, "?&")
					if strings.Contains(baseURL, "?") {
						startURL = fmt.Sprintf("%s&start=%s", baseURL, param)
					} else {
						startURL = fmt.Sprintf("%s?start=%s", baseURL, param)
					}
				} else {
					botUser := BotUsername
					if botUser == "" {
						botUser = "bot"
					}
					startURL = fmt.Sprintf("https://t.me/%s?start=%s", botUser, param)
				}
				buttons = append(buttons, telegram.Button.URL("📥 "+q, startURL))
			}
		}

		var markup telegram.ReplyMarkup
		if len(buttons) > 0 {
			kb.NewRow(len(buttons), buttons...)
			markup = kb.Build()
		}

		posted := false

		localImgPath := ""
		if imgURL != "" && channelMsgID == 0 {
			imgLower := strings.ToLower(imgURL)
			if !strings.Contains(imgLower, "logo") && !strings.Contains(imgLower, "kissasia.png") {
				localImgPath = fmt.Sprintf("poster_%s.jpg", cleanID)
				err = downloader.DownloadSubtitle(context.Background(), imgURL, localImgPath)
				if err != nil {
					localImgPath = ""
				}
			}
		}

		if config.Global.ChannelID != "" {
			if channelMsgID > 0 {
				_, err = client.EditMessage(config.Global.ChannelID, channelMsgID, caption, &telegram.SendOptions{ReplyMarkup: markup})
				if err == nil {
					posted = true
				} else {
					log.Printf("Failed to edit channel message %d in channel %s: %v", channelMsgID, config.Global.ChannelID, err)
				}
			} else {
				var sentMsg *telegram.NewMessage
				if localImgPath != "" {
					sentMsg, err = client.SendMedia(config.Global.ChannelID, localImgPath, &telegram.MediaOptions{
						Caption:     caption,
						ReplyMarkup: markup,
					})
				}

				if sentMsg == nil {
					sentMsg, err = client.SendMessage(config.Global.ChannelID, caption, &telegram.SendOptions{ReplyMarkup: markup})
				}

				if err == nil && sentMsg != nil {
					channelMsgID = sentMsg.ID
					qualities["_channel_msg_id"] = channelMsgID
					db.Global.SaveFileQualities(showID, showTitle, qualities)
					posted = true
				} else {
					log.Printf("Failed to post card to channel %s: %v", config.Global.ChannelID, err)
				}
			}
		}

		if config.Global.LogChannel != "" && config.Global.LogChannel != config.Global.ChannelID {
			if localImgPath != "" {
				_, err = client.SendMedia(config.Global.LogChannel, localImgPath, &telegram.MediaOptions{
					Caption:     caption,
					ReplyMarkup: markup,
				})
			} else {
				_, err = client.SendMessage(config.Global.LogChannel, caption, &telegram.SendOptions{ReplyMarkup: markup})
			}
			if err != nil {
				log.Printf("Failed to post card to log channel %s: %v", config.Global.LogChannel, err)
			}
		}

		if localImgPath != "" {
			_ = os.Remove(localImgPath)
		}

		if posted {
			_, _ = client.EditMessage(replyToChatID, statusMsgID, fmt.Sprintf("<b>Scrape & Upload complete for %s!</b>\nDelivering episode files...", showTitle))
		} else {
			log.Printf("Notice: Channel card was not sent to channel ID %d (Verify CHANNEL_ID env var & bot admin rights).", config.Global.ChannelID)
		}
	}

	if len(allDeliverableMsgIDs) > 0 {
		deliveredCount := 0
		var sentMsgIDs []int32
		for _, mID := range allDeliverableMsgIDs {
			msgs, err := client.GetMessages(config.Global.LogChannel, &telegram.SearchOption{IDs: []int32{mID}})
			if err == nil && len(msgs) > 0 {
				doc := msgs[0].Document()
				if doc != nil {
					sentMsg, errSend := client.SendMedia(replyToChatID, doc, &telegram.MediaOptions{Caption: msgs[0].Text()})
					if errSend == nil && sentMsg != nil {
						sentMsgIDs = append(sentMsgIDs, sentMsg.ID)
						deliveredCount++
					}
				}
			}
		}
		_, _ = client.EditMessage(replyToChatID, statusMsgID, fmt.Sprintf("✅ <b>Delivered %d episode file(s) from database archive!</b>", deliveredCount))

		if len(sentMsgIDs) > 0 {
			delMinutes := config.Global.AutoDeleteMinutes
			if delMinutes <= 0 {
				delMinutes = 10
			}
			channelLink := os.Getenv("CHANNEL_LINK")
			if channelLink == "" {
				channelLink = "https://t.me/KDramazFlix"
			}
			markup := telegram.NewKeyboard().AddRow(
				telegram.Button.URL("Backup Channel", channelLink),
			).Build()

			warnMsg, errWarn := client.SendMessage(replyToChatID, fmt.Sprintf("<b>Your files will be deleted after %d Mins. Forward and save it!</b>", delMinutes), &telegram.SendOptions{
				ReplyMarkup: markup,
			})
			if errWarn == nil && warnMsg != nil {
				sentMsgIDs = append(sentMsgIDs, warnMsg.ID)
			}

			db.Global.SaveScheduledDeletion(replyToChatID, sentMsgIDs, time.Now().Add(time.Duration(delMinutes)*time.Minute))
			go deleteMessagesAfterDelay(client, replyToChatID, sentMsgIDs, time.Duration(delMinutes)*time.Minute)
		}
	} else {
		_, _ = client.EditMessage(replyToChatID, statusMsgID, "No new or existing episode files available for this query.")
	}
}
