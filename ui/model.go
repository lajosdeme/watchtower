package ui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"watchtower/config"
	"watchtower/feeds"
	"watchtower/intel"
	"watchtower/markets"
	"watchtower/weather"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab indices — 3 tabs now
const (
	TabOverview = iota
	TabNews
	TabLocal
	tabCount
)

// Message types
type (
	globalNewsMsg struct {
		items []feeds.NewsItem
		err   error
	}
	localNewsMsg struct {
		items []feeds.NewsItem
		err   error
	}
	cryptoMsg struct {
		prices []markets.CryptoPrice
		err    error
	}
	stockMsg struct {
		indices []markets.StockIndex
		err     error
	}
	commodityMsg struct {
		commodities []markets.Commodity
		err         error
	}
	polymarketMsg struct {
		markets []markets.PredictionMarket
		err     error
	}
	weatherMsg struct {
		cond     *weather.Conditions
		forecast []weather.DayForecast
		err      error
	}
	briefMsg struct {
		brief     *intel.Brief
		err       error
		fromCache bool
	}
	tickMsg time.Time
)

// openURLMsg triggers opening a URL in the system browser
type openURLMsg struct{ url string }

// clearStatusMsg clears the status bar message
type clearStatusMsg struct{}

// Model is the root bubbletea model
type Model struct {
	cfg       *config.Config
	width     int
	height    int
	activeTab int

	// Data
	globalNews   []feeds.NewsItem
	localNews    []feeds.NewsItem
	cryptoPrices []markets.CryptoPrice
	stockIndices []markets.StockIndex
	commodities  []markets.Commodity
	polyMarkets  []markets.PredictionMarket
	weatherCond  *weather.Conditions
	forecast     []weather.DayForecast
	brief        *intel.Brief

	// News selection (for browser open)
	selectedNewsIdx      int
	newsHeaderLines      int // line count of the header above the article list (for scroll tracking)
	selectedLocalNewsIdx int
	localNewsHeaderLines int
	statusMsg            string
	statusExpiry         time.Time

	// State
	loading     map[string]bool
	errors      map[string]string
	lastRefresh time.Time

	// Viewports for scrollable panes
	viewports [tabCount]viewport.Model
	spinner   spinner.Model
}

func NewModel(cfg *config.Config) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleSpinner

	vps := [tabCount]viewport.Model{}
	for i := range vps {
		vps[i] = viewport.New(80, 30)
	}

	return Model{
		cfg:       cfg,
		loading:   make(map[string]bool),
		errors:    make(map[string]string),
		spinner:   sp,
		viewports: vps,
		activeTab: TabOverview,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		doRefreshAll(m.cfg),
		tickEvery(time.Duration(m.cfg.RefreshSec)*time.Second),
		loadCachedBrief(m.cfg),
	)
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func doRefreshAll(cfg *config.Config) tea.Cmd {
	return tea.Batch(
		fetchGlobalNews(),
		fetchLocalNews(cfg.Location.City, cfg.Location.Country),
		fetchCrypto(cfg.CryptoPairs),
		fetchStocks(),
		fetchCommodities(),
		fetchPolymarket(),
		fetchWeather(cfg.Location.Latitude, cfg.Location.Longitude, cfg.Location.City),
	)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentH := m.height - 6
		for i := range m.viewports {
			m.viewports[i].Width = msg.Width - 4
			m.viewports[i].Height = contentH
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % tabCount
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
		case "1":
			m.activeTab = TabOverview
		case "2":
			m.activeTab = TabNews
		case "3":
			m.activeTab = TabLocal
		case "r":
			m.lastRefresh = time.Time{}
			cmds = append(cmds, doRefreshAll(m.cfg))
		case "b":
			if m.cfg.GroqAPIKey != "" {
				m.loading["brief"] = true
				cmds = append(cmds, fetchBrief(m.cfg.GroqAPIKey, m.globalNews, m.cfg.BriefCacheMins, false))
			}
		case "B":
			if m.cfg.GroqAPIKey != "" {
				m.loading["brief"] = true
				m.statusMsg = "Forcing fresh brief (ignoring cache)..."
				m.statusExpiry = time.Now().Add(3 * time.Second)
				cmds = append(cmds, fetchBrief(m.cfg.GroqAPIKey, m.globalNews, m.cfg.BriefCacheMins, true))
			}
		case "j", "down":
			if m.activeTab == TabNews && len(m.globalNews) > 0 {
				m.selectedNewsIdx = minInt(m.selectedNewsIdx+1, len(m.globalNews)-1)
				{
					newsContent, hdrLines := m.renderNewsContent()
					m.newsHeaderLines = hdrLines
					m.viewports[TabNews].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabNews], m.newsHeaderLines, m.selectedNewsIdx)
				// Force a redraw by sending a WindowSizeMsg
				cmds = append(cmds, func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				})
			} else {
				m.viewports[m.activeTab].LineDown(1)
			}
		case "k", "up":
			if m.activeTab == TabNews && len(m.globalNews) > 0 {
				m.selectedNewsIdx = maxInt(m.selectedNewsIdx-1, 0)
				{
					newsContent, hdrLines := m.renderNewsContent()
					m.newsHeaderLines = hdrLines
					m.viewports[TabNews].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabNews], m.newsHeaderLines, m.selectedNewsIdx)
				// Force a redraw by sending a WindowSizeMsg
				cmds = append(cmds, func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				})
			} else {
				m.viewports[m.activeTab].LineUp(1)
			}
		case "enter":
			if m.activeTab == TabNews && m.selectedNewsIdx < len(m.globalNews) {
				item := m.globalNews[m.selectedNewsIdx]
				if item.URL != "" {
					cmds = append(cmds, openURL(item.URL))
					m.statusMsg = "Opening: " + truncate(item.Title, 60)
					m.statusExpiry = time.Now().Add(3 * time.Second)
				} else {
					m.statusMsg = "No URL available for this article"
					m.statusExpiry = time.Now().Add(3 * time.Second)
				}
			} else if m.activeTab == TabLocal && m.selectedLocalNewsIdx < len(m.localNews) {
				item := m.localNews[m.selectedLocalNewsIdx]
				if item.URL != "" {
					cmds = append(cmds, openURL(item.URL))
					m.statusMsg = "Opening: " + truncate(item.Title, 60)
					m.statusExpiry = time.Now().Add(3 * time.Second)
				} else {
					m.statusMsg = "No URL available for this article"
					m.statusExpiry = time.Now().Add(3 * time.Second)
				}
			}
		case "d":
			switch m.activeTab {
			case TabNews:
				m.selectedNewsIdx = minInt(m.selectedNewsIdx+10, maxInt(len(m.globalNews)-1, 0))
				{
					newsContent, hdrLines := m.renderNewsContent()
					m.newsHeaderLines = hdrLines
					m.viewports[TabNews].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabNews], m.newsHeaderLines, m.selectedNewsIdx)
				// Force a redraw by sending a WindowSizeMsg
				cmds = append(cmds, func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				})
			case TabLocal:
				m.selectedLocalNewsIdx = minInt(m.selectedLocalNewsIdx+10, maxInt(len(m.localNews)-1, 0))
				{
					newsContent, hdrLines := m.renderLocalContent()
					m.localNewsHeaderLines = hdrLines
					m.viewports[TabLocal].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabLocal], m.localNewsHeaderLines, m.selectedLocalNewsIdx)
			default:
				m.viewports[m.activeTab].HalfViewDown()
			}
		case "u":
			switch m.activeTab {
			case TabNews:
				m.selectedNewsIdx = maxInt(m.selectedNewsIdx-10, 0)
				{
					newsContent, hdrLines := m.renderNewsContent()
					m.newsHeaderLines = hdrLines
					m.viewports[TabNews].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabNews], m.newsHeaderLines, m.selectedNewsIdx)
				// Force a redraw by sending a WindowSizeMsg
				cmds = append(cmds, func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				})
			case TabLocal:
				m.selectedLocalNewsIdx = maxInt(m.selectedLocalNewsIdx-10, 0)
				{
					newsContent, hdrLines := m.renderLocalContent()
					m.localNewsHeaderLines = hdrLines
					m.viewports[TabLocal].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabLocal], m.localNewsHeaderLines, m.selectedLocalNewsIdx)
			default:
				m.viewports[m.activeTab].HalfViewUp()
			}
		case "G":
			switch m.activeTab {
			case TabNews:
				m.selectedNewsIdx = maxInt(len(m.globalNews)-1, 0)
				{
					newsContent, hdrLines := m.renderNewsContent()
					m.newsHeaderLines = hdrLines
					m.viewports[TabNews].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabNews], m.newsHeaderLines, m.selectedNewsIdx)
				// Force a redraw by sending a WindowSizeMsg
				cmds = append(cmds, func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				})
			case TabLocal:
				m.selectedLocalNewsIdx = maxInt(len(m.localNews)-1, 0)
				{
					newsContent, hdrLines := m.renderLocalContent()
					m.localNewsHeaderLines = hdrLines
					m.viewports[TabLocal].SetContent(newsContent)
				}
				scrollNewsIntoView(&m.viewports[TabLocal], m.localNewsHeaderLines, m.selectedLocalNewsIdx)
			default:
				m.viewports[m.activeTab].GotoBottom()
			}
		case "g":
			if m.activeTab == TabNews {
				m.selectedNewsIdx = 0
				newsContent, hdrLines := m.renderNewsContent()
				m.newsHeaderLines = hdrLines
				m.viewports[TabNews].SetContent(newsContent)
				m.viewports[TabNews].GotoTop()

				// Force a redraw by sending a WindowSizeMsg
				cmds = append(cmds, func() tea.Msg {
					return tea.WindowSizeMsg{
						Width:  m.width,
						Height: m.height,
					}
				})
			} else {
				m.viewports[m.activeTab].GotoTop()
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		// Keep overview spinner animated while loading
		if len(m.loading) > 0 {
			m.viewports[TabOverview].SetContent(m.renderOverviewContent())
		}

	case tickMsg:
		m.lastRefresh = time.Time{}
		cmds = append(cmds,
			doRefreshAll(m.cfg),
			tickEvery(time.Duration(m.cfg.RefreshSec)*time.Second),
		)

	case globalNewsMsg:
		delete(m.loading, "global")
		if msg.err != nil {
			m.errors["global"] = msg.err.Error()
		} else {
			m.globalNews = msg.items
			delete(m.errors, "global")
			if m.cfg.GroqAPIKey != "" && m.brief == nil {
				m.loading["brief"] = true
				cmds = append(cmds, fetchBrief(m.cfg.GroqAPIKey, m.globalNews, m.cfg.BriefCacheMins, false))
			}
		}
		{
			newsContent, hdrLines := m.renderNewsContent()
			m.newsHeaderLines = hdrLines
			m.viewports[TabNews].SetContent(newsContent)
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())

	case localNewsMsg:
		delete(m.loading, "local")
		if msg.err != nil {
			m.errors["local"] = msg.err.Error()
		} else {
			m.localNews = msg.items
			delete(m.errors, "local")
		}
		{
			content, hdrLines := m.renderLocalContent()
			m.localNewsHeaderLines = hdrLines
			m.viewports[TabLocal].SetContent(content)
		}

	case cryptoMsg:
		delete(m.loading, "crypto")
		if msg.err != nil {
			m.errors["crypto"] = msg.err.Error()
		} else {
			m.cryptoPrices = msg.prices
			delete(m.errors, "crypto")
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())

	case stockMsg:
		delete(m.loading, "stocks")
		if msg.err != nil {
			m.errors["stocks"] = msg.err.Error()
		} else {
			m.stockIndices = msg.indices
			delete(m.errors, "stocks")
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())

	case commodityMsg:
		delete(m.loading, "commodities")
		if msg.err != nil {
			m.errors["commodities"] = msg.err.Error()
		} else {
			m.commodities = msg.commodities
			delete(m.errors, "commodities")
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())

	case polymarketMsg:
		delete(m.loading, "poly")
		if msg.err != nil {
			m.errors["poly"] = msg.err.Error()
		} else {
			m.polyMarkets = msg.markets
			delete(m.errors, "poly")
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())

	case weatherMsg:
		delete(m.loading, "weather")
		if msg.err != nil {
			m.errors["weather"] = msg.err.Error()
		} else {
			m.weatherCond = msg.cond
			m.forecast = msg.forecast
			delete(m.errors, "weather")
		}
		{
			content, hdrLines := m.renderLocalContent()
			m.localNewsHeaderLines = hdrLines
			m.viewports[TabLocal].SetContent(content)
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())

	case briefMsg:
		delete(m.loading, "brief")
		if msg.err != nil {
			m.errors["brief"] = msg.err.Error()
		} else {
			m.brief = msg.brief
			delete(m.errors, "brief")
			m.lastRefresh = time.Now()
			if msg.fromCache {
				m.statusMsg = "Brief loaded from cache (" + msg.brief.GeneratedAt.Format("Jan 02 15:04") + ")"
			} else {
				// Persist fresh result to disk cache
				go intel.SaveCachedBrief(msg.brief)
				m.statusMsg = "Brief generated and cached"
			}
			m.statusExpiry = time.Now().Add(4 * time.Second)
		}
		m.viewports[TabOverview].SetContent(m.renderOverviewContent())
		// Re-render news pane too so country risk header updates
		{
			newsContent, hdrLines := m.renderNewsContent()
			m.newsHeaderLines = hdrLines
			m.viewports[TabNews].SetContent(newsContent)
		}

	case openURLMsg:
		// No-op — the Cmd already ran xdg-open/open; nothing to update
		_ = msg

	case clearStatusMsg:
		if time.Now().After(m.statusExpiry) {
			m.statusMsg = ""
		}
	}

	return m, tea.Batch(cmds...)
}

// ─── View ─────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing Watchtower..."
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		m.renderTabs(),
		m.renderActivePane(),
		m.renderFooter(),
	)
}

func (m Model) renderHeader() string {
	isLoading := len(m.loading) > 0
	loadStr := ""
	if isLoading {
		loadStr = "  " + m.spinner.View() + " loading..."
	}
	refreshStr := ""
	if !m.lastRefresh.IsZero() {
		refreshStr = fmt.Sprintf("  updated %s", m.lastRefresh.Format("15:04:05"))
	}
	title := StyleTitle.Render("🌍 WATCHTOWER")
	right := StyleSubtitle.Render("real-time intelligence" + loadStr + refreshStr)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(right) - 4
	if gap < 1 {
		gap = 1
	}
	return StyleHeader.Width(m.width).Render(
		title + strings.Repeat(" ", gap) + right,
	)
}

func (m Model) renderTabs() string {
	names := []string{"1 Overview", "2 Global News", "3 Local"}
	var parts []string
	for i, name := range names {
		if i == m.activeTab {
			parts = append(parts, StyleActiveTab.Render("[ "+name+" ]"))
		} else {
			parts = append(parts, StyleInactiveTab.Render("  "+name+"  "))
		}
	}
	return StyleTabBar.Width(m.width).Render(strings.Join(parts, " "))
}

func (m Model) renderActivePane() string {
	contentH := m.height - 6
	if contentH < 5 {
		contentH = 5
	}
	return StylePane.Width(m.width - 2).Height(contentH).Render(
		m.viewports[m.activeTab].View(),
	)
}

func (m Model) renderFooter() string {
	// Show status message if active (e.g. "Opening article...")
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		return StyleFooterStatus.Width(m.width).Render("  ✓ " + m.statusMsg)
	}
	var hint string
	switch m.activeTab {
	case TabNews:
		hint = "  jk navigate  enter open in browser  d/u page  g/G top/bottom  tab switch  r refresh  b brief  q quit"
	case TabLocal:
		hint = "  jk navigate  enter open in browser  d/u page  g/G top/bottom  tab switch  r refresh  q quit"
	default:
		hint = "  ↑↓/jk scroll  tab/←→ switch  1 overview  2 news  3 local  r refresh  b brief  q quit"
	}
	return StyleFooter.Width(m.width).Render(hint)
}

// ─── Overview: 2×2 grid ───────────────────────────────────────────────────────

func (m Model) renderOverviewContent() string {
	if m.width == 0 {
		return ""
	}

	// Available inner width (minus outer pane border+padding = 4)
	innerW := m.width - 4
	halfW := innerW / 2

	// Available inner height
	contentH := m.height - 6
	if contentH < 10 {
		contentH = 10
	}
	// Top row gets ~40% of height, bottom row gets ~55% (crypto panel needs more space)
	topH := (contentH * 4) / 10
	botH := contentH - topH - 3 // 3 = gap line + borders
	if topH < 8 {
		topH = 8
	}
	if botH < 8 {
		botH = 8
	}
	topQH := topH - 2
	botQH := botH - 2

	// Quadrant inner content width
	qW := halfW - 3

	// Render the four panels
	topLeft := m.quadrantBox("🌤  WEATHER  "+m.cfg.Location.City, m.renderWeatherPanel(qW, topQH), halfW-1, topH)
	topRight := m.quadrantBox("🧠  INTEL BRIEF", m.renderBriefPanel(qW, topQH), halfW-1, topH)
	botLeft := m.quadrantBox("₿  MARKETS & PRICES", m.renderCryptoPanel(qW, botQH), halfW-1, botH)
	botRight := m.quadrantBox("📊  PREDICTION MARKETS", m.renderPolyPanel(qW, botQH), halfW-1, botH)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, topLeft, " ", topRight)
	botRow := lipgloss.JoinHorizontal(lipgloss.Top, botLeft, " ", botRight)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, "", botRow)
}

// quadrantBox wraps content in a rounded border with a colored title bar
func (m Model) quadrantBox(title, content string, w, h int) string {
	titleLine := StyleQuadrantTitle.Width(w).Render(title)
	body := StyleQuadrantPane.Width(w).Height(h).Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, titleLine, body)
}

// ─── Quadrant content renderers ───────────────────────────────────────────────

func (m Model) renderWeatherPanel(w, h int) string {
	var sb strings.Builder

	if errMsg, ok := m.errors["weather"]; ok {
		return StyleError.Render("⚠ " + errMsg)
	}
	if m.weatherCond == nil {
		return m.spinner.View() + " fetching weather..."
	}

	wc := m.weatherCond
	// Large icon + temp on first line
	sb.WriteString(fmt.Sprintf("%s  %s\n", wc.Icon,
		StyleWeatherTemp.Render(fmt.Sprintf("%.1f°C", wc.TempC))))
	sb.WriteString(StyleWeatherDesc.Render(wc.Description) + "\n")
	sb.WriteString(StyleAge.Render(fmt.Sprintf("Feels like %.1f°C", wc.FeelsLikeC)) + "\n\n")
	sb.WriteString(fmt.Sprintf("💧 %d%%   💨 %.0f km/h %s   ☀ UV %.0f\n",
		wc.Humidity, wc.WindSpeedKmh,
		weather.WindDirectionStr(wc.WindDirection), wc.UVIndex))

	// Compact forecast — as many rows as fit
	if len(m.forecast) > 0 {
		sb.WriteString("\n")
		sb.WriteString(StyleTableHeader.Render(
			fmt.Sprintf("%-10s  %-4s %5s %5s %5s", "Day", "", "Hi", "Lo", "Rain")) + "\n")
		sb.WriteString(StyleDivider.Render(strings.Repeat("─", minInt(w, 36))) + "\n")
		maxRows := h - 8
		if maxRows < 1 {
			maxRows = 1
		}
		for i, f := range m.forecast {
			if i >= maxRows {
				break
			}
			dayLabel := f.Date.Format("Mon 02 Jan")
			if i == 0 {
				dayLabel = "Today     "
			}
			sb.WriteString(fmt.Sprintf("%-10s  %s  %4.0f° %4.0f° %3.0fmm\n",
				dayLabel, f.Icon, f.MaxTempC, f.MinTempC, f.RainMM))
		}
	}

	return sb.String()
}

func (m Model) renderBriefPanel(w, h int) string {
	var sb strings.Builder

	if m.cfg.GroqAPIKey == "" {
		sb.WriteString(StyleWarning.Render("⚠  No GROQ_API_KEY set.\n\n"))
		sb.WriteString(StyleMuted.Render("Add key to:\n~/.config/watchtower/config.yaml\n\nGet free key:\nconsole.groq.com\n\nPress [b] after adding key."))
		return sb.String()
	}

	if m.loading["brief"] {
		sb.WriteString(m.spinner.View() + " Generating brief...\n\n")
		sb.WriteString(StyleMuted.Render("Calling Groq / Llama 3.1..."))
		return sb.String()
	}

	if errMsg, ok := m.errors["brief"]; ok {
		sb.WriteString(StyleError.Render("⚠ "+errMsg) + "\n\n")
		sb.WriteString(StyleMuted.Render("Press [b] to retry."))
		return sb.String()
	}

	if m.brief == nil {
		if len(m.globalNews) == 0 {
			sb.WriteString(StyleMuted.Render("Waiting for news to load..."))
		} else {
			sb.WriteString(StyleMuted.Render("Press [b] to generate AI brief."))
		}
		return sb.String()
	}

	b := m.brief
	cacheAge := ""
	if time.Since(b.GeneratedAt) > time.Minute {
		mins := int(time.Since(b.GeneratedAt).Minutes())
		if mins >= 60 {
			cacheAge = fmt.Sprintf("  cached %dh%dm ago", mins/60, mins%60)
		} else {
			cacheAge = fmt.Sprintf("  cached %dm ago", mins)
		}
	}
	sb.WriteString(StyleBriefMeta.Render(b.GeneratedAt.Format("15:04")+"  "+b.Model+cacheAge) + "\n\n")

	// Word-wrapped summary
	wrapped := wordWrap(b.Summary, w-2)
	for _, line := range strings.Split(wrapped, "\n") {
		sb.WriteString(line + "\n")
	}

	if len(b.KeyThreats) > 0 {
		sb.WriteString("\n" + StyleBriefTitle.Render("KEY THREATS") + "\n")
		for _, t := range b.KeyThreats {
			// Word-wrap each threat to panel width rather than truncating
			wrapped := wordWrap("● "+t, w-2)
			for i, line := range strings.Split(wrapped, "\n") {
				if i == 0 {
					sb.WriteString(StyleThreatItem.Render(line) + "\n")
				} else {
					// Indent continuation lines to align past the bullet
					sb.WriteString(StyleThreatItem.Render("  "+line) + "\n")
				}
			}
		}
	}

	return sb.String()
}

func (m Model) renderCryptoPanel(w, h int) string {
	var sb strings.Builder

	// ── Crypto ────────────────────────────────────────────────────────────────
	if errMsg, ok := m.errors["crypto"]; ok {
		sb.WriteString(StyleError.Render("⚠ crypto: "+errMsg) + "\n")
	} else if len(m.cryptoPrices) == 0 {
		sb.WriteString(m.spinner.View() + " fetching crypto...\n")
	} else {
		symW := 5
		priceW := 11
		changeW := 8
		nameW := w - symW - priceW - changeW - 4
		if nameW < 4 {
			nameW = 4
		}
		hdr := fmt.Sprintf("%-*s %-*s %*s %*s",
			symW, "SYM", nameW, "NAME", priceW, "PRICE", changeW, "24H%")
		sb.WriteString(StyleTableHeader.Render(hdr) + "\n")
		sb.WriteString(StyleDivider.Render(strings.Repeat("─", minInt(w-1, 55))) + "\n")
		for _, p := range m.cryptoPrices {
			chStyle := StylePositive
			chIcon := "▲"
			if p.Change24h < 0 {
				chStyle = StyleNegative
				chIcon = "▼"
			}
			name := p.Name
			if len(name) > nameW {
				name = name[:nameW-1] + "…"
			}
			row := fmt.Sprintf("%-*s %-*s %*s ",
				symW, StyleSymbol.Render(p.Symbol),
				nameW, name,
				priceW, markets.FormatPrice(p.PriceUSD),
			)
			sb.WriteString(row + chStyle.Render(fmt.Sprintf("%s%5.2f%%", chIcon, p.Change24h)) + "\n")
		}
	}

	sb.WriteString("\n")

	// ── Stock Indices ─────────────────────────────────────────────────────────
	sb.WriteString(StyleSubSectionHeader.Render(" INDICES") + "\n")
	if errMsg, ok := m.errors["stocks"]; ok {
		sb.WriteString(StyleError.Render("⚠ "+errMsg) + "\n")
	} else if len(m.stockIndices) == 0 {
		sb.WriteString(StyleMuted.Render("  "+m.spinner.View()+" fetching...") + "\n")
	} else {
		nameW := w - 14
		if nameW < 6 {
			nameW = 6
		}
		for _, idx := range m.stockIndices {
			chStyle := StylePositive
			chIcon := "▲"
			if idx.ChangePct < 0 {
				chStyle = StyleNegative
				chIcon = "▼"
			}
			name := idx.Name
			if len(name) > nameW {
				name = name[:nameW-1] + "…"
			}
			sb.WriteString(fmt.Sprintf("%-*s %11s %s\n",
				nameW, name,
				markets.FormatPrice(idx.Price),
				chStyle.Render(fmt.Sprintf("%s%5.2f%%", chIcon, idx.ChangePct)),
			))
		}
	}

	sb.WriteString("\n")

	// ── Commodities ───────────────────────────────────────────────────────────
	sb.WriteString(StyleSubSectionHeader.Render(" COMMODITIES") + "\n")
	if errMsg, ok := m.errors["commodities"]; ok {
		sb.WriteString(StyleError.Render("⚠ "+errMsg) + "\n")
	} else if len(m.commodities) == 0 {
		sb.WriteString(StyleMuted.Render("  "+m.spinner.View()+" fetching...") + "\n")
	} else {
		nameW := w - 22
		if nameW < 6 {
			nameW = 6
		}
		for _, c := range m.commodities {
			chStyle := StylePositive
			chIcon := "▲"
			if c.ChangePct < 0 {
				chStyle = StyleNegative
				chIcon = "▼"
			}
			name := c.Name
			if len(name) > nameW {
				name = name[:nameW-1] + "…"
			}
			unitStr := StyleMuted.Render(fmt.Sprintf("%-6s", c.Unit))
			sb.WriteString(fmt.Sprintf("%-*s %9s %s %s\n",
				nameW, name,
				markets.FormatPrice(c.Price),
				unitStr,
				chStyle.Render(fmt.Sprintf("%s%5.2f%%", chIcon, c.ChangePct)),
			))
		}
	}

	return StyleQuadrantPane.Width(w).Height(h).Render(sb.String())
}

func (m Model) renderPolyPanel(w, h int) string {
	var sb strings.Builder

	if errMsg, ok := m.errors["poly"]; ok {
		return StyleError.Render("⚠ " + errMsg)
	}
	if len(m.polyMarkets) == 0 {
		return m.spinner.View() + " fetching markets..."
	}

	// Title column gets most space; reserve room for pct (7) + ends (6) + spacing (3)
	titleW := w - 16
	if titleW < 10 {
		titleW = 10
	}

	hdr := fmt.Sprintf("%-*s %6s  %5s", titleW, "QUESTION", "YES%", "ENDS")
	sb.WriteString(StyleTableHeader.Render(hdr) + "\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", minInt(w-1, 70))) + "\n")

	maxRows := h - 2
	if maxRows < 1 {
		maxRows = 1
	}
	for i, pm := range m.polyMarkets {
		if i >= maxRows {
			break
		}
		pct := pm.Probability * 100
		pctStyle := StyleNeutral
		switch {
		case pct >= 66:
			pctStyle = StylePositive
		case pct <= 33:
			pctStyle = StyleNegative
		}
		title := pm.Title
		if len(title) > titleW {
			title = title[:titleW-3] + "..."
		}
		endDate := pm.EndDate
		if len(endDate) >= 10 {
			endDate = endDate[5:10] // MM-DD
		}
		sb.WriteString(fmt.Sprintf("%-*s %s  %5s\n",
			titleW, title,
			pctStyle.Render(fmt.Sprintf("%5.1f%%", pct)),
			endDate,
		))
	}

	return sb.String()
}

// ─── Full-screen pane renderers ───────────────────────────────────────────────

// renderNewsContent renders the full news tab content and returns (content, headerLineCount).
func (m Model) renderNewsContent() (string, int) {
	var sb strings.Builder

	if errMsg, ok := m.errors["global"]; ok {
		sb.WriteString(StyleError.Render("⚠ Error: "+errMsg) + "\n\n")
	}
	if len(m.globalNews) == 0 {
		if m.loading["global"] {
			return "  " + m.spinner.View() + " Fetching global news...", 0
		}
		return "  No news loaded. Press r to refresh.", 0
	}

	// ── Top header: country risk panel spanning full width ────────────────
	innerW := m.width - 6 // account for pane borders/padding

	header, countryRiskLines := m.renderCountryRiskPanel(innerW)
	divider := StyleDivider.Render(strings.Repeat("─", innerW))
	sectionHdr := StyleSectionHeader.Render(
		fmt.Sprintf(" ARTICLES  (%d)  ·  j/k navigate  ·  enter to open in browser", len(m.globalNews)))

	// Header lines = country risk panel lines + divider + section header + blank lines
	// header + "\n" + divider + "\n\n" + sectionHdr + "\n\n"
	// = countryRiskLines + 1 + 3 + 3 = countryRiskLines + 7
	hdrLines := countryRiskLines + 7

	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(divider + "\n\n")
	sb.WriteString(sectionHdr + "\n\n")

	// Calculate available width for title line
	// Badge (~9) + source (~15) + age (~8) + urlIndicator (~3) + separators (~4) = ~39
	// Use innerW - 39 as title width, minimum 20 chars
	titleW := innerW - 39
	if titleW < 20 {
		titleW = 20
	}

	for i, item := range m.globalNews {
		if i >= 200 {
			break
		}
		badge := threatStyle(item.ThreatLevel).Render(fmt.Sprintf(" %-8s", item.ThreatLevel.String()))
		source := StyleSource.Render(item.Source)
		age := StyleAge.Render(formatAge(item.Published))

		// Truncate title to fit exactly one line
		titleLine := item.Title
		if len(titleLine) > titleW {
			runes := []rune(titleLine)
			titleLine = string(runes[:titleW-1]) + "…"
		}
		urlIndicator := ""
		if item.URL != "" {
			urlIndicator = StyleMuted.Render("  ↗")
		}

		if i == m.selectedNewsIdx {
			// Highlighted selected row
			titleStyled := StyleSelectedTitle.Render(titleLine)
			line1 := fmt.Sprintf("%s %s  %s%s", badge, source, age, urlIndicator)
			line2 := "  " + titleStyled
			sb.WriteString(StyleSelectedRow.Render(line1) + "\n")
			sb.WriteString(StyleSelectedRow.Render(line2) + "\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("%s %s  %s%s\n  %s\n\n",
				badge, source, age, urlIndicator,
				StyleNewsTitle.Render(titleLine)))
		}
	}
	return sb.String(), hdrLines
}

func (m Model) renderCountryRiskPanel(w int) (string, int) {
	var sb strings.Builder
	sb.WriteString(StyleBriefTitle.Render("🌡  COUNTRY RISK INDEX") + "\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", minInt(w, 120))) + "\n")

	if m.brief == nil || len(m.brief.CountryRisks) == 0 {
		if m.loading["brief"] {
			sb.WriteString("  " + m.spinner.View() + " Computing risks...\n")
		} else {
			sb.WriteString(StyleMuted.Render("  Press [b] to generate risk scores.") + "\n")
		}
		return sb.String(), strings.Count(sb.String(), "\n")
	}

	// Layout constants — all plain character widths, no ANSI in fmt verbs
	const (
		scoreW = 4  // " 82 "
		barW   = 14 // progress bar characters
		gapW   = 2  // spacing between columns
	)
	// nameW: remaining space after score + bar + gaps + leading indent(2)
	nameW := w - scoreW - barW - gapW*3 - 2
	if nameW < 8 {
		nameW = 8
	}
	// reasonW: same as nameW but offset past score col
	reasonW := nameW + scoreW + gapW

	risks := m.brief.CountryRisks

	// Lay countries out in two columns if width allows (>=100 chars)
	cols := 1
	colW := w
	if w >= 100 {
		cols = 2
		colW = w / 2
		// Recalc nameW per column
		nameW = colW - scoreW - barW - gapW*3 - 2
		if nameW < 8 {
			nameW = 8
		}
		reasonW = nameW + scoreW + gapW
	}

	// Build each row as a plain string (styled pieces joined, then padded to colW)
	renderRow := func(cr intel.CountryRisk, colWidth int) string {
		score := cr.Score

		barFilled := (score * barW) / 100
		if barFilled > barW {
			barFilled = barW
		}
		barEmpty := barW - barFilled

		var barStyle lipgloss.Style
		switch {
		case score >= 75:
			barStyle = StyleCritical
		case score >= 50:
			barStyle = StyleHighThreat
		case score >= 25:
			barStyle = StyleMediumThreat
		default:
			barStyle = StyleLowThreat
		}

		// Score badge: plain fixed-width string, then style applied
		scoreStr := StyleBriefMeta.Render(fmt.Sprintf("%3d", score))

		// Bar: styled filled + muted empty — both fixed char count
		bar := barStyle.Render(strings.Repeat("█", barFilled)) +
			StyleMuted.Render(strings.Repeat("░", barEmpty))

		// Country name: truncate to nameW plain chars BEFORE styling
		country := cr.Country
		runes := []rune(country)
		if len(runes) > nameW {
			runes = runes[:nameW-1]
			country = string(runes) + "…"
		} else {
			// Pad with spaces to nameW so columns align
			country = country + strings.Repeat(" ", nameW-len(runes))
		}

		// Reason: truncate to reasonW plain chars
		reason := cr.Reason
		reasonRunes := []rune(reason)
		if len(reasonRunes) > reasonW {
			reasonRunes = reasonRunes[:reasonW-3]
			reason = string(reasonRunes) + "..."
		}

		line1 := "  " + country + "  " + scoreStr + "  " + bar
		line2 := "  " + StyleMuted.Render(reason)
		return line1 + "\n" + line2
	}

	if cols == 1 {
		for _, cr := range risks {
			sb.WriteString(renderRow(cr, colW) + "\n")
		}
	} else {
		// Two columns: pair up rows side by side
		for i := 0; i < len(risks); i += 2 {
			left := renderRow(risks[i], colW)
			leftLines := strings.Split(left, "\n")

			var right []string
			if i+1 < len(risks) {
				r := renderRow(risks[i+1], colW)
				right = strings.Split(r, "\n")
			}

			// Pad left lines to colW visible chars and join with right
			for li, ll := range leftLines {
				// Measure visible width (strip ANSI for measurement)
				visW := lipgloss.Width(ll)
				padding := ""
				if visW < colW {
					padding = strings.Repeat(" ", colW-visW)
				}
				rl := ""
				if li < len(right) {
					rl = right[li]
				}
				sb.WriteString(ll + padding + rl + "\n")
			}
		}
	}

	return sb.String(), strings.Count(sb.String(), "\n")
}

func (m Model) renderLocalContent() (string, int) {
	var sb strings.Builder

	// Build weather section and count its lines
	weatherBlock := ""
	if m.weatherCond != nil {
		wc := m.weatherCond
		weatherBlock += StyleSectionHeader.Render(" WEATHER  "+wc.City) + "\n\n"
		weatherBlock += fmt.Sprintf("  %s  %s  %.1f°C  (feels like %.1f°C)\n",
			wc.Icon, wc.Description, wc.TempC, wc.FeelsLikeC)
		weatherBlock += fmt.Sprintf("  💧 Humidity: %d%%   💨 Wind: %.0f km/h %s   👁 Visibility: %.0f km   ☀ UV: %.0f\n\n",
			wc.Humidity, wc.WindSpeedKmh,
			weather.WindDirectionStr(wc.WindDirection),
			wc.Visibility/1000, wc.UVIndex)
		if len(m.forecast) > 0 {
			weatherBlock += StyleTableHeader.Render(
				fmt.Sprintf("  %-12s %-16s %8s %8s %10s", "DATE", "CONDITION", "MAX", "MIN", "RAIN")) + "\n"
			weatherBlock += StyleDivider.Render(strings.Repeat("─", 60)) + "\n"
			for _, f := range m.forecast {
				weatherBlock += fmt.Sprintf("  %-12s %s %-12s %6.1f°C %6.1f°C %7.1fmm\n",
					f.Date.Format("Mon Jan 02"), f.Icon, f.Desc,
					f.MaxTempC, f.MinTempC, f.RainMM)
			}
			weatherBlock += "\n"
		}
	} else if _, ok := m.errors["weather"]; ok {
		weatherBlock += StyleError.Render("⚠ Weather error") + "\n"
	} else {
		weatherBlock += "  " + m.spinner.View() + " Fetching weather...\n"
	}

	// Count header lines from actual rendered content
	hdrLines := strings.Count(weatherBlock, "\n")

	// Local news header section
	sb.WriteString(weatherBlock)
	sb.WriteString("\n")
	localHdr := StyleSectionHeader.Render(" LOCAL NEWS  " + m.cfg.Location.City)
	sb.WriteString(localHdr + "\n\n")
	hdrLines += strings.Count(localHdr+"\n\n", "\n")

	if errMsg, ok := m.errors["local"]; ok {
		sb.WriteString(StyleError.Render("⚠ "+errMsg) + "\n")
		hdrLines += strings.Count(StyleError.Render("⚠ "+errMsg)+"\n", "\n")
	} else if len(m.localNews) == 0 {
		sb.WriteString("  No local news loaded. Press r to refresh.\n")
		hdrLines += strings.Count("  No local news loaded. Press r to refresh.\n", "\n")
	} else {
		sectionHdr := fmt.Sprintf(" ARTICLES  (%d)  ·  j/k navigate  ·  enter to open in browser", len(m.localNews))
		sb.WriteString(StyleSectionHeader.Render(sectionHdr) + "\n\n")
		hdrLines += strings.Count(StyleSectionHeader.Render(sectionHdr)+"\n\n", "\n")

		for i, item := range m.localNews {
			if i >= 100 {
				break
			}
			badge := threatStyle(item.ThreatLevel).Render(fmt.Sprintf(" %-6s", item.ThreatLevel.String()))
			age := StyleAge.Render(formatAge(item.Published))
			urlIndicator := ""
			if item.URL != "" {
				urlIndicator = StyleMuted.Render("  ↗")
			}

			if i == m.selectedLocalNewsIdx {
				titleLine := item.Title
				line1 := badge + " " + age + urlIndicator
				line2 := "  " + StyleSelectedTitle.Render(titleLine)
				sb.WriteString(StyleSelectedRow.Render(line1) + "\n")
				sb.WriteString(StyleSelectedRow.Render(line2) + "\n\n")
			} else {
				sb.WriteString(fmt.Sprintf("%s %s %s\n  %s\n\n",
					badge, age, urlIndicator,
					StyleNewsTitle.Render(item.Title)))
			}
		}
	}
	return sb.String(), hdrLines
}

// scrollNewsToSelected adjusts the news viewport so the selected article stays visible.
// Each article is exactly 3 lines. Called after selectedNewsIdx or content changes.
// vp is a pointer to m.viewports[TabNews] from the calling Update copy.
func scrollNewsIntoView(vp *viewport.Model, headerLines, selectedIdx int) {
	// When selectedIdx is 0, show the country risk panel at the top
	if selectedIdx == 0 {
		vp.GotoTop() // Use built-in method instead of SetYOffset(0)
		return
	}
	const linesPerItem = 3
	vpH := vp.Height
	if vpH <= 0 {
		return
	}
	itemLine := headerLines + selectedIdx*linesPerItem
	if itemLine < 0 {
		itemLine = 0
	}
	current := vp.YOffset

	// Item is above the visible window — scroll up
	if itemLine < current {
		vp.SetYOffset(itemLine)
		return
	}
	// Item bottom is below the visible window — scroll down
	itemBottom := itemLine + linesPerItem - 1
	if itemBottom >= current+vpH {
		vp.SetYOffset(itemBottom - vpH + 1)
	}
	// Otherwise already visible — no change
}

// ─── Tea commands ─────────────────────────────────────────────────────────────

func fetchGlobalNews() tea.Cmd {
	return func() tea.Msg {
		items, err := feeds.FetchGlobalNews(context.Background())
		return globalNewsMsg{items, err}
	}
}

func fetchLocalNews(city, country string) tea.Cmd {
	return func() tea.Msg {
		items, err := feeds.FetchLocalNews(context.Background(), city, country)
		return localNewsMsg{items, err}
	}
}

func fetchCrypto(pairs []string) tea.Cmd {
	return func() tea.Msg {
		prices, err := markets.FetchCryptoPrices(context.Background(), pairs)
		return cryptoMsg{prices, err}
	}
}

func fetchStocks() tea.Cmd {
	return func() tea.Msg {
		indices, err := markets.FetchStockIndices(context.Background())
		return stockMsg{indices, err}
	}
}

func fetchCommodities() tea.Cmd {
	return func() tea.Msg {
		commodities, err := markets.FetchCommodities(context.Background())
		return commodityMsg{commodities, err}
	}
}

func fetchPolymarket() tea.Cmd {
	return func() tea.Msg {
		mkts, err := markets.FetchPredictionMarkets(context.Background())
		return polymarketMsg{mkts, err}
	}
}

func fetchWeather(lat, lon float64, city string) tea.Cmd {
	return func() tea.Msg {
		cond, forecast, err := weather.Fetch(context.Background(), lat, lon, city)
		return weatherMsg{cond, forecast, err}
	}
}

// fetchBrief generates a brief, using the disk cache unless forceRefresh is true.
// cacheMins=0 means always generate fresh (cache disabled).
func fetchBrief(apiKey string, items []feeds.NewsItem, cacheMins int, forceRefresh bool) tea.Cmd {
	return func() tea.Msg {
		// Try cache first (unless forced refresh or cache disabled)
		if !forceRefresh && cacheMins > 0 {
			maxAge := time.Duration(cacheMins) * time.Minute
			cached, err := intel.LoadCachedBrief(maxAge)
			if err == nil && cached != nil {
				return briefMsg{brief: cached, fromCache: true}
			}
		}
		// Cache miss or disabled — call Groq
		b, err := intel.GenerateBrief(context.Background(), apiKey, items)
		return briefMsg{brief: b, err: err, fromCache: false}
	}
}

// loadCachedBrief is fired on Init to immediately populate the brief from
// disk if a valid cache exists, before any news has loaded.
func loadCachedBrief(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if cfg.BriefCacheMins == 0 {
			return nil
		}
		maxAge := time.Duration(cfg.BriefCacheMins) * time.Minute
		cached, err := intel.LoadCachedBrief(maxAge)
		if err != nil || cached == nil {
			return nil
		}
		return briefMsg{brief: cached, fromCache: true}
	}
}

// ─── Tea commands (continued) ────────────────────────────────────────────────

// openURL opens a URL in the system default browser (cross-platform)
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd string
		var args []string
		// Detect OS: try xdg-open (Linux), open (macOS), start (Windows)
		// We use a simple exec approach — errors are silently ignored so
		// the TUI never crashes if a browser isn't available.
		for _, candidate := range []string{"xdg-open", "open", "start"} {
			if isCommandAvailable(candidate) {
				cmd = candidate
				args = []string{url}
				break
			}
		}
		if cmd != "" {
			execCommand(cmd, args...)
		}
		return openURLMsg{url: url}
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("Jan 02")
	}
}

func probabilityBar(p float64, width int) string {
	filled := int(p * float64(width))
	empty := width - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	switch {
	case p >= 0.66:
		return StylePositive.Render(bar)
	case p <= 0.33:
		return StyleNegative.Render(bar)
	default:
		return StyleNeutral.Render(bar)
	}
}

func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	words := strings.Fields(s)
	var lines []string
	var line strings.Builder
	for _, w := range words {
		if line.Len()+len(w)+1 > width {
			lines = append(lines, line.String())
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(w)
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

func threatStyle(level feeds.ThreatLevel) lipgloss.Style {
	switch level {
	case feeds.ThreatCritical:
		return StyleCritical
	case feeds.ThreatHigh:
		return StyleHighThreat
	case feeds.ThreatMedium:
		return StyleMediumThreat
	case feeds.ThreatLow:
		return StyleLowThreat
	default:
		return StyleInfoThreat
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// isCommandAvailable checks if a command exists on PATH
func isCommandAvailable(name string) bool {
	_, err := execLookPath(name)
	return err == nil
}

// execLookPath wraps exec.LookPath
func execLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// execCommand runs a command in the background, ignoring errors
func execCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	// Detach from the TUI process so it doesn't block
	_ = cmd.Start()
}
