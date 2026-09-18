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
	} else if strings.Contains(lower, "kdhindidubbed") {
		source = "kdhindidubbed"
	}

	cleanURL := inputURL
	if idx := strings.Index(cleanURL, "#"); idx != -1 {
		cleanURL = cleanURL[:idx]
	}
	return cleanURL, source
}

func CleanShowTitle(title string) string {
	cleaned := title

	rePrefixes := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*DOWNLOAD\s+Links?\s+for\s+`),
		regexp.MustCompile(`(?i)^\s*DOWNLOAD\s+`),
		regexp.MustCompile(`(?i)^\s*Watch\s+(?:Online\s+Free\s+with\s+English\s+Subs\s+-\s+)?`),
	}
	for _, re := range rePrefixes {
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	reSuffixes := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\|\s*Chinese Drama.*$`),
		regexp.MustCompile(`(?i)\|\s*Korean Drama.*$`),
		regexp.MustCompile(`(?i)\|\s*Japanese Drama.*$`),
		regexp.MustCompile(`(?i)\|\s*DramaCool.*$`),
		regexp.MustCompile(`(?i)\|\s*DramaKey.*$`),
		regexp.MustCompile(`(?i)[-–—]\s*KDHindiDubbed.*$`),
		regexp.MustCompile(`(?i)-\s*KissAsia.*$`),
		regexp.MustCompile(`(?i)-\s*DramaCool.*$`),
		regexp.MustCompile(`(?i)-\s*DramaKey.*$`),
		regexp.MustCompile(`(?i)Watch Online.*$`),
		regexp.MustCompile(`(?i)all episodes with English subtitles.*$`),
	}
	for _, re := range reSuffixes {
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	reList := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\(Episodes?\s*[\d\s&,-]+\s*Added\)`),
		regexp.MustCompile(`(?i)\(Episodes?\s*[\d\s&,-]+\s*Complete\)`),
		regexp.MustCompile(`(?i)\(Complete\)`),
		regexp.MustCompile(`(?i)Complete Chinese Drama`),
		regexp.MustCompile(`(?i)Complete Korean Drama`),
		regexp.MustCompile(`(?i)Korean Drama English Dubbed Episodes`),
		regexp.MustCompile(`(?i)English Dubbed Episodes`),
		regexp.MustCompile(`(?i)\(?Chinese Drama\)?`),
		regexp.MustCompile(`(?i)\(?Korean Drama\)?`),
		regexp.MustCompile(`(?i)\(?Japanese Drama\)?`),
		regexp.MustCompile(`(?i)\bS\d{1,2}\b`),
		regexp.MustCompile(`(?i)\(?dramakey\.com\)?`),
		regexp.MustCompile(`(?i)\(?kissasia\.biz\)?`),
		regexp.MustCompile(`(?i)\(?kdhindidubbed\.[a-z]+\)?`),
		regexp.MustCompile(`(?i)\(?dramacool\.[a-z]+\)?`),
		regexp.MustCompile(`(?i)\(?moviesflixers_dl\)?`),
		regexp.MustCompile(`\[.*?\]`),
	}

	for _, re := range reList {
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	reSpace := regexp.MustCompile(`[\._-]`)
	cleaned = reSpace.ReplaceAllString(cleaned, " ")

	reMultiSpace := regexp.MustCompile(`\s+`)
	cleaned = strings.TrimSpace(reMultiSpace.ReplaceAllString(cleaned, " "))

	return cleaned
}

func ExtractShowTitle(doc *goquery.Document, defaultTitle string) string {
	candidates := []string{}

	if doc != nil {
		if ogTitle, exists := doc.Find(`meta[property="og:title"]`).Attr("content"); exists && strings.TrimSpace(ogTitle) != "" {
			candidates = append(candidates, strings.TrimSpace(ogTitle))
		}
		if twTitle, exists := doc.Find(`meta[name="twitter:title"]`).Attr("content"); exists && strings.TrimSpace(twTitle) != "" {
			candidates = append(candidates, strings.TrimSpace(twTitle))
		}
		doc.Find("h1, .entry-title, .title, h1.title").Each(func(i int, s *goquery.Selection) {
			t := strings.TrimSpace(s.Text())
			if t != "" {
				candidates = append(candidates, t)
			}
		})
		if docTitle := strings.TrimSpace(doc.Find("title").Text()); docTitle != "" {
			candidates = append(candidates, docTitle)
		}
		doc.Find(".elementor-heading-title, .post-title, .name").Each(func(i int, s *goquery.Selection) {
			t := strings.TrimSpace(s.Text())
			if t != "" && !strings.EqualFold(t, "synopsis") && !strings.EqualFold(t, "trailer") && !strings.HasPrefix(strings.ToLower(t), "status") && !strings.HasPrefix(strings.ToLower(t), "season") && !strings.HasPrefix(strings.ToLower(t), "episode") {
				candidates = append(candidates, t)
			}
		})
	}

	for _, raw := range candidates {
		cleaned := CleanShowTitle(raw)
		if cleaned != "" && !strings.EqualFold(cleaned, "unknown drama") && len(cleaned) > 2 {
			return cleaned
		}
	}

	if defaultTitle != "" {
		cleaned := CleanShowTitle(defaultTitle)
		if cleaned != "" {
			return cleaned
		}
	}

	return "Unknown Drama"
}

func ExtractEpisodeNumber(text string) int {
	if text == "" {
		return 0
	}
	text = strings.Split(text, "?")[0]
	text = strings.Split(text, "#")[0]

	// 1. Season & Episode: S01E01, s1e1, s01.e01, s01_e01, etc.
	reSeasonEp := regexp.MustCompile(`(?i)\bs\d{1,2}\s*[-_.]?\s*e(\d{1,4})\b`)
	if m := reSeasonEp.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			return val
		}
	}

	// 2. Explicit Episode / Ep: "Episode 1", "Episode-01", "Ep.01", "ep 5"
	reExplicit := regexp.MustCompile(`(?i)\b(?:episodes?|ep)\s*[-_.]?\s*(\d{1,4})\b`)
	if m := reExplicit.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			return val
		}
	}

	// 3. E followed by number with boundaries or separators: E01, .E12., /E05/
	reE := regexp.MustCompile(`(?i)(?:^|[\s._\-\[/])e(\d{1,4})(?:[\s._\-\]\)/]|$)`)
	if m := reE.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			return val
		}
	}

	// 4. Trailing number before file extension, e.g. "Drama - 08.mkv", "drama_01.mp4"
	reTrailing := regexp.MustCompile(`(?i)(?:^|[\s._\-])(\d{1,3})\s*\.(?:mkv|mp4|avi|webm|html|vtt|srt)`)
	if m := reTrailing.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			if val != 360 && val != 480 && val != 540 && val != 720 {
				return val
			}
		}
	}

	// 5. Bracketed numbers: [01], (05)
	reBracketed := regexp.MustCompile(`(?:\[|\()(\d{1,3})(?:\]|\))`)
	if m := reBracketed.FindStringSubmatch(text); len(m) > 1 {
		if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
			return val
		}
	}

	return 0
}

func DetectShowStatus(doc *goquery.Document, title string) string {
	if doc != nil {
		if doc.Find("a[href*='category/completed'], a[href*='tag/completed'], a[href*='status=completed']").Length() > 0 {
			return "Completed"
		}

		relStatus := strings.ToLower(doc.Find(".release-status, .status").Text())
		if strings.Contains(relStatus, "complete") {
			return "Completed"
		}

		var headingStatus string
		doc.Find("h2, h3, h4, .elementor-heading-title, p, span").Each(func(i int, s *goquery.Selection) {
			if headingStatus != "" {
				return
			}
			txt := strings.ToLower(s.Text())
			if strings.Contains(txt, "status:") || strings.Contains(txt, "status -") {
				if strings.Contains(txt, "complete") {
					headingStatus = "Completed"
				} else if strings.Contains(txt, "ongoing") {
					headingStatus = "Ongoing"
				}
			}
		})
		if headingStatus != "" {
			return headingStatus
		}

		docText := strings.ToLower(doc.Text())
		reCompleted := regexp.MustCompile(`(?i)\bstatus\s*[:\s-]\s*completed?\b|\bcompleted?\s+drama\b`)
		if reCompleted.MatchString(docText) {
			return "Completed"
		}
	}

	lowerTitle := strings.ToLower(title)
	if strings.Contains(lowerTitle, "complete") {
		return "Completed"
	}

	return "Ongoing"
}

func DetectAudioLanguage(doc *goquery.Document, title, pageURL string) string {
	var primaryText string
	if doc != nil {
		ogTitle, _ := doc.Find(`meta[property="og:title"]`).Attr("content")
		primaryText = strings.ToLower(title + " " + pageURL + " " + ogTitle + " " + doc.Find("title").Text())
	} else {
		primaryText = strings.ToLower(title + " " + pageURL)
	}

	if strings.Contains(primaryText, "chinese") || strings.Contains(primaryText, "c-drama") || strings.Contains(primaryText, "china") || strings.Contains(primaryText, "mandarin") {
		return "Chinese"
	}
	if strings.Contains(primaryText, "south-korean") || strings.Contains(primaryText, "south korean") || strings.Contains(primaryText, "korean") || strings.Contains(primaryText, "k-drama") {
		return "Korean"
	}
	if strings.Contains(primaryText, "japanese") || strings.Contains(primaryText, "j-drama") || strings.Contains(primaryText, "japan") {
		return "Japanese"
	}
	if strings.Contains(primaryText, "thailand") || strings.Contains(primaryText, "thai") || strings.Contains(primaryText, "t-drama") {
		return "Thai"
	}

	if doc != nil {
		var metaText string
		doc.Find("a[rel='tag'], a[href*='/category/'], a[href*='/tag/'], span.wp-block-post-terms a, .entry-meta a, .country, .genre").Each(func(i int, s *goquery.Selection) {
			metaText += " " + strings.ToLower(s.Text()) + " " + strings.ToLower(s.AttrOr("href", ""))
		})

		if strings.Contains(metaText, "south-korean") || strings.Contains(metaText, "south korean") || strings.Contains(metaText, "/korean") || strings.Contains(metaText, "k-drama") {
			return "Korean"
		}
		if strings.Contains(metaText, "chinese") || strings.Contains(metaText, "c-drama") || strings.Contains(metaText, "/china") {
			return "Chinese"
		}
		if strings.Contains(metaText, "japanese") || strings.Contains(metaText, "j-drama") || strings.Contains(metaText, "/japan") {
			return "Japanese"
		}
		if strings.Contains(metaText, "thailand") || strings.Contains(metaText, "/thai") {
			return "Thai"
		}
	}

	return "Korean"
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
