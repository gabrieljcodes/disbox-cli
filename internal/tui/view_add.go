package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AddView struct {
	textarea         textarea.Model
	autoDownload     bool
	autoCloud        bool
	cloudProviderIdx int
	cloudZip         bool
	message          string
	isError          bool
	submitting       bool
}

func NewAddView() AddView {
	ta := textarea.New()
	ta.Placeholder = "Paste one or multiple links (one per line):\nhttps://pixeldrain.com/u/F6KT8AQS\nhttps://pixeldrain.com/u/YvRk3NRH\nmagnet:?xt=urn:btih:..."
	ta.Prompt = " "
	ta.ShowLineNumbers = false
	ta.SetHeight(5)
	ta.SetWidth(60)
	ta.Blur()
	ta.CharLimit = 32768

	return AddView{
		textarea:         ta,
		autoDownload:     true,
		autoCloud:        false,
		cloudProviderIdx: 0,
		cloudZip:         false,
	}
}

func (v *AddView) Focus() {
	v.textarea.Focus()
}

func (v *AddView) Blur() {
	v.textarea.Blur()
}

func (v *AddView) IsFocused() bool {
	return v.textarea.Focused()
}

func (v *AddView) Value() string {
	return v.textarea.Value()
}

func (v *AddView) GetLinks() []string {
	raw := v.textarea.Value()
	lines := strings.Split(raw, "\n")
	var links []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			links = append(links, trimmed)
		}
	}
	return links
}

func (v *AddView) Update(msg tea.Msg) (*AddView, tea.Cmd) {
	var cmd tea.Cmd
	v.textarea, cmd = v.textarea.Update(msg)
	return v, cmd
}

func (v *AddView) SetStatus(msg string, isErr bool) {
	v.message = msg
	v.isError = isErr
	v.submitting = false
}

func (v *AddView) Reset() {
	v.textarea.SetValue("")
	v.textarea.Blur()
	v.message = ""
	v.isError = false
	v.submitting = false
}

func (v *AddView) ToggleAutoDownload() {
	v.autoDownload = !v.autoDownload
}

func (v *AddView) ToggleAutoCloud() {
	v.autoCloud = !v.autoCloud
}

func (v *AddView) CycleCloudProvider() {
	v.cloudProviderIdx = (v.cloudProviderIdx + 1) % len(CloudProviders)
}

func (v *AddView) SetCloudProvider(idx int) {
	if idx >= 0 && idx < len(CloudProviders) {
		v.cloudProviderIdx = idx
	}
}

func (v *AddView) ToggleCloudZip() {
	v.cloudZip = !v.cloudZip
}

func (v *AddView) GetCloudProvider() string {
	return CloudProviders[v.cloudProviderIdx].ID
}

func (v AddView) Render(width, height int) string {
	var s strings.Builder

	s.WriteString(StyleHeader.Render("➕ Add Downloads (Single or Multiple Links / Magnets)") + "\n")

	boxWidth := width - 12
	if boxWidth < 30 {
		boxWidth = 30
	}
	v.textarea.SetWidth(boxWidth - 4)

	// Dynamically adjust textarea height based on terminal height
	taHeight := 3
	if height >= 30 {
		taHeight = 5
	} else if height >= 26 {
		taHeight = 4
	}
	v.textarea.SetHeight(taHeight)

	var boxStyle lipgloss.Style
	var boxHint string
	if v.textarea.Focused() {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1).
			Width(boxWidth)
		boxHint = StyleHelp.Render("⌨️  ") +
			StyleKey.Render("Ctrl+S") + StyleHelp.Render(" / ") + StyleKey.Render("Ctrl+D") + StyleHelp.Render(": Submit • ") +
			StyleKey.Render("Enter") + StyleHelp.Render(": New line • ") +
			StyleKey.Render("Esc") + StyleHelp.Render(": Unfocus")
	} else {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			Width(boxWidth)
		boxHint = StyleHelp.Render("💡 ") +
			StyleKey.Render("Enter") + StyleHelp.Render(" / Click to type • ") +
			StyleKey.Render("← / →") + StyleHelp.Render(" to switch tabs")
	}

	inputDisplay := boxStyle.Render(v.textarea.View())

	linkCount := len(v.GetLinks())
	var countInfo string
	if linkCount > 0 {
		countInfo = fmt.Sprintf(" (%d link(s))", linkCount)
	}

	// ── Options Row ──
	toggleDownStr := "[x] Auto download to local folder"
	if !v.autoDownload {
		toggleDownStr = "[ ] Auto download to local folder"
	}

	toggleCloudStr := "[x] Auto send to Cloud on Debrid"
	if !v.autoCloud {
		toggleCloudStr = "[ ] Auto send to Cloud on Debrid"
	}

	currProv := CloudProviders[v.cloudProviderIdx]
	cloudDetails := ""
	if v.autoCloud {
		zipStr := "[ ] .ZIP"
		if v.cloudZip {
			zipStr = "[x] .ZIP"
		}
		cloudDetails = fmt.Sprintf(
			"    → %s %s │ %s %s\n",
			StyleBadgeBlue.Render(fmt.Sprintf(" %s %s ", currProv.Icon, currProv.Name)),
			StyleHelp.Render("(Click/'p')"),
			StyleValue.Render(zipStr),
			StyleHelp.Render("(Click/'z')"),
		)
	}

	submitBtn := StyleBadgeGreen.Render(fmt.Sprintf(" 🚀 [ Submit %d Link(s) (Ctrl+S / Click) ] ", linkCount))
	if linkCount == 0 {
		submitBtn = StyleTabInactive.Render(" 🚀 [ Submit Links (Ctrl+S / Click) ] ")
	}

	content := fmt.Sprintf("Paste Torrent Magnets or WebDL URLs%s:\n", StyleValue.Render(countInfo)) +
		inputDisplay + "\n" +
		boxHint + "\n" +
		StyleValue.Render(toggleDownStr) + "  " + StyleHelp.Render("('Ctrl+T' / Click)") + "\n" +
		StyleValue.Render(toggleCloudStr) + "  " + StyleHelp.Render("('Ctrl+G' / Click)") + "\n" +
		cloudDetails +
		submitBtn

	if v.message != "" {
		content += "\n"
		if v.isError {
			content += StyleError.Render("❌ " + v.message)
		} else {
			content += StyleSuccess.Render("✓ " + v.message)
		}
	} else if v.submitting {
		content += "\n" + StyleBadgeBlue.Render(" Submitting to Disbox... ")
	}

	panelWidth := width - 6
	if panelWidth < 30 {
		panelWidth = 30
	}
	s.WriteString(StylePanel.Width(panelWidth).Render(content))
	return s.String()
}
