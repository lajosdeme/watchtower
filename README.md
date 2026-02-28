# 🌍 Watchtower

A clean, minimal, terminal-based global intelligence dashboard.

![wt](https://i.imgur.com/p4BiORi.gif)

## Why Watchtower?

The internet has made information abundant—but navigating the noise has become overwhelming. OSINT tools like WorldMonitor are powerful, but they're designed for intelligence professionals who need every data point. For the average user who just wants to stay informed without drowning in data, there's a gap.

**Watchtower fills that gap.** It lives entirely in your terminal—no browser tabs, no heavy web apps. It's lightweight, fast, and requires only a single API key (and that's optional for the AI brief feature). Just open your terminal and see what's happening in the world.

## Features

| Tab | Contents |
|-----|----------|
| **Global News** | 100+ RSS feeds, keyword threat classification (CRITICAL/HIGH/MEDIUM/LOW/INFO) |
| **Markets** | Live crypto (CoinGecko) + Polymarket prediction markets + stocks + commodities |
| **Local** | Open-Meteo weather (free, no key) + geo-targeted local news |
| **Intel Brief** | AI synthesis of top headlines |

All free APIs — only the LLM requires a key (Groq free tier is generous).

## Install
Pick the best option depending on your platform.

### Universal install script
```
curl -fsSL https://raw.githubusercontent.com/lajosdeme/watchtower/main/install.sh
```

### Homebrew
```
brew tap lajosdeme/watchtower
brew install watchtower
```

### AUR
```
yay -S watchtower-bin
```

### .deb (Ubuntu/Debian)
```
# download from the release page, then:
sudo dpkg -i watchtower_1.0.0_linux_amd64.deb
watchtower --version
```

### .rpm (Fedora)
```
# download from the release page, then:
sudo rpm -i watchtower_1.0.0_linux_amd64.rpm
watchtower --version
```

### Scoop (Windows)
```
scoop bucket add watchtower https://github.com/lajosdeme/scoop-watchtower
scoop install watchtower
```

### From source
```bash
git clone https://github.com/lajosdeme/watchtower
cd watchtower
go mod tidy
make run
# or if using docker: 
make docker-run
```
### If Go is in PATH

```bash
go install github.com/lajosdeme/watchtower@latest
```

## Setup

On first run, Watchtower will prompt you to configure a few things:

1. **Select LLM provider** — Choose Groq (free), OpenAI, Deepseek, Gemini, or Anthropic, or local model
2. **Paste your API key** — Stored locally in `~/.config/watchtower/config.yaml`, never leaves your device
3. **Specify your location** — Enter your city and coordinates for local weather and news

![setup](https://i.imgur.com/7L4soxv.gif)

That's it! The app saves your settings and you're ready to go.

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
| Yahoo Finance | Stocks & commodities | None |
| Open-Meteo | Weather | None |
| Groq / OpenAI / Anthropic / Deepseek / Gemini / Local | AI brief | Required (free tiers available) |

## Tech Stack

- **Language:** Go 1.22
- **TUI:** [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss) + [bubbles](https://github.com/charmbracelet/bubbles)
- **RSS:** [gofeed](https://github.com/mmcdole/gofeed)
- **Config:** [viper](https://github.com/spf13/viper)

## Contributing

Contributions are welcome! Whether you're adding new features, fixing bugs, or improving documentation:

1. Fork the repo
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure code is formatted (`go fmt ./`) and passes tests (`go test ./...`) before submitting.

## Supporting Watchtower

If you find Watchtower useful, consider supporting the project:

- **Star the repo** — it helps visibility
- **Share it** — tell friends and colleagues
- **Contribute** — code, docs, feedback
- **Report issues** — help make it better

## License

MIT License — see [LICENSE](LICENSE) for details.

## Author

[Lajos Deme](https://github.com/lajosdeme)
