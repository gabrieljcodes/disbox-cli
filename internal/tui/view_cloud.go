package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"disbox-cli/internal/client"
)

type CloudView struct {
	inputs  []textinput.Model
	focus   int
	status  string
	isError bool
}

func NewCloudView() CloudView {
	labels := []string{
		"PixelDrain API Key",
		"GoFile API Token",
		"1Fichier API Key",
		"Google Drive Token",
		"Dropbox Token",
		"OneDrive Token",
	}

	inputs := make([]textinput.Model, len(labels))
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = "Enter API Key / Token"
		t.CharLimit = 512
		t.Width = 50
		t.Blur() // Start unfocused so arrow keys navigate tabs
		inputs[i] = t
	}

	return CloudView{
		inputs: inputs,
		focus:  -1,
	}
}

func (v *CloudView) SetValues(cfg client.CloudConfig) {
	v.inputs[0].SetValue(cfg.Pixeldrain)
	v.inputs[1].SetValue(cfg.Gofile)
	v.inputs[2].SetValue(cfg.Onefichier)
	v.inputs[3].SetValue(cfg.Google)
	v.inputs[4].SetValue(cfg.Dropbox)
	v.inputs[5].SetValue(cfg.OneDrive)
}

func (v *CloudView) GetValues() client.CloudConfig {
	return client.CloudConfig{
		Pixeldrain: v.inputs[0].Value(),
		Gofile:     v.inputs[1].Value(),
		Onefichier: v.inputs[2].Value(),
		Google:     v.inputs[3].Value(),
		Dropbox:    v.inputs[4].Value(),
		OneDrive:   v.inputs[5].Value(),
	}
}

func (v *CloudView) IsFocused() bool {
	return v.focus >= 0
}

func (v *CloudView) BlurAll() {
	if v.focus >= 0 && v.focus < len(v.inputs) {
		v.inputs[v.focus].Blur()
	}
	v.focus = -1
}

func (v *CloudView) NextInput() {
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

func (v *CloudView) PrevInput() {
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

func (v *CloudView) SetFocus(idx int) {
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

func (v *CloudView) Update(msg tea.Msg) (*CloudView, tea.Cmd) {
	var cmds []tea.Cmd
	for i := range v.inputs {
		var cmd tea.Cmd
		v.inputs[i], cmd = v.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return v, tea.Batch(cmds...)
}

func (v *CloudView) SetStatus(msg string, isErr bool) {
	v.status = msg
	v.isError = isErr
}

func (v CloudView) Render(width int) string {
	var s strings.Builder

	s.WriteString(StyleHeader.Render("☁️  Cloud Integrations & API Keys (Disbox Account)") + "\n\n")

	labels := []string{
		"PixelDrain API Key",
		"GoFile API Token",
		"1Fichier API Key",
		"Google Drive Token",
		"Dropbox Token",
		"OneDrive Token",
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

	var helpLine string
	if v.IsFocused() {
		helpLine = StyleHelp.Render("Tab / ↑↓: Navigate Fields • ") +
			StyleKey.Render("Enter / Ctrl+S") + StyleHelp.Render(": Save • ") +
			StyleKey.Render("Esc") + StyleHelp.Render(": Unfocus & Switch Tabs")
	} else {
		helpLine = StyleHelp.Render("Click field or press ") +
			StyleKey.Render("Tab / Enter") + StyleHelp.Render(" to edit • ") +
			StyleKey.Render("← / →") + StyleHelp.Render(": Switch Tabs")
	}

	panelContent := formBuilder.String() + helpLine
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
