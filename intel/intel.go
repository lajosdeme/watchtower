package intel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"watchtower/feeds"
	"watchtower/weather"
)

// CountryRisk holds a risk score for one country
type CountryRisk struct {
	Country string
	Score   int    // 0–100
	Reason  string // one short phrase
}

// Brief holds an AI-generated intelligence summary
type Brief struct {
	Summary      string
	KeyThreats   []string
	CountryRisks []CountryRisk
	GeneratedAt  time.Time
	Model        string
}

// LocalBrief holds an AI-generated summary of local news and weather
type LocalBrief struct {
	Summary     string
	GeneratedAt time.Time
	Model       string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// GenerateBrief calls Groq to synthesize a brief, summary, and country risk scores
func GenerateBrief(ctx context.Context, apiKey string, items []feeds.NewsItem) (*Brief, error) {
	if apiKey == "" {
		return &Brief{
			Summary:     "No GROQ_API_KEY set. Add it to ~/.config/watchtower/config.yaml to enable AI briefings.",
			GeneratedAt: time.Now(),
			Model:       "none",
		}, nil
	}

	if len(items) == 0 {
		return &Brief{
			Summary:     "No news items available to summarize.",
			GeneratedAt: time.Now(),
		}, nil
	}

	// Build headline list (top 40 by severity)
	limit := 40
	if len(items) < limit {
		limit = len(items)
	}
	var sb strings.Builder
	for i, item := range items[:limit] {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (%s)\n",
			i+1, item.ThreatLevel.String(), item.Title, item.Source))
	}

	prompt := fmt.Sprintf(`You are a geopolitical intelligence analyst. Analyze these recent headlines and respond in EXACTLY this format with no extra text:

SUMMARY:
<3-4 sentences covering the most critical global developments right now>

THREATS:
• <threat 1, one line>
• <threat 2, one line>
• <threat 3, one line>
• <threat 4, one line>
• <threat 5, one line>

COUNTRY_RISKS:
<CountryName>|<score 0-100>|<one short reason phrase>
<CountryName>|<score 0-100>|<one short reason phrase>
<CountryName>|<score 0-100>|<one short reason phrase>
<CountryName>|<score 0-100>|<one short reason phrase>
<CountryName>|<score 0-100>|<one short reason phrase>
<CountryName>|<score 0-100>|<one short reason phrase>
<CountryName>|<score 0-100>|<one short reason phrase>
<CountryName>|<score 0-100>|<one short reason phrase>

Rules:
- SUMMARY: factual, analyst-toned, no fluff, max 3 sentences
- THREATS: exactly 5 bullets, one line each, most severe first
- COUNTRY_RISKS: exactly 8 countries most prominent in the news, score reflects current instability/risk (100=active war, 0=stable), pipe-separated, short reason (3-5 words max)
- No markdown, no extra formatting, no preamble

HEADLINES:
%s`, sb.String())

	body := map[string]interface{}{
		"model":       "llama-3.1-8b-instant",
		"temperature": 0,
		"max_tokens":  700,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("groq HTTP %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding groq response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response from groq")
	}

	summary, threats, risks := parseBriefResponse(result.Choices[0].Message.Content)

	return &Brief{
		Summary:      summary,
		KeyThreats:   threats,
		CountryRisks: risks,
		GeneratedAt:  time.Now(),
		Model:        result.Model,
	}, nil
}

// GenerateLocalBrief calls Groq to synthesize a local news and weather summary
func GenerateLocalBrief(ctx context.Context, apiKey string, city string, items []feeds.NewsItem, cond *weather.Conditions, forecast []weather.DayForecast) (*LocalBrief, error) {
	if apiKey == "" {
		return &LocalBrief{
			Summary:     "No GROQ_API_KEY set. Add it to ~/.config/watchtower/config.yaml to enable AI briefings.",
			GeneratedAt: time.Now(),
			Model:       "none",
		}, nil
	}

	var sb strings.Builder

	// Build local news headline list (top 20)
	sb.WriteString("LOCAL NEWS HEADLINES:\n")
	limit := 20
	if len(items) > limit {
		items = items[:limit]
	}
	for i, item := range items {
		sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, item.Title, item.Source))
	}

	// Add current weather
	sb.WriteString("\nCURRENT WEATHER:\n")
	if cond != nil {
		sb.WriteString(fmt.Sprintf("Location: %s\n", cond.City))
		sb.WriteString(fmt.Sprintf("Temperature: %.1f°C (feels like %.1f°C)\n", cond.TempC, cond.FeelsLikeC))
		sb.WriteString(fmt.Sprintf("Conditions: %s %s\n", cond.Icon, cond.Description))
		sb.WriteString(fmt.Sprintf("Humidity: %d%%, Wind: %.0f km/h, UV: %.0f\n", cond.Humidity, cond.WindSpeedKmh, cond.UVIndex))
	}

	// Add forecast
	sb.WriteString("\nFORECAST (next 5 days):\n")
	for i, f := range forecast {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("- %s: %s %s, High: %.0f°C, Low: %.0f°C, Rain: %.1fmm\n",
			f.Date.Format("Mon Jan 02"), f.Icon, f.Desc, f.MaxTempC, f.MinTempC, f.RainMM))
	}

	prompt := fmt.Sprintf(`You are a local news and weather analyst. Summarize this information for %s in 2-3 sentences.
Focus on:
1. Any notable local news stories
2. Current weather conditions and any weather concerns for the coming days
3. Short summary of the news stories

Respond in this exact format with no extra text:

SUMMARY:
<2-3 sentence summary>

Rules:
- Keep it concise and practical
- No markdown formatting
- Lead with the most important information
- Never send back the 'DATA' as is, always explain

DATA:
%s`, city, sb.String())

	body := map[string]interface{}{
		"model":       "llama-3.1-8b-instant",
		"temperature": 0,
		"max_tokens":  300,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.groq.com/openai/v1/chat/completions",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("groq HTTP %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding groq response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response from groq")
	}

	summary := parseLocalBriefResponse(result.Choices[0].Message.Content)

	return &LocalBrief{
		Summary:     summary,
		GeneratedAt: time.Now(),
		Model:       result.Model,
	}, nil
}

func parseLocalBriefResponse(content string) string {
	lines := strings.Split(content, "\n")
	inSummary := false
	var summaryLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "SUMMARY:") {
			inSummary = true
			continue
		}
		if inSummary && trimmed != "" {
			summaryLines = append(summaryLines, trimmed)
		}
	}

	if len(summaryLines) > 0 {
		return strings.Join(summaryLines, " ")
	}
	return strings.TrimSpace(content)
}

func parseBriefResponse(content string) (string, []string, []CountryRisk) {
	var summary string
	var threats []string
	var risks []CountryRisk

	// Split into sections
	sections := map[string]string{}
	current := ""
	var buf strings.Builder

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "SUMMARY:":
			if current != "" {
				sections[current] = strings.TrimSpace(buf.String())
			}
			current = "SUMMARY"
			buf.Reset()
		case "THREATS:":
			if current != "" {
				sections[current] = strings.TrimSpace(buf.String())
			}
			current = "THREATS"
			buf.Reset()
		case "COUNTRY_RISKS:":
			if current != "" {
				sections[current] = strings.TrimSpace(buf.String())
			}
			current = "COUNTRY_RISKS"
			buf.Reset()
		default:
			if current != "" {
				buf.WriteString(line + "\n")
			}
		}
	}
	if current != "" {
		sections[current] = strings.TrimSpace(buf.String())
	}

	// Parse SUMMARY
	summary = sections["SUMMARY"]

	// Parse THREATS
	for _, line := range strings.Split(sections["THREATS"], "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "•")
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line != "" {
			threats = append(threats, line)
		}
	}

	// Parse COUNTRY_RISKS  format: Country|score|reason
	for _, line := range strings.Split(sections["COUNTRY_RISKS"], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 2 {
			continue
		}
		country := strings.TrimSpace(parts[0])
		score, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || country == "" {
			continue
		}
		reason := ""
		if len(parts) == 3 {
			reason = strings.TrimSpace(parts[2])
		}
		risks = append(risks, CountryRisk{
			Country: country,
			Score:   clamp(score, 0, 100),
			Reason:  reason,
		})
	}

	return summary, threats, risks
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
