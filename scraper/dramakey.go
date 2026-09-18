/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package scraper

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ScrapeDramaKey(doc *goquery.Document, showURL string, targetEpNum int) (*ShowData, error) {
	title := ExtractShowTitle(doc, "")
	if title == "" || strings.EqualFold(title, "unknown drama") {
		// Fallback to URL slug
		uParts := strings.Split(strings.Trim(showURL, "/"), "/")
		if len(uParts) > 0 {
			slug := uParts[len(uParts)-1]
			title = CleanShowTitle(strings.ReplaceAll(slug, "-", " "))
		}
	}
	if title == "" {
		title = "Unknown Drama"
	}

	imgURL, _ := doc.Find(`meta[property="og:image"]`).Attr("content")
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL, _ = doc.Find(`meta[name="twitter:image"]`).Attr("content")
	}
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL, _ = doc.Find("img.wp-post-image, .post-thumbnail img, .entry-content img, .elementor-image img").First().Attr("src")
	}
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL, _ = doc.Find("img.wp-post-image, .post-thumbnail img, .entry-content img, .elementor-image img").First().Attr("data-src")
	}
	if imgURL != "" && !strings.HasPrefix(imgURL, "http") {
		base, _ := url.Parse(showURL)
		rel, _ := url.Parse(imgURL)
		imgURL = base.ResolveReference(rel).String()
	}

	epMap := make(map[int][]struct{ text, href string })
	var epNumbers []int

	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())
		if strings.Contains(href, "downloadwella") || strings.Contains(href, "gofile") || strings.Contains(href, "mega.nz") || strings.Contains(href, "drive.google") {
			if !strings.HasPrefix(href, "http") {
				base, _ := url.Parse(showURL)
				rel, _ := url.Parse(href)
				href = base.ResolveReference(rel).String()
			}

			parts := strings.Split(href, "/")
			filename := parts[len(parts)-1]

			epNum := ExtractEpisodeNumber(filename)
			if epNum == 0 {
				epNum = ExtractEpisodeNumber(href)
			}
			if epNum == 0 {
				epNum = ExtractEpisodeNumber(text)
			}
			if epNum == 0 {
				// Check closest preceding heading
				prevH := s.ParentsFiltered(".elementor-widget-container, .elementor-element").Prev().Find(".elementor-heading-title, h2, h3, h4").Text()
				epNum = ExtractEpisodeNumber(prevH)
			}

			if epNum > 0 {
				if _, exists := epMap[epNum]; !exists {
					epNumbers = append(epNumbers, epNum)
				}
				epMap[epNum] = append(epMap[epNum], struct{ text, href string }{text, href})
			}
		}
	})

	sort.Ints(epNumbers)

	var episodes []Episode
	for _, epNum := range epNumbers {
		links := epMap[epNum]
		qualities := detectQualities(links)
		subtitles := ""
		for _, link := range links {
			combined := strings.ToLower(link.text + " " + link.href)
			if strings.Contains(combined, "sub") || strings.Contains(combined, "eng") {
				subtitles = "English"
				break
			}
		}

		epKey := fmt.Sprintf("Episode %02d", epNum)
		episodes = append(episodes, Episode{
			Episode:   epNum,
			Title:     fmt.Sprintf("%s - %s", title, epKey),
			Link:      fmt.Sprintf("%s#%s", showURL, strings.ReplaceAll(epKey, " ", "_")),
			Qualities: qualities,
			Subtitles: subtitles,
		})
	}

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
	audio := DetectAudioLanguage(doc, title, showURL)

	return &ShowData{
		Title:         title,
		Status:        status,
		Audio:         audio,
		TotalEpisodes: totalEpisodes,
		ImgURL:        imgURL,
		Episodes:      episodes,
	}, nil
}
