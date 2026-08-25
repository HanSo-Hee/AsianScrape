/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package tgbot

import (
	"fmt"
	"log"
	"strings"

	"asianscraper/config"

	"github.com/amarnathcjd/gogram/telegram"
)

func checkUserSub(client *telegram.Client, userID int64) bool {
	if config.Global.ForcesubChannel == "" || config.Global.ForcesubChannel == "0" {
		return true
	}

	chPeer, err := client.GetSendableChannel(config.Global.ForcesubChannel)
	if err != nil {
		return true
	}

	userPeer, err := client.ResolvePeer(userID)
	if err != nil {
		return true
	}

	_, err = client.ChannelsGetParticipant(chPeer, userPeer)
	if err != nil {
		errStr := strings.ToUpper(err.Error())
		if strings.Contains(errStr, "USER_NOT_PARTICIPANT") || strings.Contains(errStr, "PARTICIPANT_ID_INVALID") {
			return false
		}
		return true
	}

	return true
}

func VerifyChannels(client *telegram.Client) {
	log.Println("------ VERIFYING CHANNELS PERMISSIONS ------")
	channelsToVerify := []struct {
		name  string
		value string
	}{
		{"CHANNEL_ID (Main Updates)", config.Global.ChannelID},
		{"LOG_CHANNEL (Media Archives)", config.Global.LogChannel},
		{"BACKUP_CHANNEL (Backups)", config.Global.BackupChannel},
		{"FORCESUB_CHANNEL (Force Subscription)", config.Global.ForcesubChannel},
	}

	for _, chInfo := range channelsToVerify {
		if chInfo.value == "" || chInfo.value == "0" {
			continue
		}

		testText := fmt.Sprintf("⚠️ **[BOT TEST MESSAGE]** Verification of permissions for %s. This message will be deleted immediately.", chInfo.name)
		msg, err := client.SendMessage(chInfo.value, testText)
		if err != nil {
			continue
		}

		if msg != nil && msg.ID > 0 {
			_, delErr := client.DeleteMessages(chInfo.value, []int32{msg.ID}, true)
			if delErr == nil {
				log.Printf("[✅ SUCCESS] Verified %s ('%s') - Write & Delete permissions are OK!", chInfo.name, chInfo.value)
			}
		}
	}
	log.Println("--------------------------------------------")
}
