/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package scraper

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ScrapeShowData(showURL string, source string) (*ShowData, error) {
	targetEpNum := ExtractTargetEpisodeNumber(showURL)

	html, err := getHTML(showURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	switch source {
	case "DramaKey":
		return ScrapeDramaKey(doc, showURL, targetEpNum)
	case "KissAsia":
		return ScrapeKissAsia(doc, showURL, targetEpNum)
	case "DramaCool":
		return ScrapeDramaCool(doc, showURL, targetEpNum)
	}

	return nil, fmt.Errorf("unsupported source: %s", source)
}
