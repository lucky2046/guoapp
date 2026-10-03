package core

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const hongguoShareTextLimit = 4096

var (
	hongguoShareURLPattern    = regexp.MustCompile(`https?://[^\s<>"'\\]+`)
	hongguoShareTitlePattern  = regexp.MustCompile(`《([^》]{1,80})》`)
	hongguoShareSeriesPattern = regexp.MustCompile(`(?i)(?:series_id(?:_str)?|book_id)\s*[=:"']+\s*([0-9]{1,32})`)
	hongguoShareHosts         = []string{
		"hongguoduanju.com",
		"fanqienovel.com",
		"fqnovel.com",
		"changdunovel.com",
		"novelquickapp.com",
		"fanqieopen.com",
		"snssdk.com",
		"toutiao.com",
		"toutiaoapi.com",
		"iesdouyin.com",
		"fanqie.cn",
		"novelfm.com",
		"changread.com",
	}
	hongguoShareKeywords = []string{
		"红果", "番茄", "免费短剧", "短剧口令", "复制此条", "长按复制",
		"打开【红果", "打开【番茄", "红果免费", "番茄免费",
	}
	hongguoSharePunctuation = ".,;:!?)]}'\"、，。；：！？》）」』"
)

func extractHongguoShareTitle(text string) string {
	match := hongguoShareTitlePattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func extractHongguoShareURLs(text string) []string {
	raw := hongguoShareURLPattern.FindAllString(text, 8)
	urls := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, item := range raw {
		item = strings.TrimRight(item, hongguoSharePunctuation)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		urls = append(urls, item)
	}
	return urls
}

func hongguoShareHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	host, _, _ = strings.Cut(host, ":")
	host = strings.TrimPrefix(host, "www.")
	if host == "" {
		return false
	}
	for _, suffix := range hongguoShareHosts {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func hongguoShareURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return hongguoShareHost(parsed.Host)
}

func looksLikeHongguoShare(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > hongguoShareTextLimit {
		return false
	}
	if seriesIDFromHongguoShareText(text) != "" {
		return true
	}
	for _, item := range extractHongguoShareURLs(text) {
		if hongguoShareURL(item) {
			return true
		}
	}
	if extractHongguoShareTitle(text) == "" {
		return false
	}
	for _, keyword := range hongguoShareKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func seriesIDFromHongguoShareURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	for _, key := range []string{"series_id", "series_id_str", "seriesId", "book_id", "bookId"} {
		if id := strings.TrimSpace(query.Get(key)); hongguoNumericID.MatchString(id) {
			return id
		}
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	candidate := parts[len(parts)-1]
	if hongguoNumericID.MatchString(candidate) {
		return candidate
	}
	if len(parts) >= 2 && (parts[len(parts)-2] == "page" || parts[len(parts)-2] == "detail" || parts[len(parts)-2] == "video") &&
		hongguoNumericID.MatchString(parts[len(parts)-1]) {
		return parts[len(parts)-1]
	}
	return ""
}

func seriesIDFromHongguoShareHTML(body string) string {
	if match := hongguoShareSeriesPattern.FindStringSubmatch(body); len(match) == 2 && hongguoNumericID.MatchString(match[1]) {
		return match[1]
	}
	return ""
}

func seriesIDFromHongguoShareText(text string) string {
	for _, item := range extractHongguoShareURLs(text) {
		if id := seriesIDFromHongguoShareURL(item); id != "" {
			return id
		}
	}
	if match := hongguoShareSeriesPattern.FindStringSubmatch(text); len(match) == 2 {
		return match[1]
	}
	return ""
}

func hongguoShareDrama(seriesID, title string) Drama {
	if !hongguoNumericID.MatchString(seriesID) {
		return Drama{}
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = seriesID
	}
	return Drama{
		ID: providerDramaID(sourceHongguo, seriesID), Source: sourceHongguo, SourceID: seriesID,
		Title: title, Name: title, ChannelName: "红果", CategoryName: "短剧",
	}
}

func pickHongguoShareDrama(title string, dramas []Drama) Drama {
	key := hongguoSearchText(title)
	if key == "" {
		return Drama{}
	}
	var exact []Drama
	var prefix []Drama
	for _, drama := range dramas {
		current := hongguoSearchText(drama.DisplayTitle())
		if current == "" {
			continue
		}
		if current == key {
			exact = append(exact, drama)
			continue
		}
		if strings.HasPrefix(current, key) || strings.HasPrefix(key, current) {
			prefix = append(prefix, drama)
		}
	}
	if len(exact) == 1 {
		return exact[0]
	}
	if len(exact) == 0 && len(prefix) == 1 {
		return prefix[0]
	}
	return Drama{}
}

func (d *Downloader) resolveHongguoShare(ctx context.Context, text string) (Drama, error) {
	text = strings.TrimSpace(text)
	if !looksLikeHongguoShare(text) {
		return Drama{}, nil
	}
	title := extractHongguoShareTitle(text)
	seriesID := seriesIDFromHongguoShareText(text)
	if seriesID == "" {
		for _, item := range extractHongguoShareURLs(text) {
			if err := ctx.Err(); err != nil {
				return Drama{}, err
			}
			if !hongguoShareURL(item) {
				continue
			}
			finalURL, body, err := d.fetchHongguoSharePage(ctx, item)
			if err != nil {
				continue
			}
			seriesID = firstNonEmpty(seriesIDFromHongguoShareURL(finalURL), seriesIDFromHongguoShareHTML(body))
			if seriesID != "" {
				break
			}
		}
	}
	if seriesID != "" {
		entry, err := d.hongguoAppDetail(ctx, seriesID)
		if err == nil && entry.Drama.ID != "" {
			drama := entry.Drama
			if title != "" && (drama.Title == "" || drama.Title == drama.SourceID) {
				drama.Title, drama.Name = title, title
			}
			return drama, nil
		}
		return hongguoShareDrama(seriesID, title), nil
	}
	if title == "" {
		return Drama{}, nil
	}
	entry, err := d.searchHongguoDramas(ctx, title)
	if err != nil {
		return Drama{}, err
	}
	return pickHongguoShareDrama(title, entry.Dramas), nil
}

func (d *Downloader) fetchHongguoSharePage(ctx context.Context, rawURL string) (string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || !hongguoShareURL(rawURL) {
		return "", "", errors.New("分享链接无效")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", "", err
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	response, err := d.doCatalogRequestWithTimeout(request, 12*time.Second)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if strings.Contains(contentType, "image/") || strings.Contains(contentType, "video/") || strings.Contains(contentType, "audio/") {
		return "", "", errors.New("分享链接不是可解析页面")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20+1))
	if err != nil {
		return "", "", err
	}
	if len(body) > 1<<20 {
		return "", "", errors.New("分享页面过大")
	}
	finalURL := rawURL
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL.String()
	}
	return finalURL, string(body), nil
}
