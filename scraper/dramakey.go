/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package scraper

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ScrapeDramaKey(doc *goquery.Document, showURL string, targetEpNum int) (*ShowData, error) {
	title := strings.TrimSpace(doc.Find("h1, .entry-title").First().Text())
	if title == "" {
		title = "Unknown Drama"
	}

	imgURL, _ := doc.Find(`meta[property="og:image"]`).Attr("content")
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL, _ = doc.Find("img.wp-post-image, .post-thumbnail img, .entry-content img").First().Attr("src")
	}
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL, _ = doc.Find("img.wp-post-image, .post-thumbnail img, .entry-content img").First().Attr("data-src")
	}
	if imgURL != "" && !strings.HasPrefix(imgURL, "http") {
		base, _ := url.Parse(showURL)
		rel, _ := url.Parse(imgURL)
		imgURL = base.ResolveReference(rel).String()
	}

	epLinks := make(map[string][]struct{ text, href string })
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
			epKey := fmt.Sprintf("Episode %02d", epNum)

			epLinks[epKey] = append(epLinks[epKey], struct{ text, href string }{text, href})
		}
	})

	var episodes []Episode
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

	return &ShowData{
		Title:    title,
		ImgURL:   imgURL,
		Episodes: episodes,
	}, nil
}
