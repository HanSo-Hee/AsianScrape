/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func getHTML(targetURL string) (string, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func GetShowPageAndSource(inputURL string) (string, string) {
	lower := strings.ToLower(inputURL)
	source := "kissasia"
	if strings.Contains(lower, "dramacool") {
		source = "dramacool"
	} else if strings.Contains(lower, "dramakey") {
		source = "dramakey"
	}

	cleanURL := inputURL
	if idx := strings.Index(cleanURL, "#"); idx != -1 {
		cleanURL = cleanURL[:idx]
	}
	return cleanURL, source
}

func CleanShowTitle(title string) string {
	reList := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\(Episodes?\s*[\d\s&,-]+\s*Added\)`),
		regexp.MustCompile(`(?i)\(Episodes?\s*[\d\s&,-]+\s*Complete\)`),
		regexp.MustCompile(`(?i)\(Complete\)`),
		regexp.MustCompile(`(?i)\(?dramakey\.com\)?`),
		regexp.MustCompile(`(?i)\(?kissasia\.biz\)?`),
		regexp.MustCompile(`(?i)\(?dramacool\.sh\)?`),
		regexp.MustCompile(`(?i)\(?dramacool\.bg\)?`),
		regexp.MustCompile(`(?i)\(?moviesflixers_dl\)?`),
		regexp.MustCompile(`\[.*?\]`),
	}

	cleaned := title
	for _, re := range reList {
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	reSpace := regexp.MustCompile(`[\._-]`)
	cleaned = reSpace.ReplaceAllString(cleaned, " ")

	reMultiSpace := regexp.MustCompile(`\s+`)
	cleaned = strings.TrimSpace(reMultiSpace.ReplaceAllString(cleaned, " "))

	return cleaned
}

func ExtractEpisodeNumber(text string) int {
	if text == "" {
		return 0
	}
	reExplicit := regexp.MustCompile(`(?i)\b(?:episodes?|ep|e)[-_\s]?(\d{1,4})\b`)
	matches := reExplicit.FindStringSubmatch(text)
	if len(matches) > 1 {
		for _, m := range matches[1:] {
			if m != "" {
				if val, err := strconv.Atoi(m); err == nil && val > 0 {
					return val
				}
			}
		}
	}
	return 0
}

func detectQualities(links []struct{ text, href string }) map[string]string {
	qualities := make(map[string]string)
	for _, link := range links {
		combined := strings.ToLower(link.text + " " + link.href)
		var q string
		if strings.Contains(combined, "1080") {
			q = "1080p"
		} else if strings.Contains(combined, "720") {
			q = "720p"
		} else if strings.Contains(combined, "480") || strings.Contains(combined, "360") {
			q = "480p"
		} else {
			if _, exists := qualities["480p"]; !exists {
				q = "480p"
			} else if _, exists := qualities["720p"]; !exists {
				q = "720p"
			} else {
				q = "1080p"
			}
		}
		qualities[q] = link.href
	}
	return qualities
}

func ExtractTargetEpisodeNumber(targetURL string) int {
	if parsed, err := url.Parse(targetURL); err == nil {
		if qEp := parsed.Query().Get("episode"); qEp != "" {
			if val, err := strconv.Atoi(qEp); err == nil {
				return val
			}
		}
	}
	re := regexp.MustCompile(`(?i)[-_\s/]episode[-_\s]?(\d+)|[-_\s/]ep[-_\s]?(\d+)`)
	matches := re.FindStringSubmatch(targetURL)
	if len(matches) > 1 {
		for _, m := range matches[1:] {
			if m != "" {
				if val, err := strconv.Atoi(m); err == nil {
					return val
				}
			}
		}
	}
	return 0
}

func UnwrapVideoURL(raw string) string {
	return raw
}

func ResolveDownloadwellaLink(raw string) string {
	return raw
}

func CleanFilename(filename string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|]`)
	clean := re.ReplaceAllString(filename, "")
	clean = strings.TrimSpace(clean)
	ext := filepath.Ext(clean)
	base := strings.TrimSuffix(clean, ext)
	return fmt.Sprintf("%s [@KDramazFlix]%s", base, ext)
}
