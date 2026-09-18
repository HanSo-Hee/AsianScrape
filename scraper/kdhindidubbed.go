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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func resolveXCloudStream(rawURL string) string {
	reID := regexp.MustCompile(`/([a-f0-9]{12,36})/?$`)
	matches := reID.FindStringSubmatch(rawURL)
	if len(matches) < 2 {
		return rawURL
	}
	vidID := matches[1]
	embedURL := fmt.Sprintf("https://stream.xcloud1.lol/%s", vidID)

	req, err := http.NewRequest("GET", embedURL, nil)
	if err != nil {
		return rawURL
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://xcloud.tattoo/")

	clientWithTimeout := &http.Client{Timeout: 10 * time.Second}
	resp, err := clientWithTimeout.Do(req)
	if err != nil {
		return rawURL
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return rawURL
	}
	body := string(bodyBytes)

	reDirect := regexp.MustCompile(`file:\s*["'](/apis/cloud[0-9]*/redirect/[^"']+)["']`)
	if m := reDirect.FindStringSubmatch(body); len(m) > 1 {
		return "https://stream.xcloud1.lol" + m[1]
	}

	reFull := regexp.MustCompile(`file:\s*["'](https?://[^"']+)["']`)
	if m := reFull.FindStringSubmatch(body); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	return rawURL
}

func parseKDHindiEpisodesFromDoc(doc *goquery.Document, quality string) map[int]string {
	epVideos := make(map[int]string)

	reEp := regexp.MustCompile(`(?i)Episode\s*[-–—_.]?\s*(\d{1,4})`)

	type epSection struct {
		epNum int
		links []struct{ text, href string }
	}

	var sections []epSection
	var currentSec *epSection

	content := doc.Find(".entry-content, .post-content, article").First()
	if content.Length() == 0 {
		content = doc.Selection
	}

	content.Find("p, div, h2, h3, h4, b, a").Each(func(i int, s *goquery.Selection) {
		if s.Is("a") {
			if currentSec != nil {
				href := strings.TrimSpace(s.AttrOr("href", ""))
				text := strings.TrimSpace(s.Text())
				if href != "" && !strings.HasPrefix(href, "#") && !strings.Contains(href, "telegram.me") && !strings.Contains(href, "t.me/s/") && !strings.Contains(href, "instagram.com") {
					currentSec.links = append(currentSec.links, struct{ text, href string }{text, href})
				}
			}
			return
		}

		txt := strings.TrimSpace(s.Text())
		if m := reEp.FindStringSubmatch(txt); len(m) > 1 {
			if val, err := strconv.Atoi(m[1]); err == nil && val > 0 {
				if currentSec == nil || currentSec.epNum != val {
					sections = append(sections, epSection{epNum: val})
					currentSec = &sections[len(sections)-1]
				}
			}
		}
	})

	for _, sec := range sections {
		if len(sec.links) == 0 {
			continue
		}

		var xcloudLink, streamLink, momoLink, telegramLink, otherLink string
		for _, l := range sec.links {
			hLower := strings.ToLower(l.href)
			tLower := strings.ToLower(l.text)

			if strings.Contains(hLower, "xcloud") || strings.Contains(tLower, "xcloud") {
				if xcloudLink == "" {
					xcloudLink = l.href
				}
			} else if strings.Contains(hLower, "hgcloud") || strings.Contains(hLower, "chuckle-tube") || strings.Contains(tLower, "streamlink") {
				if streamLink == "" {
					streamLink = l.href
				}
			} else if strings.Contains(hLower, "momofile") || strings.Contains(tLower, "momofile") {
				if momoLink == "" {
					momoLink = l.href
				}
			} else if strings.Contains(hLower, "filebee") || strings.Contains(hLower, "filepress") || strings.Contains(tLower, "telegram") {
				if telegramLink == "" {
					telegramLink = l.href
				}
			} else if otherLink == "" && strings.HasPrefix(hLower, "http") {
				otherLink = l.href
			}
		}

		// Pick ONE best video URL per episode to prevent duplicate uploads
		chosenURL := ""
		if xcloudLink != "" {
			resolved := resolveXCloudStream(xcloudLink)
			if resolved != "" {
				chosenURL = resolved
			} else {
				chosenURL = xcloudLink
			}
		} else if streamLink != "" {
			chosenURL = streamLink
		} else if momoLink != "" {
			chosenURL = momoLink
		} else if telegramLink != "" {
			chosenURL = telegramLink
		} else if otherLink != "" {
			chosenURL = otherLink
		}

		if chosenURL != "" {
			epVideos[sec.epNum] = chosenURL
		}
	}

	return epVideos
}

func ScrapeKDHindiDubbed(doc *goquery.Document, showURL string, targetEpNum int) (*ShowData, error) {
	title := ExtractShowTitle(doc, "")
	if title == "" || strings.EqualFold(title, "unknown drama") {
		uParts := strings.Split(strings.Trim(showURL, "/"), "/")
		if len(uParts) > 0 {
			title = CleanShowTitle(strings.ReplaceAll(uParts[len(uParts)-1], "-", " "))
		}
	}
	if title == "" {
		title = "Unknown Drama"
	}

	imgURL, _ := doc.Find(`meta[property="og:image"]`).Attr("content")
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL = doc.Find(`meta[name="twitter:image"]`).AttrOr("content", "")
	}
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		doc.Find(".entry-content img, article img, .post-thumbnail img").Each(func(i int, s *goquery.Selection) {
			if imgURL != "" {
				return
			}
			src := s.AttrOr("src", s.AttrOr("data-src", ""))
			if src != "" && !strings.Contains(strings.ToLower(src), "logo") && !strings.Contains(strings.ToLower(src), "icon") {
				imgURL = src
			}
		})
	}
	if imgURL != "" && !strings.HasPrefix(imgURL, "http") {
		base, _ := url.Parse(showURL)
		rel, _ := url.Parse(imgURL)
		imgURL = base.ResolveReference(rel).String()
	}

	qualityPages := make(map[string]string)
	rawHTML, _ := doc.Html()
	reQualityBlock := regexp.MustCompile(`(?i)(1080p|720p|480p)[\s\S]{1,500}?href=["'](https?://kdhindidubbed\.cfd/[^"'#\s]+)["'][^>]*>\s*DOWNLOAD LINKS`)
	matches := reQualityBlock.FindAllStringSubmatch(rawHTML, -1)
	for _, m := range matches {
		if len(m) > 2 {
			q := strings.ToLower(m[1])
			u := strings.TrimSpace(m[2])
			if !strings.HasSuffix(q, "p") {
				q += "p"
			}
			qualityPages[q] = u
		}
	}

	if len(qualityPages) == 0 {
		doc.Find(".entry-content a[href], article a[href]").Each(func(i int, s *goquery.Selection) {
			href := strings.TrimSpace(s.AttrOr("href", ""))
			if !strings.Contains(href, "kdhindidubbed.cfd") || strings.Contains(href, "#") || strings.Contains(href, "wp-") {
				return
			}
			linkText := strings.ToLower(s.Text())
			if strings.Contains(linkText, "1080") {
				qualityPages["1080p"] = href
			} else if strings.Contains(linkText, "720") {
				qualityPages["720p"] = href
			} else if strings.Contains(linkText, "480") {
				qualityPages["480p"] = href
			}
		})
	}

	allEpQualities := make(map[int]map[string]string)

	if len(qualityPages) > 0 {
		for q, qURL := range qualityPages {
			var qDoc *goquery.Document
			if qURL == showURL {
				qDoc = doc
			} else {
				html, err := getHTML(qURL)
				if err != nil {
					continue
				}
				qDoc, err = goquery.NewDocumentFromReader(strings.NewReader(html))
				if err != nil {
					continue
				}
			}

			epMap := parseKDHindiEpisodesFromDoc(qDoc, q)
			for epNum, videoURL := range epMap {
				if _, exists := allEpQualities[epNum]; !exists {
					allEpQualities[epNum] = make(map[string]string)
				}
				allEpQualities[epNum][q] = videoURL
			}
		}
	} else {
		// showURL is already a quality page directly
		defaultQ := "720p"
		pageText := strings.ToLower(doc.Text() + " " + showURL)
		if strings.Contains(pageText, "1080") {
			defaultQ = "1080p"
		} else if strings.Contains(pageText, "480") {
			defaultQ = "480p"
		}

		epMap := parseKDHindiEpisodesFromDoc(doc, defaultQ)
		for epNum, videoURL := range epMap {
			if _, exists := allEpQualities[epNum]; !exists {
				allEpQualities[epNum] = make(map[string]string)
			}
			allEpQualities[epNum][defaultQ] = videoURL
		}
	}

	var epNumbers []int
	for epNum := range allEpQualities {
		epNumbers = append(epNumbers, epNum)
	}
	sort.Ints(epNumbers)

	var episodes []Episode
	for _, epNum := range epNumbers {
		qs := allEpQualities[epNum]
		epKey := fmt.Sprintf("Episode %02d", epNum)
		episodes = append(episodes, Episode{
			Episode:   epNum,
			Title:     fmt.Sprintf("%s - %s", title, epKey),
			Link:      fmt.Sprintf("%s#%s", showURL, strings.ReplaceAll(epKey, " ", "_")),
			Qualities: qs,
			Subtitles: "English",
		})
	}

	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].Episode < episodes[j].Episode
	})

	totalEpisodes := len(episodes)

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

	status := DetectShowStatus(doc, title)

	audio := ""
	lowerFull := strings.ToLower(title + " " + showURL + " " + doc.Text())
	if strings.Contains(lowerFull, "english dubbed") || strings.Contains(lowerFull, "eng.kor.dub") || strings.Contains(lowerFull, "english-dubbed") {
		audio = "English"
	} else if strings.Contains(lowerFull, "hindi dubbed") || strings.Contains(lowerFull, "hindi-dubbed") {
		audio = "Hindi"
	} else {
		audio = DetectAudioLanguage(doc, title, showURL)
	}

	return &ShowData{
		Title:         title,
		Status:        status,
		Audio:         audio,
		TotalEpisodes: totalEpisodes,
		ImgURL:        imgURL,
		Episodes:      episodes,
	}, nil
}
