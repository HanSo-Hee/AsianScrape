/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type ProgressCallback func(current, total int64, speedBps float64)

func DownloadFileWithProgress(ctx context.Context, urlStr string, filename string, cb ProgressCallback) error {
	if strings.Contains(strings.ToLower(urlStr), ".m3u8") {
		cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-headers", "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36\r\n", "-i", urlStr, "-c", "copy", filename)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("hls stream download failed: %v (%s)", err, string(output))
		}
		fi, err := os.Stat(filename)
		if err == nil && fi.Size() < 1024 {
			return fmt.Errorf("downloaded HLS video is empty (%d bytes)", fi.Size())
		}
		if cb != nil && err == nil {
			cb(fi.Size(), fi.Size(), 0)
		}
		return nil
	}

	client := &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 32,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	totalSize := resp.ContentLength
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	buffer := make([]byte, 4*1024*1024)
	var currentSize int64
	var lastSize int64
	startTime := time.Now()
	lastUpdate := time.Now()

	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
			currentSize += int64(n)

			if time.Since(lastUpdate) >= 1*time.Second || currentSize == totalSize {
				now := time.Now()
				intervalSec := now.Sub(lastUpdate).Seconds()
				var speed float64
				if intervalSec > 0 {
					speed = float64(currentSize-lastSize) / intervalSec
				}
				lastUpdate = now
				lastSize = currentSize
				if cb != nil {
					cb(currentSize, totalSize, speed)
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	if cb != nil {
		elapsed := time.Since(startTime).Seconds()
		var speed float64
		if elapsed > 0 {
			speed = float64(currentSize) / elapsed
		}
		cb(currentSize, totalSize, speed)
	}

	fi, err := os.Stat(filename)
	if err == nil && fi.Size() < 1024 {
		return fmt.Errorf("downloaded file is empty or invalid (%d bytes)", fi.Size())
	}

	return nil
}

func DownloadSubtitle(ctx context.Context, urlStr string, filename string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
