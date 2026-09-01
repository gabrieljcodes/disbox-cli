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

func (v AddView) Render(width int) string {
	var s strings.Builder

	s.WriteString(StyleHeader.Render("➕ Add Downloads (Single or Multiple Links / Magnets)") + "\n\n")

	boxWidth := width - 14
	if boxWidth < 30 {
		boxWidth = 30
	}
	v.textarea.SetWidth(boxWidth - 4)

	var boxStyle lipgloss.Style
	var boxHint string
	if v.textarea.Focused() {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1).
			Width(boxWidth)
		boxHint = StyleHelp.Render("⌨️  Editing Mode: ") +
			StyleKey.Render("Ctrl+S") + StyleHelp.Render(" / ") + StyleKey.Render("Alt+Enter") + StyleHelp.Render(" / ") + StyleKey.Render("Ctrl+D") + StyleHelp.Render(": Submit • ") +
			StyleKey.Render("Enter") + StyleHelp.Render(": New line • ") +
			StyleKey.Render("Esc") + StyleHelp.Render(": Unfocus")
	} else {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1).
			Width(boxWidth)
		boxHint = StyleHelp.Render("💡 Click box or press ") +
			StyleKey.Render("Enter") + StyleHelp.Render(" to paste/type • Use ") +
			StyleKey.Render("← / →") + StyleHelp.Render(" to switch tabs")
	}

	inputDisplay := boxStyle.Render(v.textarea.View())

	linkCount := len(v.GetLinks())
	var countInfo string
	if linkCount > 0 {
		countInfo = fmt.Sprintf(" (%d links detected)", linkCount)
	}

	// ── Options Row ──
	toggleDownStr := "[x] Automatically download to local folder when ready"
	if !v.autoDownload {
		toggleDownStr = "[ ] Automatically download to local folder when ready"
	}

	toggleCloudStr := "[x] Automatically send to Cloud when ready on Debrid"
	if !v.autoCloud {
		toggleCloudStr = "[ ] Automatically send to Cloud when ready on Debrid"
	}

	currProv := CloudProviders[v.cloudProviderIdx]
	cloudDetails := ""
	if v.autoCloud {
		zipStr := "[ ] Send as .ZIP"
		if v.cloudZip {
			zipStr = "[x] Send as .ZIP"
		}
		cloudDetails = fmt.Sprintf(
			"    Destination: %s  %s  │  %s  %s\n",
			StyleBadgeBlue.Render(fmt.Sprintf(" %s %s (%s) ", currProv.Icon, currProv.Name, currProv.ID)),
			StyleHelp.Render("(Click / 'p' to change)"),
			StyleValue.Render(zipStr),
			StyleHelp.Render("(Click / 'z' to toggle)"),
		)
	}

	submitBtn := StyleBadgeGreen.Render(fmt.Sprintf(" 🚀 [ Submit All %d Link(s) (Ctrl+S / Alt+Enter) ] ", linkCount))
	if linkCount == 0 {
		submitBtn = StyleTabInactive.Render(" 🚀 [ Submit All Links (Ctrl+S / Alt+Enter) ] ")
	}

	content := fmt.Sprintf("Paste one or multiple Torrent Magnet links or WebDL URLs%s:\n\n", StyleValue.Render(countInfo)) +
		inputDisplay + "\n" +
		boxHint + "\n\n" +
		StyleValue.Render(toggleDownStr) + "  " + StyleHelp.Render("('Ctrl+T' / Click)") + "\n" +
		StyleValue.Render(toggleCloudStr) + "  " + StyleHelp.Render("('Ctrl+G' / Click)") + "\n" +
		cloudDetails + "\n" +
		submitBtn

	if v.message != "" {
		content += "\n\n"
		if v.isError {
			content += StyleError.Render("❌ " + v.message)
		} else {
			content += StyleSuccess.Render("✓ " + v.message)
		}
	} else if v.submitting {
		content += "\n\n" + StyleBadgeBlue.Render(" Submitting to Disbox... ")
	}

	s.WriteString(StylePanel.Width(width - 8).Render(content))
	return s.String()
}
