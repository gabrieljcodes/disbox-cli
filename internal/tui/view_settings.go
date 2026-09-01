package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"disbox-cli/internal/config"
)

type SettingsView struct {
	inputs  []textinput.Model
	focus   int
	status  string
	isError bool
}

func NewSettingsView() SettingsView {
	labels := []string{
		"Disbox Server URL",
		"Disbox API Token (Bearer)",
		"Local Download Folder",
		"Max Concurrent Downloads",
	}

	inputs := make([]textinput.Model, len(labels))
	for i := range inputs {
		t := textinput.New()
		t.CharLimit = 512
		t.Width = 50
		t.Blur() // Start unfocused so arrow keys navigate tabs
		inputs[i] = t
	}

	return SettingsView{
		inputs: inputs,
		focus:  -1,
	}
}

func (v *SettingsView) SetValues(cfg config.Config) {
	v.inputs[0].SetValue(cfg.ServerURL)
	v.inputs[1].SetValue(cfg.APIToken)
	v.inputs[2].SetValue(cfg.DownloadDir)
	v.inputs[3].SetValue(strconv.Itoa(cfg.MaxConcurrentDownloads))
}

func (v *SettingsView) GetValues() (config.Config, error) {
	concurrent, err := strconv.Atoi(v.inputs[3].Value())
	if err != nil || concurrent <= 0 {
		concurrent = 3
	}

	return config.Config{
		ServerURL:              strings.TrimRight(v.inputs[0].Value(), "/"),
		APIToken:               v.inputs[1].Value(),
		DownloadDir:            v.inputs[2].Value(),
		MaxConcurrentDownloads: concurrent,
	}, nil
}

func (v *SettingsView) IsFocused() bool {
	return v.focus >= 0
}

func (v *SettingsView) BlurAll() {
	if v.focus >= 0 && v.focus < len(v.inputs) {
		v.inputs[v.focus].Blur()
	}
	v.focus = -1
}

func (v *SettingsView) NextInput() {
	if v.focus >= 0 && v.focus < len(v.inputs) {
		v.inputs[v.focus].Blur()
	}
	if v.focus == -1 {
		v.focus = 0
	} else {
		v.focus = (v.focus + 1) % len(v.inputs)
	}
	v.inputs[v.focus].Focus()
}

func (v *SettingsView) PrevInput() {
	if v.focus >= 0 && v.focus < len(v.inputs) {
		v.inputs[v.focus].Blur()
	}
	if v.focus == -1 {
		v.focus = len(v.inputs) - 1
	} else {
		v.focus = (v.focus - 1 + len(v.inputs)) % len(v.inputs)
	}
	v.inputs[v.focus].Focus()
}

func (v *SettingsView) SetFocus(idx int) {
	if v.focus >= 0 && v.focus < len(v.inputs) {
		v.inputs[v.focus].Blur()
	}
	if idx >= 0 && idx < len(v.inputs) {
		v.focus = idx
		v.inputs[v.focus].Focus()
	} else {
		v.focus = -1
	}
}

func (v *SettingsView) Update(msg tea.Msg) (*SettingsView, tea.Cmd) {
	var cmds []tea.Cmd
	for i := range v.inputs {
		var cmd tea.Cmd
		v.inputs[i], cmd = v.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return v, tea.Batch(cmds...)
}

func (v *SettingsView) SetStatus(msg string, isErr bool) {
	v.status = msg
	v.isError = isErr
}

func (v SettingsView) Render(width int) string {
	var s strings.Builder

	s.WriteString(StyleHeader.Render("⚙️  CLI Settings & Preferences") + "\n\n")

	labels := []string{
		"Disbox Server URL",
		"Disbox API Token (Bearer)",
		"Local Download Folder",
		"Max Concurrent Downloads",
	}

	var formBuilder strings.Builder
	for i, label := range labels {
		marker := "  "
		if i == v.focus {
			marker = StyleKey.Render("▶ ")
		}
		field := fmt.Sprintf("%s%s:\n  %s\n", marker, StyleValue.Render(label), v.inputs[i].View())
		formBuilder.WriteString(field + "\n")
	}

	cfgPath, _ := config.GetConfigPath()
	infoLine := StyleValue.Render("Config file saved at: " + cfgPath + "\n\n")

	var helpLine string
	if v.IsFocused() {
		helpLine = StyleHelp.Render("Tab / ↑↓: Navigate Fields • ") +
			StyleKey.Render("Enter / Ctrl+S") + StyleHelp.Render(": Save Settings • ") +
			StyleKey.Render("Esc") + StyleHelp.Render(": Unfocus & Switch Tabs")
	} else {
		helpLine = StyleHelp.Render("Click field or press ") +
			StyleKey.Render("Tab / Enter") + StyleHelp.Render(" to edit • ") +
			StyleKey.Render("← / →") + StyleHelp.Render(": Switch Tabs")
	}

	panelContent := formBuilder.String() + infoLine + helpLine
	if v.status != "" {
		if v.isError {
			panelContent += "\n\n" + StyleError.Render("❌ "+v.status)
		} else {
			panelContent += "\n\n" + StyleSuccess.Render("✓ "+v.status)
		}
	}

	s.WriteString(StylePanel.Width(width - 8).Render(panelContent))
	return s.String()
}
