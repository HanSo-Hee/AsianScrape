/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package scraper

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
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

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

func postForm(targetURL string, data url.Values) (string, error) {
	req, err := http.NewRequest("POST", targetURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}

func UnwrapVideoURL(videoURL string) string {
	if strings.Contains(videoURL, "cdnvideo.autos/media/") || strings.Contains(videoURL, "cdnvideo") {
		re := regexp.MustCompile(`/media/([A-Za-z0-9+/=]+)(?:\.mp4)?`)
		match := re.FindStringSubmatch(videoURL)
		if len(match) > 1 {
			b64Str := match[1]
			missingPadding := len(b64Str) % 4
			if missingPadding > 0 {
				b64Str += strings.Repeat("=", 4-missingPadding)
			}
			decoded, err := base64.StdEncoding.DecodeString(b64Str)
			if err == nil {
				decStr := string(decoded)
				if strings.HasPrefix(decStr, "http") {
					return decStr
				}
			}
		}
	}
	return videoURL
}

func ResolveDownloadwellaLink(urlStr string) string {
	if !strings.Contains(urlStr, "downloadwella.com") {
		return urlStr
	}
	html, err := getHTML(urlStr)
	if err != nil {
		return urlStr
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return urlStr
	}

	form := doc.Find("form").First()
	if form.Length() == 0 {
		return urlStr
	}

	payload := url.Values{}
	form.Find("input").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		val, _ := s.Attr("value")
		if name != "" {
			payload.Set(name, val)
		}
	})

	html2, err := postForm(urlStr, payload)
	if err != nil {
		return urlStr
	}

	doc2, err := goquery.NewDocumentFromReader(strings.NewReader(html2))
	if err != nil {
		return urlStr
	}

	var directLink string
	doc2.Find("a").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.HasSuffix(href, ".mkv") || strings.HasSuffix(href, ".mp4") || strings.Contains(href, ".mkv") {
			directLink = href
		}
	})

	if directLink != "" {
		return directLink
	}
	return urlStr
}

func CleanFilename(filename string) string {
	extIdx := strings.LastIndex(filename, ".")
	var base, ext string
	if extIdx != -1 {
		base = filename[:extIdx]
		ext = filename[extIdx:]
	} else {
		base = filename
		ext = ".mp4"
	}

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

	cleaned := base
	for _, re := range reList {
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	reSpace := regexp.MustCompile(`[\._-]`)
	cleaned = reSpace.ReplaceAllString(cleaned, " ")

	reMultiSpace := regexp.MustCompile(`\s+`)
	cleaned = strings.TrimSpace(reMultiSpace.ReplaceAllString(cleaned, " "))

	return fmt.Sprintf("%s [@KDramazFlix]%s", cleaned, ext)
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

func ExtractEpisodeNumber(filename string) int {
	re := regexp.MustCompile(`(?i)[Ee](\d+)|episode[s]?[-_\s]?(\d+)|ep[-_\s]?(\d+)`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) > 1 {
		for _, m := range matches[1:] {
			if m != "" {
				if val, err := strconv.Atoi(m); err == nil {
					return val
				}
			}
		}
	}
	return 1
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

func GetShowPageAndSource(targetURL string) (string, string) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", ""
	}
	host := strings.ToLower(parsed.Host)
	cleanURL := strings.Split(targetURL, "#")[0]

	if strings.Contains(host, "dramakey.com") {
		return cleanURL, "DramaKey"
	} else if strings.Contains(host, "kissasia") {
		return cleanURL, "KissAsia"
	} else if strings.Contains(host, "dramacool") {
		if strings.Contains(parsed.Path, "/drama-detail/") {
			return cleanURL, "DramaCool"
		}
		return cleanURL, "DramaCool"
	}
	return "", ""
}
