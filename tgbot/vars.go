/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package tgbot

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
)

var (
	Client             *telegram.Client
	BotUsername        string
	ForcesubInviteLink string
	BotStartTime       = time.Now()
	activeTaskCancels  sync.Map
)

func RegisterTask(chatID int64, cancel context.CancelFunc) {
	activeTaskCancels.Store(chatID, cancel)
}

func CancelTask(chatID int64) bool {
	if val, ok := activeTaskCancels.Load(chatID); ok {
		if cancel, okFn := val.(context.CancelFunc); okFn {
			cancel()
			activeTaskCancels.Delete(chatID)
			return true
		}
	}
	return false
}

func UnregisterTask(chatID int64) {
	activeTaskCancels.Delete(chatID)
}

func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.2f MB/s", bytesPerSec/(1024*1024))
	}
	if bytesPerSec >= 1024 {
		return fmt.Sprintf("%.2f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.0f B/s", bytesPerSec)
}

func getProgressBar(current, total int64, speedBps float64) string {
	if total <= 0 {
		return "Unknown size"
	}
	percentage := (float64(current) / float64(total)) * 100
	completed := int(percentage / 10)
	if completed > 10 {
		completed = 10
	}
	bar := strings.Repeat("■", completed) + strings.Repeat("□", 10-completed)
	speedStr := formatSpeed(speedBps)
	return fmt.Sprintf("[%s] %.1f%% (%.1fMB / %.1fMB) | ⚡ %s", bar, percentage, float64(current)/1024/1024, float64(total)/1024/1024, speedStr)
}

func parseFloodWait(err error) int {
	if err == nil {
		return 0
	}
	errStr := err.Error()
	re := regexp.MustCompile(`(?i)(?:FLOOD_WAIT_|wait\s+)(\d+)`)
	matches := re.FindStringSubmatch(errStr)
	if len(matches) > 1 {
		if sec, err := strconv.Atoi(matches[1]); err == nil {
			return sec
		}
	}
	return 0
}

func encodeBase64(str string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(str))
}

func decodeBase64(str string) string {
	b, err := base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		b, err = base64.StdEncoding.DecodeString(str)
		if err != nil {
			return ""
		}
	}
	return string(b)
}
