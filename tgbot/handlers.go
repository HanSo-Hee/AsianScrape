/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package tgbot

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

				audio, _ := qualities["_audio"].(string)
				audioLine := ""
				if audio != "" {
					audioLine = fmt.Sprintf("🔊 <b>Audio:</b> <code>%s</code>\n", audio)
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
									caption := fmt.Sprintf("📁 <b>File:</b> <code>%s</code>\n🎬 <b>Title:</b> <code>%s</code>\n🔢 <b>Episode:</b> <code>E%02d</code>\n💿 <b>Quality:</b> <code>%s</code>\n%s%s⚡ <b>Uploaded By:</b> @KDramaZFlix", docFileName, title, epNum, quality, audioLine, subLine)
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

					warnMsg, err := client.SendMessage(chatID, fmt.Sprintf("<b>Your files will be deleted after %d Mins. Forward and save it!</b>", delMinutes), &telegram.SendOptions{
						ReplyMarkup: markup,
					})
					if err == nil {
						sentMsgIDs = append(sentMsgIDs, warnMsg.ID)
					}

					db.Global.SaveScheduledDeletion(chatID, sentMsgIDs, time.Now().Add(time.Duration(delMinutes)*time.Minute))
					go deleteMessagesAfterDelay(client, chatID, sentMsgIDs, time.Duration(delMinutes)*time.Minute)
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

				warnMsg, err := client.SendMessage(chatID, fmt.Sprintf("<b>Your files will be deleted after %d Mins. Forward and save it!</b>", delMinutes), &telegram.SendOptions{
					ReplyMarkup: markup,
				})
				if err == nil {
					sentMsgIDs = append(sentMsgIDs, warnMsg.ID)
				}

				db.Global.SaveScheduledDeletion(chatID, sentMsgIDs, time.Now().Add(time.Duration(delMinutes)*time.Minute))
				go deleteMessagesAfterDelay(client, chatID, sentMsgIDs, time.Duration(delMinutes)*time.Minute)
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

func StartDeletionScheduler(client *telegram.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		tasks := db.Global.GetPendingDeletions()
		for _, task := range tasks {
			if len(task.MsgIDs) > 0 {
				_, _ = client.DeleteMessages(task.ChatID, task.MsgIDs, true)
			}
			db.Global.RemoveScheduledDeletion(task.ID)
		}
	}
}

func cancelHandler(client *telegram.Client, chatID int64) {
	if CancelTask(chatID) {
		_, _ = client.SendMessage(chatID, "🛑 <b>Current task has been cancelled!</b>\nTemporary files have been cleaned up.")
	} else {
		_, _ = client.SendMessage(chatID, "ℹ️ <i>No active download/upload task found to cancel.</i>")
	}
}

func getNetworkBandwidth() (rxBytes uint64, txBytes uint64) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) >= 9 {
			rx, _ := strconv.ParseUint(fields[0], 10, 64)
			tx, _ := strconv.ParseUint(fields[8], 10, 64)
			rxBytes += rx
			txBytes += tx
		}
	}
	return rxBytes, txBytes
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func statsHandler(client *telegram.Client, chatID int64) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(BotStartTime).Round(time.Second)

	var stat syscall.Statfs_t
	diskTotal := uint64(0)
	diskFree := uint64(0)
	diskUsed := uint64(0)
	diskPercent := 0.0
	if err := syscall.Statfs(".", &stat); err == nil {
		diskTotal = stat.Blocks * uint64(stat.Bsize)
		diskFree = stat.Bavail * uint64(stat.Bsize)
		if diskTotal > 0 {
			diskUsed = diskTotal - diskFree
			diskPercent = float64(diskUsed) / float64(diskTotal) * 100
		}
	}

	rxBytes, txBytes := getNetworkBandwidth()
	bwStr := ""
	if rxBytes > 0 || txBytes > 0 {
		bwStr = fmt.Sprintf(
			"🌐 <b>Network Bandwidth (Traffic):</b>\n"+
				"  • <b>Downloaded (RX):</b> <code>%s</code>\n"+
				"  • <b>Uploaded (TX):</b> <code>%s</code>\n"+
				"  • <b>Total Used:</b> <code>%s</code>\n\n",
			formatBytes(rxBytes),
			formatBytes(txBytes),
			formatBytes(rxBytes+txBytes),
		)
	}

	showsCount, postedCount, pendingDeletions, usersCount := db.Global.GetDBStats()

	statsText := fmt.Sprintf(
		"📊 <b>Bot & System Statistics</b>\n\n"+
			"⏱ <b>Uptime:</b> <code>%s</code>\n"+
			"🤖 <b>Goroutines:</b> <code>%d</code> | <b>CPUs:</b> <code>%d</code>\n\n"+
			"🧠 <b>Memory (RAM):</b>\n"+
			"  • <b>Allocated:</b> <code>%.2f MB</code>\n"+
			"  • <b>Total Alloc:</b> <code>%.2f MB</code>\n"+
			"  • <b>Sys (OS):</b> <code>%.2f MB</code>\n"+
			"  • <b>GC Cycles:</b> <code>%d</code>\n\n"+
			"%s"+
			"💾 <b>Disk Usage (Storage):</b>\n"+
			"  • <b>Total:</b> <code>%.2f GB</code>\n"+
			"  • <b>Used:</b> <code>%.2f GB (%.1f%%)</code>\n"+
			"  • <b>Free:</b> <code>%.2f GB</code>\n\n"+
			"👥 <b>Total Bot Users:</b> <code>%d</code>\n\n"+
			"🗄 <b>Database Records:</b>\n"+
			"  • <b>Stored Shows:</b> <code>%d</code>\n"+
			"  • <b>Posted Items:</b> <code>%d</code>\n"+
			"  • <b>Pending Auto-Deletes:</b> <code>%d</code>",
		uptime.String(),
		runtime.NumGoroutine(),
		runtime.NumCPU(),
		float64(mem.Alloc)/1024/1024,
		float64(mem.TotalAlloc)/1024/1024,
		float64(mem.Sys)/1024/1024,
		mem.NumGC,
		bwStr,
		float64(diskTotal)/(1024*1024*1024),
		float64(diskUsed)/(1024*1024*1024),
		diskPercent,
		float64(diskFree)/(1024*1024*1024),
		usersCount,
		showsCount,
		postedCount,
		pendingDeletions,
	)

	_, _ = client.SendMessage(chatID, statsText)
}

func isSudoUser(userID int64) bool {
	if len(config.Global.SudoUsers) == 0 {
		return true
	}
	for _, id := range config.Global.SudoUsers {
		if id == userID {
			return true
		}
	}
	return false
}

func broadcastHandler(client *telegram.Client, message *telegram.NewMessage) {
	senderID := message.SenderID()
	if !isSudoUser(senderID) {
		_, _ = client.SendMessage(message.ChatID(), "⚠️ <i>Access denied. Only bot administrators can broadcast.</i>")
		return
	}

	replyMsg, _ := message.GetReplyMessage()
	textParam := strings.TrimSpace(strings.TrimPrefix(message.Text(), "/broadcast"))
	textParam = strings.TrimSpace(strings.TrimPrefix(textParam, "/bcast"))

	if replyMsg == nil && textParam == "" {
		usage := "📢 <b>Broadcast Usage:</b>\n\n" +
			"1. <b>Reply to any message/media:</b>\n" +
			"   Reply to the photo, video, document, or text with <code>/broadcast</code>\n\n" +
			"2. <b>Direct text broadcast:</b>\n" +
			"   <code>/broadcast Hello everyone! New drama added...</code>"
		_, _ = client.SendMessage(message.ChatID(), usage)
		return
	}

	userIDs := db.Global.GetAllUserIDs()
	total := len(userIDs)
	if total == 0 {
		_, _ = client.SendMessage(message.ChatID(), "⚠️ <i>No bot users found in database to broadcast to.</i>")
		return
	}

	statusMsg, _ := client.SendMessage(message.ChatID(), fmt.Sprintf("🚀 <b>Starting broadcast to %d users...</b>", total))
	statusMsgID := int32(0)
	if statusMsg != nil {
		statusMsgID = statusMsg.ID
	}

	go func() {
		success := 0
		failed := 0
		blocked := 0

		startTime := time.Now()
		for i, uid := range userIDs {
			var err error
			if replyMsg != nil {
				_, err = replyMsg.ForwardTo(uid)
			} else {
				_, err = client.SendMessage(uid, textParam)
			}

			if err == nil {
				success++
			} else {
				errStr := strings.ToLower(err.Error())
				if strings.Contains(errStr, "blocked") || strings.Contains(errStr, "deactivated") || strings.Contains(errStr, "user_is_blocked") {
					blocked++
				} else {
					failed++
				}
			}

			if (i+1)%25 == 0 && statusMsgID > 0 {
				progressText := fmt.Sprintf(
					"📢 <b>Broadcast in Progress...</b>\n\n"+
						"👥 <b>Processed:</b> %d / %d (%.1f%%)\n"+
						"✅ <b>Success:</b> %d\n"+
						"🚫 <b>Blocked:</b> %d\n"+
						"❌ <b>Failed:</b> %d",
					i+1, total, float64(i+1)/float64(total)*100, success, blocked, failed,
				)
				_, _ = client.EditMessage(message.ChatID(), statusMsgID, progressText)
			}

			time.Sleep(25 * time.Millisecond)
		}

		elapsed := time.Since(startTime).Round(time.Second)
		finalReport := fmt.Sprintf(
			"✅ <b>Broadcast Completed!</b>\n\n"+
				"⏱ <b>Time Taken:</b> <code>%s</code>\n"+
				"👥 <b>Total Targets:</b> <code>%d</code>\n"+
				"✅ <b>Delivered:</b> <code>%d</code>\n"+
				"🚫 <b>Blocked/Deactivated:</b> <code>%d</code>\n"+
				"❌ <b>Failed:</b> <code>%d</code>",
			elapsed.String(), total, success, blocked, failed,
		)

		if statusMsgID > 0 {
			_, _ = client.EditMessage(message.ChatID(), statusMsgID, finalReport)
		} else {
			_, _ = client.SendMessage(message.ChatID(), finalReport)
		}
	}()
}



