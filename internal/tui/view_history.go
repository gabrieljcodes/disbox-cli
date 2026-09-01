package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"disbox-cli/internal/client"
)

type CloudProviderOption struct {
	ID   string
	Name string
	Icon string
}

var CloudProviders = []CloudProviderOption{
	{ID: "pixeldrain", Name: "PixelDrain", Icon: "🌊"},
	{ID: "gofile", Name: "GoFile", Icon: "📁"},
	{ID: "1fichier", Name: "1Fichier", Icon: "📦"},
	{ID: "googledrive", Name: "Google Drive", Icon: "💾"},
	{ID: "dropbox", Name: "Dropbox", Icon: "📫"},
	{ID: "onedrive", Name: "OneDrive", Icon: "☁️"},
}

type HistoryView struct {
	items       []client.HistoryItem
	cursor      int
	offset      int
	pageSize    int
	status      string
	isError     bool
	searchInput textinput.Model

	// Cloud dispatch modal
	showCloudModal bool
	cloudCursor    int
	sendZip        bool
}

func NewHistoryView() HistoryView {
	ti := textinput.New()
	ti.Placeholder = "Type to search history..."
	ti.Prompt = "🔍 "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.CharLimit = 128
	ti.Width = 40
	ti.Blur()

	return HistoryView{
		items:          make([]client.HistoryItem, 0),
		pageSize:       10,
		searchInput:    ti,
		showCloudModal: false,
		cloudCursor:    0,
		sendZip:        false,
	}
}

func (v *HistoryView) SetItems(items []client.HistoryItem) {
	v.items = items
	v.clampCursor()
}

func (v *HistoryView) getVisibleItems() []client.HistoryItem {
	query := strings.ToLower(strings.TrimSpace(v.searchInput.Value()))
	if query == "" {
		return v.items
	}
	var res []client.HistoryItem
	for _, it := range v.items {
		if strings.Contains(strings.ToLower(it.Name), query) ||
			strings.Contains(strings.ToLower(it.Type), query) {
			res = append(res, it)
		}
	}
	return res
}

func (v *HistoryView) clampCursor() {
	vis := v.getVisibleItems()
	if v.cursor >= len(vis) && len(vis) > 0 {
		v.cursor = len(vis) - 1
	}
	if len(vis) == 0 {
		v.cursor = 0
		v.offset = 0
	}
}

func (v *HistoryView) Next() {
	vis := v.getVisibleItems()
	if len(vis) == 0 {
		return
	}
	if v.cursor < len(vis)-1 {
		v.cursor++
		if v.cursor >= v.offset+v.pageSize {
			v.offset++
		}
	}
}

func (v *HistoryView) Prev() {
	if v.cursor > 0 {
		v.cursor--
		if v.cursor < v.offset {
			v.offset--
		}
	}
}

func (v *HistoryView) NextPage() {
	vis := v.getVisibleItems()
	if len(vis) == 0 {
		return
	}
	if v.offset+v.pageSize < len(vis) {
		v.offset += v.pageSize
		v.cursor = v.offset
	} else {
		v.cursor = len(vis) - 1
	}
}

func (v *HistoryView) PrevPage() {
	if v.offset >= v.pageSize {
		v.offset -= v.pageSize
		v.cursor = v.offset
	} else {
		v.offset = 0
		v.cursor = 0
	}
}

func (v *HistoryView) SelectRow(row int) {
	vis := v.getVisibleItems()
	target := v.offset + row
	if target >= 0 && target < len(vis) {
		v.cursor = target
	}
}

func (v *HistoryView) SelectedItem() *client.HistoryItem {
	vis := v.getVisibleItems()
	if len(vis) == 0 || v.cursor < 0 || v.cursor >= len(vis) {
		return nil
	}
	return &vis[v.cursor]
}

func (v *HistoryView) SetStatus(msg string, isErr bool) {
	v.status = msg
	v.isError = isErr
}

// Search handling
func (v *HistoryView) FocusSearch() {
	v.searchInput.Focus()
}

func (v *HistoryView) BlurSearch() {
	v.searchInput.Blur()
}

func (v *HistoryView) IsSearchFocused() bool {
	return v.searchInput.Focused()
}

func (v *HistoryView) ClearSearch() {
	v.searchInput.SetValue("")
	v.offset = 0
	v.cursor = 0
}

func (v *HistoryView) UpdateSearch(msg tea.Msg) (*HistoryView, tea.Cmd) {
	var cmd tea.Cmd
	v.searchInput, cmd = v.searchInput.Update(msg)
	v.offset = 0
	v.clampCursor()
	return v, cmd
}

// Modal handling
func (v *HistoryView) OpenCloudModal() {
	if v.SelectedItem() != nil {
		v.showCloudModal = true
		v.cloudCursor = 0
		v.sendZip = false
		v.searchInput.Blur()
	}
}

func (v *HistoryView) CloseCloudModal() {
	v.showCloudModal = false
}

func (v *HistoryView) IsModalOpen() bool {
	return v.showCloudModal
}

func (v *HistoryView) ToggleZip() {
	v.sendZip = !v.sendZip
}

func (v *HistoryView) NextCloudProvider() {
	v.cloudCursor = (v.cloudCursor + 1) % len(CloudProviders)
}

func (v *HistoryView) PrevCloudProvider() {
	v.cloudCursor = (v.cloudCursor - 1 + len(CloudProviders)) % len(CloudProviders)
}

func (v *HistoryView) SelectCloudProvider(idx int) {
	if idx >= 0 && idx < len(CloudProviders) {
		v.cloudCursor = idx
	}
}

func (v *HistoryView) SelectedCloudProvider() (string, bool) {
	return CloudProviders[v.cloudCursor].ID, v.sendZip
}

func (v HistoryView) Render(width, height int) string {
	var s strings.Builder

	panelWidth := width - 6
	if panelWidth < 30 {
		panelWidth = 30
	}

	s.WriteString(StyleHeader.Render("📜 Disbox Download History & Cloud Export") + "\n")

	// ── Cloud Export Modal ──
	if v.showCloudModal {
		item := v.SelectedItem()
		itemName := "Selected File"
		if item != nil {
			itemName = item.Name
		}
		if len(itemName) > panelWidth-20 && panelWidth > 25 {
			itemName = itemName[:panelWidth-23] + "..."
		}

		var modalContent strings.Builder
		modalContent.WriteString(StyleHeader.Render(fmt.Sprintf("☁️  Dispatch '%s' to Cloud", itemName)) + "\n")
		modalContent.WriteString(StyleValue.Render("Select destination cloud provider:\n"))

		for i, p := range CloudProviders {
			marker := "  "
			rowText := fmt.Sprintf("[%d] %s %s (%s)", i+1, p.Icon, p.Name, p.ID)
			if i == v.cloudCursor {
				marker = StyleKey.Render("▶ ")
				modalContent.WriteString(StyleRowSelected.Render(marker+rowText) + "\n")
			} else {
				modalContent.WriteString("  " + rowText + "\n")
			}
		}

		zipBox := "[ ] Send as .ZIP archive"
		if v.sendZip {
			zipBox = "[x] Send as .ZIP archive"
		}
		modalContent.WriteString(StyleValue.Render(zipBox) + "  " + StyleHelp.Render("(Press 'z' to toggle)\n\n"))

		modalContent.WriteString(
			StyleHelp.Render("Press ") + StyleKey.Render("1-6") + StyleHelp.Render(" / ") + StyleKey.Render("Enter") + StyleHelp.Render(" to Dispatch • ") +
				StyleKey.Render("Esc") + StyleHelp.Render(" to Cancel"),
		)

		s.WriteString(StylePanel.Width(panelWidth).Render(modalContent.String()))
		return s.String()
	}

	// ── Normal History View ──
	// Calculate dynamic page size based on available window height
	calcPageSize := height - 14
	if calcPageSize < 4 {
		calcPageSize = 4
	} else if calcPageSize > 12 {
		calcPageSize = 12
	}
	v.pageSize = calcPageSize

	vis := v.getVisibleItems()

	// Search bar row
	var searchBoxStyle lipgloss.Style
	if v.searchInput.Focused() {
		searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(0, 1)
	} else {
		searchBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)
	}

	searchHint := StyleHelp.Render("Press '/' to search")
	if v.searchInput.Focused() {
		searchHint = StyleHelp.Render("Type query • Enter/Esc when done")
	} else if v.searchInput.Value() != "" {
		searchHint = StyleHelp.Render(fmt.Sprintf("%d matched • '/' edit • Ctrl+U clear", len(vis)))
	}

	searchRow := lipgloss.JoinHorizontal(lipgloss.Center, searchBoxStyle.Render(v.searchInput.View()), "  ", searchHint)

	if len(vis) == 0 {
		var emptyMsg string
		if v.searchInput.Value() != "" {
			emptyMsg = fmt.Sprintf("No history matches for '%s'.\nPress '/' to edit search, or Ctrl+U to clear.", v.searchInput.Value())
		} else {
			emptyMsg = "No history items found.\nPress " + StyleKey.Render("r") + " to refresh from server."
		}

		panelContent := searchRow + "\n\n" + emptyMsg
		s.WriteString(StylePanel.Width(panelWidth).Render(panelContent))
		return s.String()
	}

	start := v.offset
	end := start + v.pageSize
	if end > len(vis) {
		end = len(vis)
	}

	totalPages := (len(vis) + v.pageSize - 1) / v.pageSize
	currentPage := (v.offset / v.pageSize) + 1
	if totalPages == 0 {
		totalPages = 1
	}

	var listBuilder strings.Builder
	listBuilder.WriteString(searchRow + "\n")

	for i := start; i < end; i++ {
		item := vis[i]
		typeBadge := StyleBadgeGreen.Render(" TORRENT ")
		if item.Type == "webdl" {
			typeBadge = StyleBadgeBlue.Render(" WEBDL ")
		}

		sizeStr := FormatBytes(item.Size)
		dateStr := item.CreatedAt.Format("2006-01-02 15:04")

		name := item.Name
		maxNameWidth := panelWidth - 42
		if maxNameWidth < 20 {
			maxNameWidth = 20
		}
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-3] + "..."
		}

		row := fmt.Sprintf("%s %s (%s) • %s", typeBadge, name, sizeStr, dateStr)

		if i == v.cursor {
			listBuilder.WriteString(StyleRowSelected.Render("▶ "+row) + "\n")
		} else {
			listBuilder.WriteString("  " + row + "\n")
		}
	}

	// Pagination bar with interactive buttons
	prevBtn := StyleBadgeBlue.Render(" ◀ Prev (PgUp) ")
	nextBtn := StyleBadgeBlue.Render(" Next (PgDn) ▶ ")
	pageInfo := StyleValue.Render(fmt.Sprintf("Page %d of %d (%d items)", currentPage, totalPages, len(vis)))

	paginationBar := fmt.Sprintf("%s  %s  %s\n", prevBtn, pageInfo, nextBtn)

	helpLine := StyleHelp.Render("↑↓: Select • ") +
		StyleKey.Render("d/Enter") + StyleHelp.Render(": Download • ") +
		StyleKey.Render("c/p") + StyleHelp.Render(": Cloud • ") +
		StyleKey.Render("/") + StyleHelp.Render(": Search • ") +
		StyleKey.Render("←/→") + StyleHelp.Render(": Tabs")

	panelContent := listBuilder.String() + "\n" + paginationBar + helpLine
	if v.status != "" {
		if v.isError {
			panelContent += "\n" + StyleError.Render("❌ "+v.status)
		} else {
			panelContent += "\n" + StyleSuccess.Render("✓ "+v.status)
		}
	}

	s.WriteString(StylePanel.Width(panelWidth).Render(panelContent))
	return s.String()
}
