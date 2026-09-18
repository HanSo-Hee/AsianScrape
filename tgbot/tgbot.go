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
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"asianscraper/config"
	"asianscraper/db"

	"github.com/amarnathcjd/gogram/telegram"
)

func StartBot(ctx context.Context) {
	tempFiles, globErr := filepath.Glob("temp_*")
	if globErr == nil {
		for _, f := range tempFiles {
			_ = os.Remove(f)
		}
	}

	log.Println("Starting Go Telethon Client bot (using gogram)...")

	client, err := telegram.NewClient(telegram.ClientConfig{
		AppID:    int32(config.Global.APIID),
		AppHash:  config.Global.APIHash,
		LogLevel: telegram.LogInfo,
	})
	if err != nil {
		log.Fatalf("Failed to create Gogram client: %v", err)
	}

	_, err = client.Conn()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	err = client.LoginBot(config.Global.BotToken)
	if err != nil {
		log.Fatalf("Failed to login bot: %v", err)
	}

	me, err := client.GetMe()
	if err != nil {
		log.Fatalf("Failed to get bot info: %v", err)
	}
	BotUsername = me.Username
	log.Printf("Bot successfully started as @%s", BotUsername)

	go StartDeletionScheduler(client)

	VerifyChannels(client)

	if config.Global.ForcesubChannel != "" && config.Global.ForcesubChannel != "0" {
		resp, err := client.ExportInvite(config.Global.ForcesubChannel)
		if err == nil {
			if invite, ok := resp.(*telegram.ChatInviteExported); ok {
				ForcesubInviteLink = invite.Link
				log.Printf("ForceSub invite link generated: %s", ForcesubInviteLink)
			}
		}
		if ForcesubInviteLink == "" {
			ForcesubInviteLink = config.Global.ForcesubChannelLink
		}
	}

	client.On(telegram.OnMessage, func(message *telegram.NewMessage) error {
		if message.IsOutgoing() {
			return nil
		}

		text := strings.TrimSpace(message.Text())
		if text == "" {
			return nil
		}

		chatID := message.ChatID()
		senderID := message.SenderID()
		if senderID > 0 {
			username := ""
			if sender, err := message.GetSender(); err == nil && sender != nil {
				username = sender.Username
			}
			go db.Global.SaveUser(senderID, username)
		}

		if strings.HasPrefix(text, "/start") {
			parts := strings.SplitN(text, " ", 2)
			param := ""
			if len(parts) > 1 {
				param = strings.TrimSpace(parts[1])
			}
			startHandler(client, chatID, param)
			return nil
		}

		if strings.HasPrefix(text, "/cancel") {
			if !isSudoUser(senderID) {
				_, _ = client.SendMessage(chatID, "⚠️ <i>Access denied. This command is restricted to administrators.</i>")
				return nil
			}
			cancelHandler(client, chatID)
			return nil
		}

		if strings.HasPrefix(text, "/stats") {
			if !isSudoUser(senderID) {
				_, _ = client.SendMessage(chatID, "⚠️ <i>Access denied. This command is restricted to administrators.</i>")
				return nil
			}
			statsHandler(client, chatID)
			return nil
		}

		if strings.HasPrefix(text, "/broadcast") || strings.HasPrefix(text, "/bcast") {
			broadcastHandler(client, message)
			return nil
		}

		if strings.HasPrefix(text, "/delete") || strings.HasPrefix(text, "/delshow") || strings.HasPrefix(text, "/del") {
			if !isSudoUser(senderID) {
				_, _ = client.SendMessage(chatID, "⚠️ <i>Access denied. This command is restricted to administrators.</i>")
				return nil
			}
			parts := strings.SplitN(text, " ", 2)
			param := ""
			if len(parts) > 1 {
				param = strings.TrimSpace(parts[1])
			}
			deleteShowHandler(client, chatID, param)
			return nil
		}

		if strings.HasPrefix(text, "/genlink") {
			parts := strings.Fields(text)
			if len(parts) < 2 {
				_, _ = client.SendMessage(chatID, "<b>Usage:</b> <code>/genlink [msg_id]</code>")
				return nil
			}
			msgID := parts[1]
			base64Str := encodeBase64("get-" + msgID)
			shareLink := fmt.Sprintf("https://t.me/%s?start=%s", BotUsername, base64Str)
			markup := telegram.NewKeyboard().AddRow(
				telegram.Button.URL("🔁 Share URL", fmt.Sprintf("https://telegram.me/share/url?url=%s", shareLink)),
			).Build()
			_, _ = client.SendMessage(chatID, fmt.Sprintf("<b>Here is your link:</b>\n\n%s", shareLink), &telegram.SendOptions{ReplyMarkup: markup})
			return nil
		}

		if strings.HasPrefix(text, "/batch") {
			parts := strings.Fields(text)
			if len(parts) < 3 {
				_, _ = client.SendMessage(chatID, "<b>Usage:</b> <code>/batch [start_msg_id] [end_msg_id]</code>")
				return nil
			}
			startID := parts[1]
			endID := parts[2]
			base64Str := encodeBase64(fmt.Sprintf("get-%s-%s", startID, endID))
			shareLink := fmt.Sprintf("https://t.me/%s?start=%s", BotUsername, base64Str)
			markup := telegram.NewKeyboard().AddRow(
				telegram.Button.URL("🔁 Share URL", fmt.Sprintf("https://telegram.me/share/url?url=%s", shareLink)),
			).Build()
			_, _ = client.SendMessage(chatID, fmt.Sprintf("<b>Here is your batch link:</b>\n\n%s", shareLink), &telegram.SendOptions{ReplyMarkup: markup})
			return nil
		}

		urlRegex := regexp.MustCompile(`(https?://[^\s]+)`)
		match := urlRegex.FindString(text)
		if match != "" {
			urlStr := match
			if strings.Contains(urlStr, "dramakey.com") || strings.Contains(urlStr, "kissasia") || strings.Contains(urlStr, "dramacool") || strings.Contains(urlStr, "kdhindidubbed") {
				go ProcessUserURL(client, chatID, message.ID, urlStr)
			}
		}

		return nil
	})

	client.On(telegram.OnCallbackQuery, func(cb *telegram.CallbackQuery) error {
		data := cb.DataString()
		if strings.HasPrefix(data, "cancel_") {
			targetChatStr := strings.TrimPrefix(data, "cancel_")
			targetChatID, err := strconv.ParseInt(targetChatStr, 10, 64)
			if err != nil {
				targetChatID = cb.GetChatID()
			}
			if CancelTask(targetChatID) {
				_, _ = cb.Answer("Task cancelled successfully!", &telegram.CallbackOptions{Alert: true})
				_, _ = cb.Edit("🛑 <b>Download/Upload cancelled by user.</b>")
			} else {
				_, _ = cb.Answer("No active task to cancel or already completed.", &telegram.CallbackOptions{Alert: false})
			}
		}
		return nil
	})

	go func() {
		<-ctx.Done()
		log.Println("Context cancelled, stopping client...")
		_ = client.Stop()
	}()

	client.Idle()
}
