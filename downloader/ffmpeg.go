/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package downloader

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type SubtitleInput struct {
	Path     string
	Language string
}

func GetVideoResolution(filepath string) (int, int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=s=x:p=0", filepath)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(strings.TrimSpace(string(output)), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid ffprobe output: %s", string(output))
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("failed to parse dimensions: %s", string(output))
	}
	return w, h, nil
}

func ProcessVideoQuality(ctx context.Context, inputPath string, subtitlePath string, outputPath string, targetHeight int) error {
	var subs []SubtitleInput
	if subtitlePath != "" {
		subs = append(subs, SubtitleInput{Path: subtitlePath, Language: "English"})
	}
	return ProcessVideoQualityMultiSub(ctx, inputPath, subs, outputPath, targetHeight)
}

func ProcessVideoQualityMultiSub(ctx context.Context, inputPath string, subtitles []SubtitleInput, outputPath string, targetHeight int) error {
	_, origH, err := GetVideoResolution(inputPath)
	args := []string{"-y", "-i", inputPath}
	for _, sub := range subtitles {
		if sub.Path != "" {
			args = append(args, "-i", sub.Path)
		}
	}

	vfFilters := []string{}
	if err == nil && origH > 0 && origH != targetHeight {
		vfFilters = append(vfFilters, fmt.Sprintf("scale=-2:%d", targetHeight))
	}

	if len(subtitles) > 0 {
		args = append(args, "-map", "0:v:0", "-map", "0:a:0?")
		for idx := range subtitles {
			args = append(args, "-map", fmt.Sprintf("%d:0", idx+1))
		}
		args = append(args, "-c:v", "copy", "-c:a", "copy", "-c:s", "srt")
		for idx, sub := range subtitles {
			langCode := "eng"
			langTitle := sub.Language
			if langTitle == "" {
				langTitle = "English"
			}
			lowerL := strings.ToLower(langTitle)
			if strings.Contains(lowerL, "ind") {
				langCode = "ind"
			} else if strings.Contains(lowerL, "spa") || strings.Contains(lowerL, "esp") {
				langCode = "spa"
			}
			args = append(args,
				fmt.Sprintf("-metadata:s:s:%d", idx), fmt.Sprintf("language=%s", langCode),
				fmt.Sprintf("-metadata:s:s:%d", idx), fmt.Sprintf("title=%s", langTitle),
			)
		}
		if len(vfFilters) > 0 {
			for i, arg := range args {
				if arg == "-c:v" && i+1 < len(args) {
					args[i+1] = "libx264"
				}
			}
			args = append(args, "-vf", strings.Join(vfFilters, ","), "-preset", "ultrafast", "-crf", "28")
		}
	} else {
		if len(vfFilters) > 0 {
			args = append(args, "-vf", strings.Join(vfFilters, ","), "-c:v", "libx264", "-preset", "ultrafast", "-crf", "28", "-c:a", "copy")
		} else {
			args = append(args, "-c", "copy")
		}
	}

	args = append(args, outputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	return cmd.Run()
}
