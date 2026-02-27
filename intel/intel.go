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

var httpClient = &http.Client{Timeout: 30 * time.Second}

// GenerateBrief calls Groq to synthesize a brief, summary, and country risk scores
func GenerateBrief(ctx context.Context, apiKey string, items []feeds.NewsItem) (*Brief, error) {
	if apiKey == "" {
		return &Brief{
			Summary:     "No GROQ_API_KEY set. Add it to ~/.config/worldtui/config.yaml to enable AI briefings.",
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
