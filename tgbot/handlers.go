/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package tgbot

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"asianscraper/config"
	"asianscraper/db"

	"github.com/amarnathcjd/gogram/telegram"
)

func sendWelcomeMessage(client *telegram.Client, chatID int64) {
	channelLink := os.Getenv("CHANNEL_LINK")
	if channelLink == "" {
		channelLink = "https://t.me/KDramazFlix"
	}
	supportLink := os.Getenv("SUPPORT_LINK")
	if supportLink == "" {
		supportLink = "https://t.me/TeleRoidGroup"
	}

	welcomeText := "👋 <b>Hello! Welcome to Asian Drama Auto Uploader & FileStore Bot!</b>\n\nI can search, scrape, process and deliver Asian dramas in multi-qualities.\n\n📢 <b>Updates Channel:</b> @KDramazFlix\n👥 <b>Support Group:</b> @TeleRoidGroup\n\n<i>Send me any DramaKey, KissAsia or DramaCool link to begin!</i>"

	markup := telegram.NewKeyboard().AddRow(
		telegram.Button.URL("📢 Updates Channel", channelLink),
		telegram.Button.URL("👥 Support Group", supportLink),
	).Build()

	_, _ = client.SendMessage(chatID, welcomeText, &telegram.SendOptions{
		ReplyMarkup: markup,
	})
}

func startHandler(client *telegram.Client, chatID int64, param string) {
	if param == "" {
		sendWelcomeMessage(client, chatID)
		return
	}

	if strings.HasPrefix(param, "batch_") {
		paramWithoutBatch := strings.TrimPrefix(param, "batch_")
		lastIdx := strings.LastIndex(paramWithoutBatch, "_")
		if lastIdx != -1 {
			showID := paramWithoutBatch[:lastIdx]
			quality := paramWithoutBatch[lastIdx+1:]

			isSubbed := checkUserSub(client, chatID)
			if !isSubbed {
				markup := telegram.NewKeyboard().AddRow(
					telegram.Button.URL("Join Channel", ForcesubInviteLink),
				).AddRow(
					telegram.Button.URL("Try Again", fmt.Sprintf("https://t.me/%s?start=%s", BotUsername, param)),
				).Build()

				_, _ = client.SendMessage(chatID, "You must join our channel to get the download files. Please join and try again.", &telegram.SendOptions{
					ReplyMarkup: markup,
				})
				return
			}

			title, qualities, found := db.Global.GetFileQualities(showID)
			if found {
				epListRaw, ok := qualities[quality].([]interface{})
				if !ok || len(epListRaw) == 0 {
					_, _ = client.SendMessage(chatID, "No episodes found for this quality.")
					return
				}

				loadingMsg, err := client.SendMessage(chatID, "<b>Preparing your batch files... Please wait.</b>")
				if err != nil {
					return
				}
				loadingMsgID := loadingMsg.ID

				subtitles, _ := qualities["_subtitles"].(string)
				subLine := ""
				if subtitles != "" {
					subLine = fmt.Sprintf("🌐 <b>Subtitles:</b> <code>%s</code>\n", subtitles)
				}

				var sentMsgIDs []int32
				for _, epRaw := range epListRaw {
					if epMap, ok := epRaw.(map[string]interface{}); ok {
						var epNum int
						var msgID int32
						if epFloat, ok := epMap["episode"].(float64); ok {
							epNum = int(epFloat)
						}
						if msgFloat, ok := epMap["msg_id"].(float64); ok {
							msgID = int32(msgFloat)
						} else if msgInt, ok := epMap["msg_id"].(int32); ok {
							msgID = int32(msgInt)
						} else if msgInt, ok := epMap["msg_id"].(int); ok {
							msgID = int32(msgInt)
						}

						if msgID > 0 {
							msgs, err := client.GetMessages(config.Global.LogChannel, &telegram.SearchOption{IDs: []int32{msgID}})
							if err == nil && len(msgs) > 0 {
								doc := msgs[0].Document()
								if doc != nil {
									docFileName := fmt.Sprintf("%s E%02d %s.mkv", title, epNum, quality)
									for _, attr := range doc.Attributes {
										if fn, ok := attr.(*telegram.DocumentAttributeFilename); ok {
											docFileName = fn.FileName
											break
										}
									}
									caption := fmt.Sprintf("📁 <b>File:</b> <code>%s</code>\n🎬 <b>Title:</b> <code>%s</code>\n🔢 <b>Episode:</b> <code>E%02d</code>\nCD <b>Quality:</b> <code>%s</code>\n%s\n⚡ <b>Uploaded By:</b> @MoviesFlixers_DL", docFileName, title, epNum, quality, subLine)
									sentMsg, err := client.SendMedia(chatID, doc, &telegram.MediaOptions{Caption: caption})
									if err == nil {
										sentMsgIDs = append(sentMsgIDs, sentMsg.ID)
									}
								}
							}
						}
					}
				}

				_, _ = client.DeleteMessages(chatID, []int32{loadingMsgID}, true)

				if len(sentMsgIDs) > 0 {
					channelLink := os.Getenv("CHANNEL_LINK")
					if channelLink == "" {
						channelLink = "https://t.me/KDramazFlix"
					}
					markup := telegram.NewKeyboard().AddRow(
						telegram.Button.URL("Backup Channel", channelLink),
					).Build()

					warnMsg, err := client.SendMessage(chatID, "<b>Your files will be deleted after 30 Mins forward and save it.</b>", &telegram.SendOptions{
						ReplyMarkup: markup,
					})
					if err == nil {
						sentMsgIDs = append(sentMsgIDs, warnMsg.ID)
					}

					go deleteMessagesAfterDelay(client, chatID, sentMsgIDs, 30*time.Minute)
				}
			} else {
				_, _ = client.SendMessage(chatID, "Batch files not found or link has expired.")
			}
			return
		}
	}

	decoded := decodeBase64(param)
	if strings.HasPrefix(decoded, "get-") {
		parts := strings.Split(strings.TrimPrefix(decoded, "get-"), "-")
		var msgIDs []int32
		if len(parts) == 1 {
			if id, err := strconv.Atoi(parts[0]); err == nil {
				msgIDs = append(msgIDs, int32(id))
			}
		} else if len(parts) >= 2 {
			startID, err1 := strconv.Atoi(parts[0])
			endID, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				if startID <= endID {
					for i := startID; i <= endID; i++ {
						msgIDs = append(msgIDs, int32(i))
					}
				} else {
					for i := startID; i >= endID; i-- {
						msgIDs = append(msgIDs, int32(i))
					}
				}
			}
		}

		if len(msgIDs) > 0 {
			isSubbed := checkUserSub(client, chatID)
			if !isSubbed {
				markup := telegram.NewKeyboard().AddRow(
					telegram.Button.URL("Join Channel", ForcesubInviteLink),
				).AddRow(
					telegram.Button.URL("Try Again", fmt.Sprintf("https://t.me/%s?start=%s", BotUsername, param)),
				).Build()

				_, _ = client.SendMessage(chatID, "You must join our channel to get the download files. Please join and try again.", &telegram.SendOptions{
					ReplyMarkup: markup,
				})
				return
			}

			loadingMsg, _ := client.SendMessage(chatID, "<b>Preparing your requested files... Please wait.</b>")
			var sentMsgIDs []int32

			for _, mID := range msgIDs {
				msgs, err := client.GetMessages(config.Global.LogChannel, &telegram.SearchOption{IDs: []int32{mID}})
				if err == nil && len(msgs) > 0 {
					doc := msgs[0].Document()
					if doc != nil {
						sentMsg, err := client.SendMedia(chatID, doc, &telegram.MediaOptions{Caption: msgs[0].Text()})
						if err == nil {
							sentMsgIDs = append(sentMsgIDs, sentMsg.ID)
						}
					}
				}
			}

			if loadingMsg != nil {
				_, _ = client.DeleteMessages(chatID, []int32{loadingMsg.ID}, true)
			}

			if len(sentMsgIDs) > 0 {
				channelLink := os.Getenv("CHANNEL_LINK")
				if channelLink == "" {
					channelLink = "https://t.me/KDramazFlix"
				}
				markup := telegram.NewKeyboard().AddRow(
					telegram.Button.URL("Backup Channel", channelLink),
				).Build()

				warnMsg, err := client.SendMessage(chatID, "<b>Your files will be deleted after 30 Mins forward and save it.</b>", &telegram.SendOptions{
					ReplyMarkup: markup,
				})
				if err == nil {
					sentMsgIDs = append(sentMsgIDs, warnMsg.ID)
				}

				go deleteMessagesAfterDelay(client, chatID, sentMsgIDs, 30*time.Minute)
			}
			return
		}
	}

	sendWelcomeMessage(client, chatID)
}

func deleteShowHandler(client *telegram.Client, chatID int64, param string) {
	param = strings.TrimSpace(param)
	if param == "" {
		_, _ = client.SendMessage(chatID, "<b>Usage:</b> <code>/delete &lt;show_name_or_url&gt;</code>\n<i>Example: /delete teach you a lesson</i>")
		return
	}

	cleanTitle := param
	if idx := strings.Index(cleanTitle, "#"); idx != -1 {
		cleanTitle = cleanTitle[:idx]
	}
	cleanTitle = strings.TrimPrefix(cleanTitle, "https://")
	cleanTitle = strings.TrimPrefix(cleanTitle, "http://")
	if strings.Contains(cleanTitle, "/") {
		parts := strings.Split(strings.Trim(cleanTitle, "/"), "/")
		cleanTitle = parts[len(parts)-1]
	}

	deleted := db.Global.DeleteShow(cleanTitle)
	if deleted {
		_, _ = client.SendMessage(chatID, fmt.Sprintf("✅ <b>Successfully deleted database records for '%s'!</b>\n\nYou can now send the drama URL to re-scrape and re-upload cleanly.", param))
	} else {
		_, _ = client.SendMessage(chatID, fmt.Sprintf("⚠️ <b>No matching database records found for '%s'.</b>", param))
	}
}
