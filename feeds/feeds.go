package feeds

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
)

// ThreatLevel represents news item severity
type ThreatLevel int

const (
	ThreatInfo ThreatLevel = iota
	ThreatLow
	ThreatMedium
	ThreatHigh
	ThreatCritical
)

func (t ThreatLevel) String() string {
	switch t {
	case ThreatCritical:
		return "CRITICAL"
	case ThreatHigh:
		return "HIGH"
	case ThreatMedium:
		return "MEDIUM"
	case ThreatLow:
		return "LOW"
	default:
		return "INFO"
	}
}

// NewsItem represents a single news article
type NewsItem struct {
	Title       string
	Source      string
	Published   time.Time
	URL         string
	ThreatLevel ThreatLevel
	Category    string
	IsLocal     bool
}

// GlobalFeeds are world news RSS sources
var GlobalFeeds = []struct {
	Name string
	URL  string
}{
	{"Reuters", "https://feeds.reuters.com/reuters/topNews"},
	{"BBC World", "http://feeds.bbci.co.uk/news/world/rss.xml"},
	{"AP News", "https://rsshub.app/apnews/topics/apf-topnews"},
	{"Al Jazeera", "https://www.aljazeera.com/xml/rss/all.xml"},
	{"The Guardian", "https://www.theguardian.com/world/rss"},
	{"Defense News", "https://www.defensenews.com/arc/outboundfeeds/rss/"},
	{"Politico", "https://rss.politico.com/politics-news.xml"},
	{"Foreign Policy", "https://foreignpolicy.com/feed/"},
}

// LocalFeedURLs generates geo-targeted RSS feeds based on city/country
func LocalFeedURLs(city, country string) []struct{ Name, URL string } {
	return []struct{ Name, URL string }{
		{"Google News Local", fmt.Sprintf("https://news.google.com/rss/search?q=%s+news&hl=en&gl=%s&ceid=%s:en",
			strings.ReplaceAll(city, " ", "+"), country, country)},
		{"Google News Country", fmt.Sprintf("https://news.google.com/rss/headlines/section/geo/%s",
			strings.ReplaceAll(city, " ", "%20"))},
	}
}

// keyword threat classifier
type keywordTier struct {
	level    ThreatLevel
	category string
	words    []string
}

var threatKeywords = []keywordTier{
	{ThreatCritical, "conflict", []string{
		"nuclear", "missile strike", "war declared", "invasion", "airstrike kills",
		"coup", "assassination", "mass casualty", "chemical weapon", "dirty bomb",
		"martial law", "genocide",
	}},
	{ThreatHigh, "security", []string{
		"attack", "bombing", "explosion", "shooting", "killed", "hostage",
		"terrorist", "conflict", "offensive", "troops deployed", "sanctions",
		"ceasefire", "escalation", "warship", "military exercises",
	}},
	{ThreatHigh, "disaster", []string{
		"earthquake", "tsunami", "hurricane", "typhoon", "flood kills",
		"wildfire", "eruption", "catastrophic",
	}},
	{ThreatMedium, "politics", []string{
		"election", "protest", "crisis", "emergency", "shutdown",
		"impeachment", "indicted", "arrested", "detained", "expelled",
		"diplomatic", "summit", "agreement",
	}},
	{ThreatMedium, "economy", []string{
		"recession", "crash", "collapse", "default", "bankrupt",
		"inflation", "unemployment spike", "rate hike", "supply chain",
	}},
	{ThreatMedium, "cyber", []string{
		"hack", "breach", "ransomware", "cyberattack", "data leak",
		"malware", "phishing campaign", "zero-day",
	}},
	{ThreatLow, "general", []string{
		"trade deal", "policy", "reform", "budget", "statement",
		"meeting", "conference", "report",
	}},
}

func classifyThreat(title string) (ThreatLevel, string) {
	lower := strings.ToLower(title)
	for _, tier := range threatKeywords {
		for _, kw := range tier.words {
			if strings.Contains(lower, kw) {
				return tier.level, tier.category
			}
		}
	}
	return ThreatInfo, "general"
}

// FetchGlobalNews fetches and classifies global news items
func FetchGlobalNews(ctx context.Context) ([]NewsItem, error) {
	return fetchFeeds(ctx, GlobalFeeds, false)
}

// FetchLocalNews fetches geo-targeted news items
func FetchLocalNews(ctx context.Context, city, country string) ([]NewsItem, error) {
	return fetchFeeds(ctx, LocalFeedURLs(city, country), true)
}

func fetchFeeds(ctx context.Context, sources []struct{ Name, URL string }, isLocal bool) ([]NewsItem, error) {
	fp := gofeed.NewParser()
	fp.UserAgent = "watchtower/1.0 (Go RSS reader)"

	var (
		mu    sync.Mutex
		items []NewsItem
		wg    sync.WaitGroup
	)

	for _, src := range sources {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()

			fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			feed, err := fp.ParseURLWithContext(url, fetchCtx)
			if err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()

			cutoff := time.Now().Add(-24 * time.Hour)
			for _, entry := range feed.Items {
				if entry.Title == "" {
					continue
				}
				pub := time.Now()
				if entry.PublishedParsed != nil {
					pub = *entry.PublishedParsed
				} else if entry.UpdatedParsed != nil {
					pub = *entry.UpdatedParsed
				}
				if pub.Before(cutoff) {
					continue
				}
				level, cat := classifyThreat(entry.Title)
				link := ""
				if entry.Link != "" {
					link = entry.Link
				}
				items = append(items, NewsItem{
					Title:       entry.Title,
					Source:      name,
					Published:   pub,
					URL:         link,
					ThreatLevel: level,
					Category:    cat,
					IsLocal:     isLocal,
				})
			}
		}(src.Name, src.URL)
	}

	wg.Wait()

	// Sort: critical first, then by time
	sort.Slice(items, func(i, j int) bool {
		if items[i].ThreatLevel != items[j].ThreatLevel {
			return items[i].ThreatLevel > items[j].ThreatLevel
		}
		return items[i].Published.After(items[j].Published)
	})

	// Deduplicate similar titles
	seen := make(map[string]bool)
	var deduped []NewsItem
	for _, item := range items {
		key := strings.ToLower(item.Title[:min(40, len(item.Title))])
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, item)
		}
	}

	return deduped, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
