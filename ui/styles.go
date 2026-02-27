package ui

import "github.com/charmbracelet/lipgloss"

// Color palette — dark terminal friendly
var (
	colorBg       = lipgloss.Color("#0d1117")
	colorBorder   = lipgloss.Color("#30363d")
	colorAccent   = lipgloss.Color("#58a6ff")
	colorGold     = lipgloss.Color("#d29922")
	colorGreen    = lipgloss.Color("#3fb950")
	colorRed      = lipgloss.Color("#f85149")
	colorOrange   = lipgloss.Color("#db6d28")
	colorYellow   = lipgloss.Color("#e3b341")
	colorMuted    = lipgloss.Color("#8b949e")
	colorWhite    = lipgloss.Color("#e6edf3")
	colorPurple   = lipgloss.Color("#bc8cff")
	colorTeal     = lipgloss.Color("#39d353")

	// Backgrounds for badges
	bgCritical = lipgloss.Color("#b91c1c")
	bgHigh     = lipgloss.Color("#92400e")
	bgMedium   = lipgloss.Color("#1e3a5f")
	bgLow      = lipgloss.Color("#1a3622")
	bgInfo     = lipgloss.Color("#1c2128")
)

var (
	// Layout
	StyleHeader = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorWhite).
			Padding(0, 1)

	StyleTitle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleTabBar = lipgloss.NewStyle().
			Background(colorBg).
			Padding(0, 1)

	StyleActiveTab = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorAccent)

	StyleInactiveTab = lipgloss.NewStyle().
				Foreground(colorMuted)

	StylePane = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	StyleFooter = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(colorBg).
			Padding(0, 1)

	// Content styles
	StyleSectionHeader = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true).
				Background(lipgloss.Color("#161b22")).
				Padding(0, 2)

	StyleTableHeader = lipgloss.NewStyle().
				Foreground(colorMuted).
				Bold(true)

	StyleDivider = lipgloss.NewStyle().
			Foreground(colorBorder)

	StyleNewsTitle = lipgloss.NewStyle().
			Foreground(colorWhite)

	StyleSource = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	StyleAge = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleSymbol = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true)

	StyleMktCap = lipgloss.NewStyle().
			Foreground(colorMuted)

	StylePositive = lipgloss.NewStyle().
			Foreground(colorGreen)

	StyleNegative = lipgloss.NewStyle().
			Foreground(colorRed)

	StyleNeutral = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleError = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	StyleWarning = lipgloss.NewStyle().
			Foreground(colorYellow)

	StyleSpinner = lipgloss.NewStyle().
			Foreground(colorAccent)

	// Threat level badges
	StyleCritical = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(bgCritical).
			Bold(true)

	StyleHighThreat = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(bgHigh).
			Bold(true)

	StyleMediumThreat = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(bgMedium)

	StyleLowThreat = lipgloss.NewStyle().
			Foreground(colorGreen).
			Background(bgLow)

	StyleInfoThreat = lipgloss.NewStyle().
			Foreground(colorMuted).
			Background(bgInfo)

	// Intel Brief
	StyleBriefTitle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true)

	StyleBriefMeta = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	StyleThreatItem = lipgloss.NewStyle().
			Foreground(colorOrange)

	StyleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Overview quadrant boxes
	StyleQuadrantTitle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Background(lipgloss.Color("#161b22")).
				Bold(true).
				Padding(0, 1)

	StyleQuadrantPane = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 1)

	StyleSubSectionHeader = lipgloss.NewStyle().
				Foreground(colorGold).
				Bold(true)

	// Weather-specific
	StyleWeatherTemp = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true).
				// Large-ish — terminal bold is the best we can do without sixels
				Underline(false)

	StyleWeatherDesc = lipgloss.NewStyle().
				Foreground(colorAccent)
)
