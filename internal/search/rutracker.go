// Rutracker search backend.
//
// Rutracker requires an authenticated session.
// Authentication can be done via:
//  1. Direct Login(username, password)
//  2. SetCookie(cookieStr) — Recommended when Rutracker is behind Cloudflare Turnstile/DDoS protection.
//     Users can copy bb_data / cf_clearance cookies from their browser.
package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	defaultRutrackerHost = "https://rutracker.org"
)

// RutrackerSearcher implements Searcher for Rutracker.
type RutrackerSearcher struct {
	client   *http.Client
	jar      *cookiejar.Jar
	baseURL  string
	loggedIn bool
}

// NewRutrackerSearcher creates a new Rutracker backend.
func NewRutrackerSearcher() (*RutrackerSearcher, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	rs := &RutrackerSearcher{
		jar:     jar,
		baseURL: defaultRutrackerHost,
	}
	rs.initClient(nil)
	return rs, nil
}

func (r *RutrackerSearcher) initClient(proxyURL *url.URL) {
	transport := &http.Transport{
		Proxy:              http.ProxyFromEnvironment,
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
	}
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	r.client = &http.Client{
		Jar:       r.jar,
		Timeout:   30 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 10 {
				return fmt.Errorf("too many redirects")
			}
			if len(via) > 0 {
				req.Header.Set("User-Agent", via[0].Header.Get("User-Agent"))
			}
			return nil
		},
	}
}

// SetBaseURL configures the Rutracker domain (e.g. https://rutracker.org or https://rutracker.net).
func (r *RutrackerSearcher) SetBaseURL(u string) {
	u = strings.TrimRight(u, "/")
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	r.baseURL = u
}

// SetProxy configures an HTTP/HTTPS/SOCKS5 proxy for Rutracker requests.
func (r *RutrackerSearcher) SetProxy(proxyStr string) error {
	if strings.TrimSpace(proxyStr) == "" {
		r.initClient(nil)
		return nil
	}
	pURL, err := url.Parse(proxyStr)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	r.initClient(pURL)
	return nil
}

// SetCookie parses and sets browser cookies (e.g. bb_data or full cookie string).
// This is the most reliable authentication method against Cloudflare.
func (r *RutrackerSearcher) SetCookie(rawCookie string) error {
	rawCookie = strings.TrimSpace(rawCookie)
	if rawCookie == "" {
		return fmt.Errorf("cookie cannot be empty")
	}

	domains := []string{"https://rutracker.org", "https://rutracker.net", r.baseURL}

	// Check if user just pasted the session value (e.g. "0-12345-abcdef...")
	if !strings.Contains(rawCookie, "=") {
		rawCookie = "bb_session=" + rawCookie
	}

	parts := strings.Split(rawCookie, ";")
	for _, domain := range domains {
		u, err := url.Parse(domain)
		if err != nil {
			continue
		}
		var cookies []*http.Cookie
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				name := strings.TrimSpace(kv[0])
				val := strings.TrimSpace(kv[1])
				cookies = append(cookies, &http.Cookie{
					Name:   name,
					Value:  val,
					Path:   "/",
					Domain: u.Hostname(),
				})
			}
		}
		r.jar.SetCookies(u, cookies)
	}

	r.loggedIn = true
	return nil
}

// Login authenticates with Rutracker credentials.
// Note: If Cloudflare Turnstile blocks the request (HTTP 403), an error will prompt
// the user to use SetCookie instead.
func (r *RutrackerSearcher) Login(username, password string) error {
	loginURL := fmt.Sprintf("%s/forum/login.php", r.baseURL)

	// Step 1: Pre-login GET
	getReq, err := http.NewRequest(http.MethodGet, loginURL, nil)
	if err != nil {
		return err
	}
	r.setCommonHeaders(getReq)
	preResp, err := r.client.Do(getReq)
	if err != nil {
		return fmt.Errorf("ошибка подключения к Rutracker: %w", err)
	}
	preBody, _ := io.ReadAll(preResp.Body)
	preResp.Body.Close()

	if preResp.StatusCode == http.StatusForbidden && strings.Contains(string(preBody), "challenges.cloudflare.com") {
		return fmt.Errorf("Rutracker защищен Cloudflare (код 403). Пожалуйста, авторизуйтесь через ввод Cookie (bb_data) из браузера!")
	}

	// Step 2: POST credentials with exact format and proper Windows-1251 button encoding
	bodyData := fmt.Sprintf("redirect=groupcp.php&login_username=%s&login_password=%s&login=%%C2%%F5%%EE%%E4",
		url.QueryEscape(username), url.QueryEscape(password))

	postReq, err := http.NewRequest(http.MethodPost, loginURL, strings.NewReader(bodyData))
	if err != nil {
		return err
	}
	r.setCommonHeaders(postReq)
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("Referer", fmt.Sprintf("%s/forum/login.php?redirect=groupcp.php", r.baseURL))
	postReq.Header.Set("Origin", r.baseURL)
	postReq.Header.Set("Sec-GPC", "1")
	postReq.Header.Set("Upgrade-Insecure-Requests", "1")
	postReq.Header.Set("Sec-Fetch-Dest", "document")
	postReq.Header.Set("Sec-Fetch-Mode", "navigate")
	postReq.Header.Set("Sec-Fetch-Site", "same-origin")
	postReq.Header.Set("Sec-Fetch-User", "?1")

	resp, err := r.client.Do(postReq)
	if err != nil {
		return fmt.Errorf("ошибка отправки формы Rutracker: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения ответа Rutracker: %w", err)
	}
	bodyStr := string(body)

	if resp.StatusCode == http.StatusForbidden || strings.Contains(bodyStr, "challenges.cloudflare.com") {
		return fmt.Errorf("Rutracker заблокировал запрос через Cloudflare (код 403). Пожалуйста, используйте вход через Cookie (bb_data) из браузера!")
	}

	if strings.Contains(bodyStr, "logged-in-as") ||
		strings.Contains(bodyStr, "profile.php?mode=viewprofile") ||
		r.hasSessionCookie() {
		r.loggedIn = true
		return nil
	}

	if strings.Contains(bodyStr, "Неверный логин") || strings.Contains(bodyStr, "Вы ввели неверный") {
		return fmt.Errorf("неверный логин или пароль")
	}

	if r.hasSessionCookie() {
		r.loggedIn = true
		return nil
	}

	return fmt.Errorf("не удалось войти на Rutracker. Если включена защита Cloudflare, используйте вход по Cookie")
}

func (r *RutrackerSearcher) hasSessionCookie() bool {
	u, _ := url.Parse(r.baseURL)
	for _, c := range r.jar.Cookies(u) {
		if c.Name == "bb_session" || c.Name == "bb_t" || c.Name == "bb_guid" || c.Name == "bb_data" || strings.HasPrefix(c.Name, "bb_") {
			if len(c.Value) > 2 {
				return true
			}
		}
	}
	return false
}

func (r *RutrackerSearcher) Name() string { return "rutracker" }

func (r *RutrackerSearcher) IsLoggedIn() bool { return r.loggedIn }

// Search implements Searcher.
func (r *RutrackerSearcher) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if !r.loggedIn {
		return nil, fmt.Errorf("rutracker: не авторизован")
	}

	searchURL := fmt.Sprintf("%s/forum/tracker.php", r.baseURL)
	form := url.Values{"nm": {query}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	r.setCommonHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", searchURL)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rutracker search error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Rutracker вернул 403 (Cloudflare). Обновите Cookie (bb_data + cf_clearance)")
	}

	return r.parseResults(resp, limit)
}

func (r *RutrackerSearcher) parseResults(resp *http.Response, limit int) ([]Result, error) {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse rutracker html: %w", err)
	}

	var results []Result

	doc.Find("tr.tCenter.hl-tr").Each(func(i int, s *goquery.Selection) {
		if i >= limit {
			return
		}

		titleSel := s.Find("td.t-title a.tLink")
		title := strings.TrimSpace(titleSel.Text())
		topicHref, _ := titleSel.Attr("href")

		seeders := parseIntFromText(s.Find("b.seedmed").Text())
		leechers := parseIntFromText(s.Find("td.leechmed b").Text())
		size := strings.TrimSpace(s.Find("td.tor-size u").Text())
		category := strings.TrimSpace(s.Find("td.f-name a").Text())

		topicURL := topicHref
		if !strings.HasPrefix(topicHref, "http") {
			topicURL = fmt.Sprintf("%s/forum/%s", r.baseURL, strings.TrimPrefix(topicHref, "/forum/"))
		}

		if title == "" {
			return
		}

		results = append(results, Result{
			Title:      title,
			TorrentURL: topicURL,
			Seeders:    seeders,
			Leechers:   leechers,
			Size:       size,
			Source:     "rutracker",
			Category:   category,
		})
	})

	return results, nil
}

// DownloadTorrentFile downloads the .torrent file for a topic.
func (r *RutrackerSearcher) DownloadTorrentFile(ctx context.Context, topicURL string) ([]byte, error) {
	if !r.loggedIn {
		return nil, fmt.Errorf("rutracker: не авторизован")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, topicURL, nil)
	if err != nil {
		return nil, err
	}
	r.setCommonHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch topic page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	dlHref, exists := doc.Find("a.dl-stub").Attr("href")
	if !exists {
		doc.Find("a[href*='dl.php']").Each(func(_ int, s *goquery.Selection) {
			if h, ok := s.Attr("href"); ok && dlHref == "" {
				dlHref = h
			}
		})
	}
	if dlHref == "" {
		return nil, fmt.Errorf("ссылка на .torrent не найдена (возможно требуется обновить cookie сессию)")
	}

	dlURL := dlHref
	if !strings.HasPrefix(dlHref, "http") {
		dlURL = fmt.Sprintf("%s/forum/%s", r.baseURL, strings.TrimPrefix(dlHref, "/forum/"))
	}

	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return nil, err
	}
	r.setCommonHeaders(dlReq)
	dlReq.Header.Set("Referer", topicURL)

	dlResp, err := r.client.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("download torrent file: %w", err)
	}
	defer dlResp.Body.Close()

	return io.ReadAll(dlResp.Body)
}

func (r *RutrackerSearcher) setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:157.0) Gecko/20100101 Firefox/157.0")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
}
