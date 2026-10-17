// 1337x search backend.
package search

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const x1337Base = "https://1337x.to"

// X1337Searcher implements Searcher for 1337x.to
type X1337Searcher struct {
	client *http.Client
}

func NewX1337Searcher() *X1337Searcher {
	return &X1337Searcher{
		client: &http.Client{Timeout: 4 * time.Second},
	}
}

func (s *X1337Searcher) Name() string { return "1337x" }

func (s *X1337Searcher) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	searchURL := fmt.Sprintf("%s/search/%s/1/", x1337Base, url.PathEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/124.0.0.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("1337x search: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []Result
	doc.Find("table.table-list tbody tr").Each(func(i int, row *goquery.Selection) {
		if i >= limit {
			return
		}

		nameSel := row.Find("td.name a").Last()
		title := strings.TrimSpace(nameSel.Text())
		href, _ := nameSel.Attr("href")

		seeders := parseIntFromText(row.Find("td.seeds").Text())
		leechers := parseIntFromText(row.Find("td.leeches").Text())
		size := strings.TrimSpace(row.Find("td.size").Text())
		// Remove invisible text nodes from size
		size = strings.Split(size, "\n")[0]

		if title == "" {
			return
		}

		results = append(results, Result{
			Title:      title,
			TorrentURL: x1337Base + href, // topic page, magnet resolved on detail page
			Seeders:    seeders,
			Leechers:   leechers,
			Size:       size,
			Source:     "1337x",
			Category:   "music",
		})
	})

	return results, nil
}

// GetMagnet fetches the magnet link from a 1337x torrent detail page.
func (s *X1337Searcher) GetMagnet(ctx context.Context, topicURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, topicURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	magnet, exists := doc.Find("a[href^='magnet:']").First().Attr("href")
	if !exists {
		return "", fmt.Errorf("1337x: magnet link not found on page %s", topicURL)
	}
	return magnet, nil
}
