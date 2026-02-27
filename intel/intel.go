package intel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"watchtower/feeds"
)

// Brief holds an AI-generated intelligence summary
type Brief struct {
	Summary     string
	KeyThreats  []string
	GeneratedAt time.Time
	Model       string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// GenerateBrief calls Groq to synthesize a brief from news headlines
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

	// Build headline list (top 30 by severity)
	limit := 30
	if len(items) < limit {
		limit = len(items)
	}
	var sb strings.Builder
	for i, item := range items[:limit] {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (%s)\n",
			i+1, item.ThreatLevel.String(), item.Title, item.Source))
	}

	prompt := fmt.Sprintf(`You are a geopolitical analyst. Given these recent news headlines, provide:

1. A concise 3-4 sentence WORLD BRIEF covering the most significant developments.
2. A bullet list of TOP THREATS (max 5), one line each.

Be factual, direct, and analyst-toned. No fluff.

HEADLINES:
%s

Respond in this exact format:
BRIEF:
<your 3-4 sentence summary>

THREATS:
• <threat 1>
• <threat 2>
...`, sb.String())

	body := map[string]interface{}{
		"model":       "llama-3.1-8b-instant",
		"temperature": 0,
		"max_tokens":  512,
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

	content := result.Choices[0].Message.Content
	brief, threats := parseBriefResponse(content)

	return &Brief{
		Summary:     brief,
		KeyThreats:  threats,
		GeneratedAt: time.Now(),
		Model:       result.Model,
	}, nil
}

func parseBriefResponse(content string) (string, []string) {
	var brief string
	var threats []string

	parts := strings.Split(content, "THREATS:")
	if len(parts) >= 2 {
		briefPart := strings.TrimPrefix(parts[0], "BRIEF:")
		brief = strings.TrimSpace(briefPart)

		for _, line := range strings.Split(parts[1], "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "•")
			line = strings.TrimPrefix(line, "-")
			line = strings.TrimPrefix(line, "*")
			line = strings.TrimSpace(line)
			if line != "" {
				threats = append(threats, line)
			}
		}
	} else {
		brief = strings.TrimSpace(content)
	}

	return brief, threats
}
