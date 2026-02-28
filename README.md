# 🌍 watchtower

A clean, minimal, terminal-based global intelligence dashboard.

![wt](https://github.com/user-attachments/assets/635aa9cb-2603-4654-9f55-68b73d8ea4d0)

## Features

| Tab | Contents |
|-----|----------|
| **Global News** | 100+ RSS feeds, keyword threat classification (CRITICAL/HIGH/MEDIUM/LOW/INFO) |
| **Markets** | Live crypto prices (CoinGecko, free) + Polymarket prediction markets |
| **Local** | Open-Meteo weather (free, no key) + geo-targeted local news |
| **Intel Brief** | Groq Llama 3.1 AI synthesis of top headlines |

All free APIs — only Groq requires a key (free tier is generous).

## Quick Start

### Prerequisites

- Go 1.22+
- A terminal that supports 256 colors (most modern terminals do)

### Install

```bash
git clone https://github.com/lajosdeme/watchtower
cd watchtower
go mod tidy
go build -o watchtower ./cmd/watchtower
./watchtower
```

On first run, a default config is created at `~/.config/watchtower/config.yaml`. Edit it:

```yaml
# Get free key at https://console.groq.com
groq_api_key: "gsk_YOUR_KEY_HERE"

location:
  city: "Lisbon"
  country: "PT"
  latitude: 38.7169
  longitude: -9.1395

refresh_seconds: 120

crypto_pairs:
  - bitcoin
  - ethereum
  - solana
  - binancecoin
  - ripple
```

Then run again:

```bash
./watchtower
```

### One-liner install (if Go is in PATH)

```bash
go install github.com/lajosdeme/watchtower/cmd/watchtower@latest
```

## Keybindings

| Key | Action |
|-----|--------|
| `1` `2` `3` `4` | Jump to tab |
| `Tab` / `Shift+Tab` | Next / previous tab |
| `← →` / `h l` | Switch tabs |
| `↑ ↓` / `j k` | Scroll content |
| `d` / `u` | Half-page down/up |
| `g` / `G` | Top / bottom |
| `r` | Force refresh all data |
| `b` | Generate AI brief (on Brief tab) |
| `q` / `Ctrl+C` | Quit |

## Data Sources

| Source | What | Key? |
|--------|------|------|
| Reuters, BBC, AP, Al Jazeera, etc. | Global news | None (RSS) |
| Google News | Local news | None (RSS) |
| CoinGecko | Crypto prices | None (public API) |
| Polymarket | Prediction markets | None (public API) |
| Open-Meteo | Weather | None |
| Groq (Llama 3.1 8B) | AI brief | Free at console.groq.com |

## Tech Stack

- **Language:** Go 1.22
- **TUI:** [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss) + [bubbles](https://github.com/charmbracelet/bubbles)
- **RSS:** [gofeed](https://github.com/mmcdole/gofeed)
- **Config:** [viper](https://github.com/spf13/viper)

## Extending

### Add more crypto pairs

Edit `~/.config/watchtower/config.yaml` — use any CoinGecko coin ID:

```yaml
crypto_pairs:
  - bitcoin
  - ethereum
  - dogecoin
  - chainlink
```

### Add more RSS feeds

Edit `internal/feeds/feeds.go` and add to `GlobalFeeds`:

```go
{"My Source", "https://example.com/rss.xml"},
```

### Adjust threat keywords

Edit the `threatKeywords` slice in `internal/feeds/feeds.go`.

## License

MIT
