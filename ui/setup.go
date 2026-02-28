package ui

import (
	"context"
	"fmt"
	"watchtower/config"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	stepSelectProvider = iota
	stepAPIKey
	stepLocation
	stepSaving
	stepDone
)

var providers = []string{"groq", "openai", "deepseek", "gemini", "claude", "local"}

type SetupModel struct {
	step        int
	selectedIdx int

	apiKeyInput  textinput.Model
	cityInput    textinput.Model
	countryInput textinput.Model

	spinner   spinner.Model
	geocoding bool
	saving    bool
	err       string

	width  int
	height int
}

func NewSetupModel() SetupModel {
	apiKeyInput := textinput.New()
	apiKeyInput.Placeholder = "Paste your API key here"
	apiKeyInput.EchoMode = textinput.EchoPassword
	apiKeyInput.EchoCharacter = '*'
	apiKeyInput.Focus()

	cityInput := textinput.New()
	cityInput.Placeholder = "e.g., Lisbon"
	cityInput.Focus()

	countryInput := textinput.New()
	countryInput.Placeholder = "e.g., PT"
	countryInput.CharLimit = 2

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleSpinner

	return SetupModel{
		step:         stepSelectProvider,
		selectedIdx:  0,
		apiKeyInput:  apiKeyInput,
		cityInput:    cityInput,
		countryInput: countryInput,
		spinner:      sp,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return func() tea.Msg {
		return spinner.TickMsg{}
	}
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.apiKeyInput.Width = minInt(50, msg.Width-20)
		m.cityInput.Width = minInt(30, msg.Width-20)
		m.countryInput.Width = 4

	case tea.KeyMsg:
		switch m.step {
		case stepSelectProvider:
			switch msg.Type {
			case tea.KeyUp, tea.KeyShiftTab:
				m.selectedIdx = (m.selectedIdx - 1 + len(providers)) % len(providers)
			case tea.KeyDown, tea.KeyTab:
				m.selectedIdx = (m.selectedIdx + 1) % len(providers)
			case tea.KeyEnter:
				m.step = stepAPIKey
			}

		case stepAPIKey:
			switch msg.Type {
			case tea.KeyEnter:
				if m.apiKeyInput.Value() != "" {
					m.step = stepLocation
				}
			default:
				var cmd tea.Cmd
				m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
				cmds = append(cmds, cmd)
			}

		case stepLocation:
			switch msg.Type {
			case tea.KeyEnter:
				if m.cityInput.Value() != "" && m.countryInput.Value() != "" {
					m.step = stepSaving
					m.geocoding = true
					cmds = append(cmds, m.doGeocode())
				}
			case tea.KeyTab:
				m.cityInput.Blur()
				m.countryInput.Focus()
			default:
				var cmd1, cmd2 tea.Cmd
				m.cityInput, cmd1 = m.cityInput.Update(msg)
				m.countryInput, cmd2 = m.countryInput.Update(msg)
				cmds = append(cmds, cmd1, cmd2)
			}

		case stepSaving:
			if msg.Type == tea.KeyEnter && m.err != "" {
				m.step = stepLocation
				m.err = ""
			}

		case stepDone:
			return m, tea.Quit
		}

		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case geocodeResultMsg:
		m.geocoding = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.saving = true
			cmds = append(cmds, m.doSave(msg.lat, msg.lon))
		}

	case saveResultMsg:
		m.saving = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.step = stepDone
		}
	}

	return m, tea.Batch(cmds...)
}

func (m SetupModel) View() string {
	if m.width == 0 {
		return "Initializing setup..."
	}

	stepIndicator := StyleStepIndicator.Render(fmt.Sprintf("[%d/4]", m.step+1))
	title := StyleSetupTitle.Render("Watchtower Setup")
	header := lipgloss.JoinHorizontal(lipgloss.Center, stepIndicator, "  ", title)

	var content string
	switch m.step {
	case stepSelectProvider:
		content = m.renderProviderStep()
	case stepAPIKey:
		content = m.renderAPIKeyStep()
	case stepLocation:
		content = m.renderLocationStep()
	case stepSaving:
		content = m.renderSavingStep()
	case stepDone:
		content = m.renderDoneStep()
	}

	footer := StyleMuted.Render("↑↓ select  tab/enter confirm  esc quit")

	centeredContent := lipgloss.Place(
		m.width-4, m.height-6,
		lipgloss.Center, lipgloss.Center,
		content,
	)

	container := lipgloss.JoinVertical(
		lipgloss.Center,
		header,
		"",
		centeredContent,
		"",
		footer,
	)

	return StyleSetupPane.Width(m.width).Render(container)
}

func (m SetupModel) renderProviderStep() string {
	selected := providers[m.selectedIdx]

	var items []string
	for i, p := range providers {
		if i == m.selectedIdx {
			items = append(items, StyleSelectedItem.Render("> "+p))
		} else {
			items = append(items, StyleMuted.Render("  "+p))
		}
	}

	content := StylePrompt.Render("Select your preferred LLM:") + "\n\n"
	content += lipgloss.JoinVertical(lipgloss.Left, items...)
	content += "\n\n" + StyleMuted.Render("Selected: "+StyleAccent.Render(selected))

	return content
}

func (m SetupModel) renderAPIKeyStep() string {
	selectedProvider := providers[m.selectedIdx]

	prompt := StylePrompt.Render("Selected: "+selectedProvider) + "\n\n"
	prompt += "Enter your " + StyleAccent.Render(selectedProvider) + " API key:\n\n"
	prompt += m.apiKeyInput.View() + "\n\n"
	prompt += StyleHint.Render("Your API key is stored locally and never leaves your machine.")

	return prompt
}

func (m SetupModel) renderLocationStep() string {
	prompt := StylePrompt.Render("Enter your location for weather and local news:") + "\n\n"
	prompt += "  City:          " + m.cityInput.View() + "\n"
	prompt += "  Country code: " + m.countryInput.View() + "\n\n"

	if m.err != "" {
		prompt += StyleError.Render("Error: "+m.err) + "\n"
		prompt += StyleHint.Render("Press Enter to go back and try again.")
	} else {
		prompt += StyleHint.Render("Example: Lisbon / PT, New York / US, London / GB")
	}

	return prompt
}

func (m SetupModel) renderSavingStep() string {
	var lines []string

	if m.geocoding {
		lines = append(lines, m.spinner.View()+" Looking up coordinates...")
	}
	if m.saving {
		lines = append(lines, m.spinner.View()+" Saving configuration...")
	}
	if m.err != "" {
		lines = append(lines, StyleError.Render("Error: "+m.err))
		lines = append(lines, StyleHint.Render("Press Enter to go back and try again."))
	}

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

func (m SetupModel) renderDoneStep() string {
	provider := providers[m.selectedIdx]
	location := m.cityInput.Value() + ", " + m.countryInput.Value()

	msg := StyleSuccess.Render("Setup complete!") + "\n\n"
	msg += "  Provider: " + StyleAccent.Render(provider) + "\n"
	msg += "  Location: " + StyleAccent.Render(location) + "\n\n"
	msg += StyleHint.Render("Press any key to launch Watchtower...")

	return msg
}

func (m SetupModel) doGeocode() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		city := m.cityInput.Value()
		country := m.countryInput.Value()
		lat, lon, err := config.Geocode(ctx, city, country)
		return geocodeResultMsg{lat: lat, lon: lon, err: err}
	}
}

func (m SetupModel) doSave(lat, lon float64) tea.Cmd {
	return func() tea.Msg {
		cfg := &config.Config{
			LLMProvider: providers[m.selectedIdx],
			LLMAPIKey:   m.apiKeyInput.Value(),
			Location: config.Location{
				City:      m.cityInput.Value(),
				Country:   m.countryInput.Value(),
				Latitude:  lat,
				Longitude: lon,
			},
			RefreshSec:     120,
			BriefCacheMins: 60,
			CryptoPairs:    []string{"bitcoin", "ethereum", "dogecoin", "usd-coin"},
		}
		err := config.Save(cfg)
		return saveResultMsg{err: err}
	}
}

type geocodeResultMsg struct {
	lat float64
	lon float64
	err error
}

type saveResultMsg struct {
	err error
}
