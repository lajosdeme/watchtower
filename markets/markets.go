package markets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// CryptoPrice holds price data for one coin
type CryptoPrice struct {
	ID           string
	Symbol       string
	Name         string
	PriceUSD     float64
	Change24h    float64
	MarketCapUSD float64
	Volume24hUSD float64
	LastUpdated  time.Time
}

// StockIndex holds data for a market index (S&P 500, Dow, etc.)
type StockIndex struct {
	Symbol    string
	Name      string
	Price     float64
	PrevClose float64
	ChangePct float64
}

// Commodity holds price data for a commodity (oil, gold, etc.)
type Commodity struct {
	Symbol    string
	Name      string
	Price     float64
	PrevClose float64
	Unit      string // e.g. "$/bbl", "$/oz", "$/t"
	ChangePct float64
}

// PredictionMarket holds a Polymarket market
type PredictionMarket struct {
	Title       string
	Probability float64 // 0.0 to 1.0
	Volume      float64
	Category    string
	EndDate     string
	Slug        string
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// ─── Crypto ───────────────────────────────────────────────────────────────────

// FetchCryptoPrices fetches prices for the given CoinGecko IDs
func FetchCryptoPrices(ctx context.Context, ids []string) ([]CryptoPrice, error) {
	joined := strings.Join(ids, ",")
	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s"+
			"&order=market_cap_desc&per_page=20&page=1&sparkline=false"+
			"&price_change_percentage=24h",
		joined,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coingecko request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("CoinGecko rate limited (try again in ~1min)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("coingecko HTTP %d", resp.StatusCode)
	}

	var raw []struct {
		ID                       string  `json:"id"`
		Symbol                   string  `json:"symbol"`
		Name                     string  `json:"name"`
		CurrentPrice             float64 `json:"current_price"`
		PriceChangePercentage24h float64 `json:"price_change_percentage_24h"`
		MarketCap                float64 `json:"market_cap"`
		TotalVolume              float64 `json:"total_volume"`
		LastUpdated              string  `json:"last_updated"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding coingecko response: %w", err)
	}

	prices := make([]CryptoPrice, 0, len(raw))
	for _, r := range raw {
		t, _ := time.Parse(time.RFC3339, r.LastUpdated)
		prices = append(prices, CryptoPrice{
			ID:           r.ID,
			Symbol:       strings.ToUpper(r.Symbol),
			Name:         r.Name,
			PriceUSD:     r.CurrentPrice,
			Change24h:    r.PriceChangePercentage24h,
			MarketCapUSD: r.MarketCap,
			Volume24hUSD: r.TotalVolume,
			LastUpdated:  t,
		})
	}

	return prices, nil
}

// ─── Yahoo Finance chart endpoint ────────────────────────────────────────────
// Uses the same v8/finance/chart endpoint as:
//   curl -s -L "https://query1.finance.yahoo.com/v8/finance/chart/%5EGSPC" \
//        -H "User-Agent: Mozilla/5.0"
// The meta object contains regularMarketPrice, previousClose, and
// regularMarketChangePercent — everything we need in one request.

type yahooMeta struct {
	RegularMarketPrice         float64 `json:"regularMarketPrice"`
	PreviousClose              float64 `json:"previousClose"`
	RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
	ChartPreviousClose         float64 `json:"chartPreviousClose"`
	Symbol                     string  `json:"symbol"`
}

func fetchYahooChart(ctx context.Context, symbol string) (yahooMeta, error) {
	url := "https://query1.finance.yahoo.com/v8/finance/chart/" + symbol

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return yahooMeta{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return yahooMeta{}, fmt.Errorf("yahoo chart request for %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return yahooMeta{}, fmt.Errorf("yahoo chart HTTP %d for %s", resp.StatusCode, symbol)
	}

	var envelope struct {
		Chart struct {
			Result []struct {
				Meta yahooMeta `json:"meta"`
			} `json:"result"`
			Error *struct {
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return yahooMeta{}, fmt.Errorf("decoding yahoo chart for %s: %w", symbol, err)
	}
	if envelope.Chart.Error != nil {
		return yahooMeta{}, fmt.Errorf("yahoo chart error for %s: %s", symbol, envelope.Chart.Error.Description)
	}
	if len(envelope.Chart.Result) == 0 {
		return yahooMeta{}, fmt.Errorf("no results from yahoo chart for %s", symbol)
	}

	meta := envelope.Chart.Result[0].Meta
	// Compute pct change if not provided directly
	if meta.RegularMarketChangePercent == 0 && meta.PreviousClose != 0 {
		meta.RegularMarketChangePercent = ((meta.RegularMarketPrice - meta.PreviousClose) / meta.PreviousClose) * 100
	}
	// Fallback: use chartPreviousClose if previousClose is zero
	if meta.PreviousClose == 0 && meta.ChartPreviousClose != 0 {
		meta.PreviousClose = meta.ChartPreviousClose
		if meta.RegularMarketChangePercent == 0 {
			meta.RegularMarketChangePercent = ((meta.RegularMarketPrice - meta.ChartPreviousClose) / meta.ChartPreviousClose) * 100
		}
	}

	return meta, nil
}

// ─── Stock Indices ────────────────────────────────────────────────────────────

// FetchStockIndices fetches S&P 500 and Dow Jones via Yahoo Finance chart API
func FetchStockIndices(ctx context.Context) ([]StockIndex, error) {
	type indexDef struct {
		yahooSymbol string // URL-encoded if needed
		displayName string
	}
	defs := []indexDef{
		{"%5EGSPC", "S&P 500"},
		{"%5EDJI", "Dow Jones"},
	}

	type result struct {
		idx StockIndex
		err error
		pos int
	}

	results := make([]result, len(defs))
	var wg sync.WaitGroup

	for i, def := range defs {
		wg.Add(1)
		go func(i int, sym, name string) {
			defer wg.Done()
			meta, err := fetchYahooChart(ctx, sym)
			if err != nil {
				results[i] = result{pos: i, err: err}
				return
			}
			results[i] = result{
				pos: i,
				idx: StockIndex{
					Symbol:    meta.Symbol,
					Name:      name,
					Price:     meta.RegularMarketPrice,
					PrevClose: meta.PreviousClose,
					ChangePct: meta.RegularMarketChangePercent,
				},
			}
		}(i, def.yahooSymbol, def.displayName)
	}

	wg.Wait()

	var indices []StockIndex
	var errs []string
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
		} else {
			indices = append(indices, r.idx)
		}
	}

	if len(indices) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return indices, nil
}

// ─── Commodities ──────────────────────────────────────────────────────────────

// FetchCommodities fetches WTI crude oil, gold, and copper via Yahoo Finance chart API
// Tickers: CL=F (WTI crude), GC=F (gold), HG=F (copper)
func FetchCommodities(ctx context.Context) ([]Commodity, error) {
	type commDef struct {
		yahooSymbol string
		name        string
		unit        string
	}
	defs := []commDef{
		{"CL%3DF", "WTI Crude Oil", "$/bbl"},
		{"GC%3DF", "Gold", "$/oz"},
		{"HG%3DF", "Copper", "$/lb"},
	}

	type result struct {
		comm Commodity
		err  error
		pos  int
	}

	results := make([]result, len(defs))
	var wg sync.WaitGroup

	for i, def := range defs {
		wg.Add(1)
		go func(i int, sym, name, unit string) {
			defer wg.Done()
			meta, err := fetchYahooChart(ctx, sym)
			if err != nil {
				results[i] = result{pos: i, err: err}
				return
			}
			results[i] = result{
				pos: i,
				comm: Commodity{
					Symbol:    meta.Symbol,
					Name:      name,
					Price:     meta.RegularMarketPrice,
					PrevClose: meta.PreviousClose,
					Unit:      unit,
					ChangePct: meta.RegularMarketChangePercent,
				},
			}
		}(i, def.yahooSymbol, def.name, def.unit)
	}

	wg.Wait()

	// Return in definition order
	var commodities []Commodity
	var errs []string
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
		} else {
			commodities = append(commodities, r.comm)
		}
	}

	if len(commodities) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return commodities, nil
}

// ─── Prediction Markets ───────────────────────────────────────────────────────

// FetchPredictionMarkets fetches top geopolitical markets from Polymarket
func FetchPredictionMarkets(ctx context.Context) ([]PredictionMarket, error) {
	url := "https://gamma-api.polymarket.com/markets?limit=20&active=true&closed=false&order=volume&ascending=false&tag_slug=politics"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polymarket request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("polymarket HTTP %d", resp.StatusCode)
	}

	var raw []struct {
		Question      string `json:"question"`
		OutcomePrices string `json:"outcomePrices"`
		Volume        string `json:"volume"`
		EndDateIso    string `json:"endDateIso"`
		Slug          string `json:"slug"`
		Tags          []struct {
			Slug string `json:"slug"`
		} `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding polymarket response: %w", err)
	}

	result := make([]PredictionMarket, 0, len(raw))
	for _, r := range raw {
		if r.Question == "" {
			continue
		}
		prob := 0.5
		var prices []string
		if err := json.Unmarshal([]byte(r.OutcomePrices), &prices); err == nil && len(prices) > 0 {
			if f, err := strconv.ParseFloat(prices[0], 64); err == nil {
				prob = f
			}
		}
		var vol float64
		fmt.Sscanf(r.Volume, "%f", &vol)

		cat := "politics"
		if len(r.Tags) > 0 {
			cat = r.Tags[0].Slug
		}
		endDate := ""
		if len(r.EndDateIso) >= 10 {
			endDate = r.EndDateIso[:10]
		}
		result = append(result, PredictionMarket{
			Title:       r.Question,
			Probability: prob,
			Volume:      vol,
			Category:    cat,
			EndDate:     endDate,
			Slug:        r.Slug,
		})
	}
	return result, nil
}

// ─── Formatters ───────────────────────────────────────────────────────────────

// FormatPrice returns a human-readable price string with thousands separators
func FormatPrice(p float64) string {
	if p >= 1000 {
		return "$" + commaSeparate(fmt.Sprintf("%.0f", p))
	} else if p >= 1 {
		return fmt.Sprintf("$%.2f", p)
	} else if p >= 0.01 {
		return fmt.Sprintf("$%.4f", p)
	} else {
		return fmt.Sprintf("$%.6f", p)
	}
}

// FormatLargeNum abbreviates large numbers (e.g. 1200000 → $1.2M)
func FormatLargeNum(n float64) string {
	switch {
	case n >= 1e12:
		return fmt.Sprintf("$%.2fT", n/1e12)
	case n >= 1e9:
		return fmt.Sprintf("$%.2fB", n/1e9)
	case n >= 1e6:
		return fmt.Sprintf("$%.1fM", n/1e6)
	default:
		return fmt.Sprintf("$%.0f", n)
	}
}

// commaSeparate inserts commas into an integer string: "1234567" → "1,234,567"
func commaSeparate(s string) string {
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	n := len(s)
	if n <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	start := n % 3
	if start > 0 {
		b.WriteString(s[:start])
	}
	for i := start; i < n; i += 3 {
		if i > 0 || start > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
