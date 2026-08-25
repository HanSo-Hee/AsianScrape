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

func ScrapeDramaCool(doc *goquery.Document, showURL string, targetEpNum int) (*ShowData, error) {
	title := strings.TrimSpace(doc.Find("h1, .title, h1.title").First().Text())
	if title == "" {
		title = "Unknown Drama"
	}

	imgURL, _ := doc.Find(`meta[property="og:image"]`).Attr("content")
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL, _ = doc.Find(".img.thumb img, .thumb img, img.wp-post-image").First().Attr("src")
	}
	if imgURL == "" || strings.Contains(strings.ToLower(imgURL), "logo") {
		imgURL, _ = doc.Find(".img.thumb img, .thumb img, img.wp-post-image").First().Attr("data-src")
	}
	if imgURL != "" && !strings.HasPrefix(imgURL, "http") {
		base, _ := url.Parse(showURL)
		rel, _ := url.Parse(imgURL)
		imgURL = base.ResolveReference(rel).String()
	}

	var episodePages []string
	listContainer := doc.Find("ul.list-episode-item, ul.all-episode").First()
	if listContainer.Length() == 0 {
		listContainer = doc.Selection
	}

	listContainer.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.Contains(href, "-episode-") {
			if !strings.HasPrefix(href, "http") {
				base, _ := url.Parse(showURL)
				rel, _ := url.Parse(href)
				href = base.ResolveReference(rel).String()
			}
			exists := false
			for _, page := range episodePages {
				if page == href {
					exists = true
					break
				}
			}
			if !exists {
				episodePages = append(episodePages, href)
			}
		}
	})

	if len(episodePages) == 0 {
		episodePages = append(episodePages, showURL)
	}

	var episodes []Episode
	for _, epURL := range episodePages {
		parts := strings.Split(epURL, "/")
		epNum := ExtractEpisodeNumber(parts[len(parts)-1])

		epHTML, err := getHTML(epURL)
		if err != nil {
			continue
		}

		epDoc, err := goquery.NewDocumentFromReader(strings.NewReader(epHTML))
		if err != nil {
			continue
		}

		var rawLinks []struct{ text, href string }
		epDoc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			text := strings.TrimSpace(s.Text())
			if strings.Contains(href, "gofile") || strings.Contains(href, "mega") || strings.Contains(href, "download") {
				if !strings.HasPrefix(href, "http") {
					base, _ := url.Parse(epURL)
					rel, _ := url.Parse(href)
					href = base.ResolveReference(rel).String()
				}
				rawLinks = append(rawLinks, struct{ text, href string }{text, href})
			}
		})

		if len(rawLinks) > 0 {
			qualities := detectQualities(rawLinks)
			subtitles := ""
			for _, link := range rawLinks {
				combined := strings.ToLower(link.text + " " + link.href)
				if strings.Contains(combined, "sub") || strings.Contains(combined, "eng") {
					subtitles = "English"
					break
				}
			}

			episodes = append(episodes, Episode{
				Episode:   epNum,
				Title:     fmt.Sprintf("%s - Episode %02d", title, epNum),
				Link:      epURL,
				Qualities: qualities,
				Subtitles: subtitles,
			})
		}
	}

	return &ShowData{
		Title:    title,
		ImgURL:   imgURL,
		Episodes: episodes,
	}, nil
}
