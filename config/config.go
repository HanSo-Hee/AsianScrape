/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	APIID               int
	APIHash             string
	BotToken            string
	MongoSRV            string
	ChannelID           string
	LogChannel          string
	BackupChannel       string
	ForcesubChannel     string
	ForcesubChannelLink string
	CloudflareBaseURL   string
	BotURL              string
	ButtonUpload        bool
	CheckInterval       int
	AutoDeleteMinutes   int
	SudoUsers           []int64
}

var Global *Config

func Load() {
	_ = godotenv.Load() // ignore error if .env doesn't exist (e.g. on Render)

	apiIDStr := os.Getenv("API_ID")
	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil {
		log.Fatalf("API_ID must be an integer: %v", err)
	}

	apiHash := os.Getenv("API_HASH")
	if apiHash == "" {
		log.Fatal("API_HASH is required")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN is required")
	}

	mongoSRV := os.Getenv("MONGO_SRV")
	if mongoSRV == "" {
		log.Fatal("MONGO_SRV is required")
	}

	channelID := os.Getenv("CHANNEL_ID")
	logChannel := os.Getenv("LOG_CHANNEL")
	if logChannel == "" {
		logChannel = channelID
	}
	backupChannel := os.Getenv("BACKUP_CHANNEL")
	forcesubChannel := os.Getenv("FORCESUB_CHANNEL")

	buttonUpload := true
	if val, exists := os.LookupEnv("BUTTON_UPLOAD"); exists {
		buttonUpload = val == "True" || val == "true"
	}

	checkInterval := 600
	if val, err := strconv.Atoi(os.Getenv("CHECK_INTERVAL")); err == nil {
		checkInterval = val
	}

	autoDeleteMinutes := 10
	if val, err := strconv.Atoi(os.Getenv("AUTO_DELETE_MINUTES")); err == nil && val > 0 {
		autoDeleteMinutes = val
	}

	botURL := os.Getenv("BOT_URL")
	if botURL == "" {
		botURL = os.Getenv("BOT_LINK")
	}
	if botURL == "" {
		botURL = os.Getenv("CLOUDFLARE_BASE_URL")
	}

	var sudoUsers []int64
	sudoStr := os.Getenv("SUDO_USERS")
	if sudoStr == "" {
		sudoStr = os.Getenv("ADMINS")
	}
	if sudoStr == "" {
		sudoStr = os.Getenv("OWNER_ID")
	}
	if sudoStr != "" {
		for _, part := range strings.FieldsFunc(sudoStr, func(r rune) bool {
			return r == ',' || r == ' ' || r == ';'
		}) {
			if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
				sudoUsers = append(sudoUsers, id)
			}
		}
	}

	Global = &Config{
		APIID:               apiID,
		APIHash:             apiHash,
		BotToken:            botToken,
		MongoSRV:            mongoSRV,
		ChannelID:           channelID,
		LogChannel:          logChannel,
		BackupChannel:       backupChannel,
		ForcesubChannel:     forcesubChannel,
		ForcesubChannelLink: os.Getenv("FORCESUB_CHANNEL_LINK"),
		CloudflareBaseURL:   botURL,
		BotURL:              botURL,
		ButtonUpload:        buttonUpload,
		CheckInterval:       checkInterval,
		AutoDeleteMinutes:   autoDeleteMinutes,
		SudoUsers:           sudoUsers,
	}
}

