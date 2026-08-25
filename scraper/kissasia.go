/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package scraper

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ScrapeKissAsia(doc *goquery.Document, showURL string, targetEpNum int) (*ShowData, error) {
	title := strings.TrimSpace(doc.Find("h1, .wp-block-post-title, .entry-title").First().Text())
	if title == "" {
		title = "Unknown Drama"
	}

	imgURL, _ := doc.Find(`meta[property="og:image"]`).Attr("content")
	if imgURL != "" {
		imgLower := strings.ToLower(imgURL)
		if strings.Contains(imgLower, "logo") || strings.Contains(imgLower, "kissasia.png") {
			imgURL = ""
		}
	}
	if imgURL == "" {
		doc.Find("img.wp-post-image, .post-thumbnail img, .entry-content img").Each(func(i int, s *goquery.Selection) {
			if imgURL != "" {
				return
			}
			src, _ := s.Attr("src")
			if src == "" {
				src, _ = s.Attr("data-src")
			}
			srcLower := strings.ToLower(src)
			if src != "" && !strings.Contains(srcLower, "logo") && !strings.Contains(srcLower, "kissasia.png") {
				imgURL = src
			}
		})
	}
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") || strings.Contains(strings.ToLower(imgURL), "kissasia.png") {
		cleanShowURL := strings.Split(showURL, "?")[0]
		cleanHTML, errClean := getHTML(cleanShowURL)
		if errClean == nil {
			cleanDoc, errDoc := goquery.NewDocumentFromReader(strings.NewReader(cleanHTML))
			if errDoc == nil {
				ogClean, _ := cleanDoc.Find(`meta[property="og:image"]`).Attr("content")
				ogLower := strings.ToLower(ogClean)
				if ogClean != "" && !strings.Contains(ogLower, "logo") && !strings.Contains(ogLower, "kissasia.png") {
					imgURL = ogClean
				}
			}
		}
	}
	if imgURL != "" && !strings.HasPrefix(imgURL, "http") {
		base, _ := url.Parse(showURL)
		rel, _ := url.Parse(imgURL)
		imgURL = base.ResolveReference(rel).String()
	}

	postID, _ := doc.Find("#Play, [data-post-id]").First().Attr("data-post-id")
	var matchedEntry map[string]interface{}

	if postID != "" {
		feedURL := fmt.Sprintf("https://www.blogger.com/feeds/4927638411765974267/posts/default/%s?alt=json", postID)
		respHTML, err := getHTML(feedURL)
		if err == nil {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(respHTML), &data); err == nil {
				matchedEntry, _ = data["entry"].(map[string]interface{})
			}
		}
	}

	if matchedEntry == nil {
		query := url.QueryEscape(title)
		feedURL := fmt.Sprintf("https://www.blogger.com/feeds/4927638411765974267/posts/default?alt=json&q=%s", query)
		respHTML, err := getHTML(feedURL)
		if err == nil {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(respHTML), &data); err == nil {
				feed, _ := data["feed"].(map[string]interface{})
				entries, _ := feed["entry"].([]interface{})
				cleanedShow := strings.ToLower(CleanShowTitle(title))

				for _, item := range entries {
					entry, _ := item.(map[string]interface{})
					bTitleObj, _ := entry["title"].(map[string]interface{})
					bTitle, _ := bTitleObj["$t"].(string)

					bShow := bTitle
					if strings.Contains(bTitle, "-") {
						parts := strings.SplitN(bTitle, "-", 2)
						bShow = parts[1]
					}
					cleanedB := strings.ToLower(CleanShowTitle(bShow))
					if len(cleanedShow) > 2 && (strings.Contains(cleanedB, cleanedShow) || strings.Contains(cleanedShow, cleanedB)) {
						matchedEntry = entry
						break
					}
				}
			}
		}
	}

	var episodes []Episode
	if matchedEntry != nil {
		contentObj, _ := matchedEntry["content"].(map[string]interface{})
		content, _ := contentObj["$t"].(string)

		lines := strings.Split(content, ";")
		for idx, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if strings.Contains(strings.ToLower(line), "<img") {
				reImg := regexp.MustCompile(`src="([^"]+)"`)
				imgMatch := reImg.FindStringSubmatch(line)
				if len(imgMatch) > 1 && (imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") || strings.Contains(strings.ToLower(imgURL), "kissasia.png")) {
					foundImg := imgMatch[1]
					if !strings.Contains(strings.ToLower(foundImg), "logo") && !strings.Contains(strings.ToLower(foundImg), "kissasia.png") {
						imgURL = foundImg
					}
				}
				continue
			}

			reURL := regexp.MustCompile(`(https?://[^\s|]+)`)
			urlMatch := reURL.FindStringSubmatch(line)
			if len(urlMatch) > 1 {
				videoURL := urlMatch[1]
				subtitles := ""
				subtitleURL := ""
				var tracks []SubtitleTrack

				subParts := strings.Split(line, "|")
				if len(subParts) >= 2 {
					subtitles = strings.TrimSpace(subParts[1])
				}
				if len(subParts) >= 3 {
					subURLs := strings.Split(subParts[2], ",")
					subLangs := strings.Split(subtitles, ",")
					for i, rawURL := range subURLs {
						cleanU := strings.TrimSpace(rawURL)
						if cleanU != "" {
							lang := "English"
							if i < len(subLangs) && strings.TrimSpace(subLangs[i]) != "" {
								lang = strings.TrimSpace(subLangs[i])
							}
							tracks = append(tracks, SubtitleTrack{
								Language: lang,
								URL:      cleanU,
							})
						}
					}
					if len(subURLs) > 0 && strings.TrimSpace(subURLs[0]) != "" {
						subtitleURL = strings.TrimSpace(subURLs[0])
					}
				}

				epNum := idx + 1
				reEp := regexp.MustCompile(`(?i)[Ee](\d+)|episode[s]?[-_\s]?(\d+)|ep[-_\s]?(\d+)`)
				epMatch := reEp.FindStringSubmatch(line)
				if len(epMatch) > 1 {
					for _, m := range epMatch[1:] {
						if m != "" {
							if val, err := strconv.Atoi(m); err == nil {
								epNum = val
								break
							}
						}
					}
				}

				if subtitles == "" && (strings.Contains(strings.ToLower(line), "sub") || strings.Contains(strings.ToLower(line), "eng")) {
					subtitles = "English"
				}

				if strings.Contains(strings.ToLower(subtitles), "eng") {
					subtitles = "English"
				}

				if len(tracks) == 0 && subtitleURL != "" {
					tracks = append(tracks, SubtitleTrack{Language: "English", URL: subtitleURL})
				}

				episodes = append(episodes, Episode{
					Episode:        epNum,
					Title:          fmt.Sprintf("%s - Episode %02d", title, epNum),
					Link:           fmt.Sprintf("%s?episode=%d", showURL, epNum),
					Qualities:      map[string]string{"720p": videoURL},
					Subtitles:      subtitles,
					SubtitleURL:    subtitleURL,
					SubtitleTracks: tracks,
				})
			}
		}
	}

	if targetEpNum > 0 && len(episodes) > 0 {
		var filtered []Episode
		for _, ep := range episodes {
			if ep.Episode == targetEpNum {
				filtered = append(filtered, ep)
			}
		}
		if len(filtered) > 0 {
			episodes = filtered
		}
	}

	if len(episodes) == 0 {
		var rawLinks []struct{ text, href string }
		doc.Find(".entry-content a[href], #Play a[href]").Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			text := strings.TrimSpace(s.Text())
			if strings.Contains(href, "gofile") || strings.Contains(href, "mega.nz") || strings.Contains(href, "drive.google") || strings.Contains(href, "download") {
				if !strings.HasPrefix(href, "http") {
					base, _ := url.Parse(showURL)
					rel, _ := url.Parse(href)
					href = base.ResolveReference(rel).String()
				}
				rawLinks = append(rawLinks, struct{ text, href string }{text, href})
			}
		})

		epLinks := make(map[string][]struct{ text, href string })
		for _, link := range rawLinks {
			parts := strings.Split(link.href, "/")
			filename := parts[len(parts)-1]
			epNum := ExtractEpisodeNumber(filename)
			epKey := fmt.Sprintf("Episode %02d", epNum)
			epLinks[epKey] = append(epLinks[epKey], link)
		}

		for epKey, links := range epLinks {
			var epNum int
			_, _ = fmt.Sscanf(epKey, "Episode %d", &epNum)
			qualities := detectQualities(links)

			subtitles := ""
			for _, link := range links {
				combined := strings.ToLower(link.text + " " + link.href)
				if strings.Contains(combined, "sub") || strings.Contains(combined, "eng") {
					subtitles = "English"
					break
				}
			}

			episodes = append(episodes, Episode{
				Episode:   epNum,
				Title:     fmt.Sprintf("%s - %s", title, epKey),
				Link:      fmt.Sprintf("%s#%s", showURL, strings.ReplaceAll(epKey, " ", "_")),
				Qualities: qualities,
				Subtitles: subtitles,
			})
		}
	}

	status := "Ongoing"
	docText := strings.ToLower(doc.Text())
	if strings.Contains(docText, "status: completed") || strings.Contains(docText, "status: complete") || strings.Contains(strings.ToLower(title), "complete") {
		status = "Completed"
	}

	return &ShowData{
		Title:    title,
		Status:   status,
		ImgURL:   imgURL,
		Episodes: episodes,
	}, nil
}
