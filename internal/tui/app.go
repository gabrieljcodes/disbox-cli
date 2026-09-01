package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"disbox-cli/internal/client"
	"disbox-cli/internal/config"
	"disbox-cli/internal/downloader"
)

type tickMsg time.Time
type historyLoadedMsg []client.HistoryItem
type cloudLoadedMsg *client.CloudConfig
type errMsg error

type addResultMsg struct {
	Count          int
	Failures       int
	AutoDownload   bool
	AutoCloud      bool
	CloudProvider  string
	CloudZip       bool
	ExistingTokens map[string]bool
	SubmittedLinks []string
	Err            error
}

type enqueueMsg struct {
	Name          string
	DownloadURL   string
	Token         string
	Size          int64
	AutoCloud     bool
	CloudProvider string
	CloudZip      bool
}

type AppModel struct {
	cfg       config.Config
	apiClient *client.Client
	queueMgr  *downloader.QueueManager

	activeTab int
	width     int
	height    int
	tabWidths []int // rendered width of each tab for mouse hit detection

	downloadsView DownloadsView
	addView       AddView
	historyView   HistoryView
	cloudView     CloudView
	settingsView  SettingsView
}

func NewAppModel(cfg config.Config) AppModel {
	apiClient := client.New(cfg.ServerURL, cfg.APIToken)

	// The readiness checker polls /v1/progress to know when debrid is done.
	missCount := 0
	checker := func(token string) (bool, float64, string, error) {
		progMap, err := apiClient.GetProgress([]string{token})
		if err != nil {
			return false, 0, "", err
		}
		prog, ok := progMap[token]
		if !ok {
			missCount++
			if missCount >= 10 {
				return true, 100, "ready", nil
			}
			return false, 0, "waiting for debrid...", nil
		}
		missCount = 0

		state := strings.ToLower(prog.DownloadState)
		pct := prog.Progress
		if pct > 0 && pct <= 1 {
			pct *= 100
		}

		switch state {
		case "completed", "finished", "cached", "seeding", "downloaded":
			return true, 100, state, nil
		}
		if pct >= 100 {
			return true, 100, state, nil
		}
		return false, pct, state, nil
	}

	dispatcher := func(provider, token string, zip bool) (string, error) {
		return apiClient.SendToCloud(provider, token, zip)
	}

	queueMgr := downloader.NewQueueManager(cfg.MaxConcurrentDownloads, cfg.APIToken, checker, dispatcher)

	addView := NewAddView()
	settingsView := NewSettingsView()
	settingsView.SetValues(cfg)

	return AppModel{
		cfg:           cfg,
		apiClient:     apiClient,
		queueMgr:      queueMgr,
		activeTab:     0,
		width:         80,
		height:        24,
		downloadsView: NewDownloadsView(),
		addView:       addView,
		historyView:   NewHistoryView(),
		cloudView:     NewCloudView(),
		settingsView:  settingsView,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchHistoryCmd(),
		m.fetchCloudCmd(),
		m.tickCmd(),
	)
}

func (m AppModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m AppModel) fetchHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.apiClient.GetHistory()
		if err != nil {
			return errMsg(err)
		}
		return historyLoadedMsg(items)
	}
}

func (m AppModel) fetchCloudCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := m.apiClient.GetCloudConfig()
		if err != nil {
			return errMsg(err)
		}
		return cloudLoadedMsg(cfg)
	}
}

func (m *AppModel) switchTab(tabIndex int) []tea.Cmd {
	if tabIndex < 0 {
		tabIndex = 4
	} else if tabIndex > 4 {
		tabIndex = 0
	}
	m.activeTab = tabIndex

	// When changing tabs, blur inputs and close modals
	m.addView.Blur()
	m.historyView.BlurSearch()
	m.historyView.CloseCloudModal()
	m.cloudView.BlurAll()
	m.settingsView.BlurAll()

	var cmds []tea.Cmd
	switch tabIndex {
	case 2:
		cmds = append(cmds, m.fetchHistoryCmd())
	case 3:
		cmds = append(cmds, m.fetchCloudCmd())
	}
	return cmds
}

func (m *AppModel) submitLinksCmd() tea.Cmd {
	links := m.addView.GetLinks()
	if len(links) == 0 {
		return nil
	}
	m.addView.submitting = true
	autoDown := m.addView.autoDownload
	autoCloud := m.addView.autoCloud
	cloudProv := m.addView.GetCloudProvider()
	cloudZip := m.addView.cloudZip
	apiClient := m.apiClient

	// Capture existing history tokens before submission
	existingTokens := make(map[string]bool)
	for _, it := range m.historyView.items {
		tok := it.Token
		if tok == "" {
			tok = it.LinkToken
		}
		if tok != "" {
			existingTokens[tok] = true
		}
	}

	return func() tea.Msg {
		successCount := 0
		failCount := 0
		var lastErr error
		for _, link := range links {
			var addErr error
			if strings.HasPrefix(link, "magnet:") {
				_, addErr = apiClient.AddTorrent(link)
			} else {
				_, addErr = apiClient.AddWebDL(link)
			}
			if addErr != nil {
				failCount++
				lastErr = addErr
			} else {
				successCount++
			}
		}

		if successCount == 0 && lastErr != nil {
			return addResultMsg{Err: lastErr}
		}
		return addResultMsg{
			Count:          successCount,
			Failures:       failCount,
			AutoDownload:   autoDown,
			AutoCloud:      autoCloud,
			CloudProvider:  cloudProv,
			CloudZip:       cloudZip,
			ExistingTokens: existingTokens,
			SubmittedLinks: links,
		}
	}
}

func (m *AppModel) dispatchSelectedToCloudCmd(provider string, zip bool) tea.Cmd {
	selected := m.historyView.SelectedItem()
	if selected == nil {
		return nil
	}
	m.historyView.CloseCloudModal()
	apiClient := m.apiClient
	tok := selected.Token
	name := selected.Name
	return func() tea.Msg {
		detail, err := apiClient.SendToCloud(provider, tok, zip)
		if err != nil {
			return errMsg(fmt.Errorf("Failed to send '%s' to %s: %w", name, provider, err))
		}
		_ = detail
		return historyLoadedMsg(nil)
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		cmds = append(cmds, m.tickCmd())

	case historyLoadedMsg:
		if msg != nil {
			m.historyView.SetItems([]client.HistoryItem(msg))
		} else {
			cmds = append(cmds, m.fetchHistoryCmd())
		}

	case addResultMsg:
		if msg.Err != nil {
			m.addView.SetStatus("Failed to submit: "+msg.Err.Error(), true)
		} else {
			statusText := fmt.Sprintf("Submitted %d download(s) to Disbox!", msg.Count)
			if msg.Failures > 0 {
				statusText += fmt.Sprintf(" (%d failed)", msg.Failures)
			}
			m.addView.SetStatus(statusText, false)
			m.addView.Reset()
			cmds = append(cmds, m.fetchHistoryCmd())
			if (msg.AutoDownload || msg.AutoCloud) && msg.Count > 0 {
				m.activeTab = 0
				apiClient := m.apiClient
				downloadDir := m.cfg.DownloadDir
				queueMgr := m.queueMgr
				targetCount := msg.Count
				autoCloud := msg.AutoCloud
				cloudProv := msg.CloudProvider
				cloudZip := msg.CloudZip
				existingTokens := msg.ExistingTokens
				submittedLinks := msg.SubmittedLinks

				// Smart poller to discover and enqueue newly added items
				cmds = append(cmds, func() tea.Msg {
					enqueuedTokens := make(map[string]bool)
					var lastItems []client.HistoryItem

					// Poll up to 15 attempts (1s intervals)
					for attempt := 0; attempt < 15; attempt++ {
						time.Sleep(1 * time.Second)
						items, err := apiClient.GetHistory()
						if err != nil {
							continue
						}
						lastItems = items

						for _, item := range items {
							tok := item.Token
							if tok == "" {
								tok = item.LinkToken
							}
							if tok == "" || enqueuedTokens[tok] {
								continue
							}

							// Check if this item is newly created OR matches one of the submitted URLs
							isNewToken := !existingTokens[tok]
							matchesURL := false
							if len(submittedLinks) > 0 && item.SourceURL != "" {
								cleanSource := strings.TrimRight(item.SourceURL, "/")
								for _, sl := range submittedLinks {
									if strings.EqualFold(cleanSource, strings.TrimRight(sl, "/")) {
										matchesURL = true
										break
									}
								}
							}

							if isNewToken || matchesURL {
								enqueuedTokens[tok] = true
								queueMgr.Enqueue(item.Name, item.DownloadURL, tok, downloadDir, item.Size, autoCloud, cloudProv, cloudZip)
							}
						}

						if len(enqueuedTokens) >= targetCount {
							break
						}
					}
					return historyLoadedMsg(lastItems)
				})
			}
		}

	case enqueueMsg:
		m.queueMgr.Enqueue(msg.Name, msg.DownloadURL, msg.Token, m.cfg.DownloadDir, msg.Size, msg.AutoCloud, msg.CloudProvider, msg.CloudZip)
		m.activeTab = 0

	case cloudLoadedMsg:
		if msg != nil {
			m.cloudView.SetValues(*msg)
		}

	case errMsg:
		if msg != nil {
			m.historyView.SetStatus(msg.Error(), true)
		}

	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseLeft:
			if clickedCmds := m.handleTabClick(msg.X, msg.Y); clickedCmds != nil {
				cmds = append(cmds, clickedCmds...)
				return m, tea.Batch(cmds...)
			}
			if contentCmds := m.handleContentClick(msg.X, msg.Y); contentCmds != nil {
				cmds = append(cmds, contentCmds...)
				return m, tea.Batch(cmds...)
			}

		case tea.MouseWheelUp:
			if m.activeTab == 2 {
				m.historyView.Prev()
			}

		case tea.MouseWheelDown:
			if m.activeTab == 2 {
				m.historyView.Next()
			}
		}

	case tea.KeyMsg:
		key := msg.String()

		// ── Global Quit ──
		if key == "ctrl+c" {
			return m, tea.Quit
		}

		// ── F-keys & Alt+1..5 work from any tab ──
		switch key {
		case "f1", "alt+1":
			cmds = append(cmds, m.switchTab(0)...)
			return m, tea.Batch(cmds...)
		case "f2", "alt+2":
			cmds = append(cmds, m.switchTab(1)...)
			return m, tea.Batch(cmds...)
		case "f3", "alt+3":
			cmds = append(cmds, m.switchTab(2)...)
			return m, tea.Batch(cmds...)
		case "f4", "alt+4":
			cmds = append(cmds, m.switchTab(3)...)
			return m, tea.Batch(cmds...)
		case "f5", "alt+5":
			cmds = append(cmds, m.switchTab(4)...)
			return m, tea.Batch(cmds...)
		}

		// ── Per-tab dispatch ──
		switch m.activeTab {
		case 0: // Queue
			switch key {
			case "1":
				cmds = append(cmds, m.switchTab(0)...)
			case "2":
				cmds = append(cmds, m.switchTab(1)...)
			case "3":
				cmds = append(cmds, m.switchTab(2)...)
			case "4":
				cmds = append(cmds, m.switchTab(3)...)
			case "5":
				cmds = append(cmds, m.switchTab(4)...)
			case "tab", "right", "l":
				cmds = append(cmds, m.switchTab(m.activeTab+1)...)
			case "shift+tab", "left", "h":
				cmds = append(cmds, m.switchTab(m.activeTab-1)...)
			case "c":
				n := m.queueMgr.ClearFinished()
				if n > 0 {
					m.downloadsView.flash = fmt.Sprintf("Cleared %d finished tasks", n)
				}
			case "r":
				cmds = append(cmds, m.fetchHistoryCmd())
			case "q":
				return m, tea.Quit
			}

		case 1: // Add Download
			if !m.addView.IsFocused() {
				switch key {
				case "1":
					cmds = append(cmds, m.switchTab(0)...)
				case "2":
					cmds = append(cmds, m.switchTab(1)...)
				case "3":
					cmds = append(cmds, m.switchTab(2)...)
				case "4":
					cmds = append(cmds, m.switchTab(3)...)
				case "5":
					cmds = append(cmds, m.switchTab(4)...)
				case "left", "h", "shift+tab", "[":
					cmds = append(cmds, m.switchTab(0)...)
				case "right", "l", "tab", "]":
					cmds = append(cmds, m.switchTab(2)...)
				case "enter", "i", " ":
					if len(m.addView.GetLinks()) > 0 {
						if submitCmd := m.submitLinksCmd(); submitCmd != nil {
							return m, submitCmd
						}
					}
					m.addView.Focus()
					return m, nil
				case "s", "ctrl+s", "alt+enter", "ctrl+d":
					if submitCmd := m.submitLinksCmd(); submitCmd != nil {
						return m, submitCmd
					}
				case "t", "ctrl+t":
					m.addView.ToggleAutoDownload()
					return m, nil
				case "g", "ctrl+g":
					m.addView.ToggleAutoCloud()
					return m, nil
				case "p":
					m.addView.CycleCloudProvider()
					return m, nil
				case "z":
					m.addView.ToggleCloudZip()
					return m, nil
				case "esc", "q":
					cmds = append(cmds, m.switchTab(0)...)
					return m, tea.Batch(cmds...)
				}
				return m, nil
			}

			// Focused mode (typing/pasting in textarea)
			switch key {
			case "esc":
				m.addView.Blur()
				return m, nil
			case "ctrl+t":
				m.addView.ToggleAutoDownload()
				return m, nil
			case "ctrl+g":
				m.addView.ToggleAutoCloud()
				return m, nil
			case "ctrl+s", "ctrl+d", "alt+enter", "ctrl+j":
				if submitCmd := m.submitLinksCmd(); submitCmd != nil {
					return m, submitCmd
				}
				return m, nil
			}
			var cmd tea.Cmd
			_, cmd = m.addView.Update(msg)
			cmds = append(cmds, cmd)

		case 2: // History
			// ── Modal Mode: Choosing Cloud Destination ──
			if m.historyView.IsModalOpen() {
				switch key {
				case "1", "2", "3", "4", "5", "6":
					idx := int(key[0] - '1')
					m.historyView.SelectCloudProvider(idx)
					provider, zip := m.historyView.SelectedCloudProvider()
					return m, m.dispatchSelectedToCloudCmd(provider, zip)
				case "up", "k":
					m.historyView.PrevCloudProvider()
				case "down", "j":
					m.historyView.NextCloudProvider()
				case "z":
					m.historyView.ToggleZip()
				case "enter":
					provider, zip := m.historyView.SelectedCloudProvider()
					return m, m.dispatchSelectedToCloudCmd(provider, zip)
				case "esc", "q":
					m.historyView.CloseCloudModal()
				}
				return m, nil
			}

			// ── Search Input Mode ──
			if m.historyView.IsSearchFocused() {
				switch key {
				case "esc", "enter":
					m.historyView.BlurSearch()
					return m, nil
				case "ctrl+u":
					m.historyView.ClearSearch()
					return m, nil
				}
				var cmd tea.Cmd
				_, cmd = m.historyView.UpdateSearch(msg)
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}

			// ── Normal Navigation Mode ──
			switch key {
			case "1":
				cmds = append(cmds, m.switchTab(0)...)
			case "2":
				cmds = append(cmds, m.switchTab(1)...)
			case "3":
				cmds = append(cmds, m.switchTab(2)...)
			case "4":
				cmds = append(cmds, m.switchTab(3)...)
			case "5":
				cmds = append(cmds, m.switchTab(4)...)
			case "right", "l", "tab":
				cmds = append(cmds, m.switchTab(3)...)
			case "left", "h", "shift+tab":
				cmds = append(cmds, m.switchTab(1)...)
			case "/", "s":
				m.historyView.FocusSearch()
				return m, nil
			case "ctrl+u":
				m.historyView.ClearSearch()
				return m, nil
			case "c", "u", "p", "e":
				m.historyView.OpenCloudModal()
				return m, nil
			case "pgup", "b", "[":
				m.historyView.PrevPage()
			case "pgdown", "f", "space", "]":
				m.historyView.NextPage()
			case "up", "k":
				m.historyView.Prev()
			case "down", "j":
				m.historyView.Next()
			case "r":
				cmds = append(cmds, m.fetchHistoryCmd())
			case "q":
				return m, tea.Quit
			case "d", "enter":
				selected := m.historyView.SelectedItem()
				if selected != nil {
					m.queueMgr.Enqueue(selected.Name, selected.DownloadURL, selected.Token, m.cfg.DownloadDir, selected.Size, false, "", false)
					m.historyView.SetStatus(fmt.Sprintf("Enqueued '%s' → %s", selected.Name, m.cfg.DownloadDir), false)
					m.activeTab = 0
				}
			}

		case 3: // Cloud Keys
			if !m.cloudView.IsFocused() {
				switch key {
				case "1":
					cmds = append(cmds, m.switchTab(0)...)
				case "2":
					cmds = append(cmds, m.switchTab(1)...)
				case "3":
					cmds = append(cmds, m.switchTab(2)...)
				case "4":
					cmds = append(cmds, m.switchTab(3)...)
				case "5":
					cmds = append(cmds, m.switchTab(4)...)
				case "left", "h", "shift+tab", "[":
					cmds = append(cmds, m.switchTab(2)...)
				case "right", "l", "tab", "]":
					cmds = append(cmds, m.switchTab(4)...)
				case "enter", "i", "down", "up":
					m.cloudView.NextInput()
					return m, nil
				case "esc", "q":
					cmds = append(cmds, m.switchTab(0)...)
					return m, tea.Batch(cmds...)
				}
				return m, nil
			}

			// Focused mode
			if key == "esc" {
				m.cloudView.BlurAll()
				return m, nil
			}
			if key == "tab" || key == "down" {
				m.cloudView.NextInput()
				return m, nil
			}
			if key == "shift+tab" || key == "up" {
				m.cloudView.PrevInput()
				return m, nil
			}
			if key == "enter" || key == "ctrl+s" {
				cloudValues := m.cloudView.GetValues()
				apiClient := m.apiClient
				return m, func() tea.Msg {
					err := apiClient.SaveCloudConfig(cloudValues)
					if err != nil {
						return errMsg(err)
					}
					return cloudLoadedMsg(&cloudValues)
				}
			}
			var cmd tea.Cmd
			_, cmd = m.cloudView.Update(msg)
			cmds = append(cmds, cmd)

		case 4: // Settings
			if !m.settingsView.IsFocused() {
				switch key {
				case "1":
					cmds = append(cmds, m.switchTab(0)...)
				case "2":
					cmds = append(cmds, m.switchTab(1)...)
				case "3":
					cmds = append(cmds, m.switchTab(2)...)
				case "4":
					cmds = append(cmds, m.switchTab(3)...)
				case "5":
					cmds = append(cmds, m.switchTab(4)...)
				case "left", "h", "shift+tab", "[":
					cmds = append(cmds, m.switchTab(3)...)
				case "right", "l", "tab", "]":
					cmds = append(cmds, m.switchTab(0)...)
				case "enter", "i", "down", "up":
					m.settingsView.NextInput()
					return m, nil
				case "esc", "q":
					cmds = append(cmds, m.switchTab(0)...)
					return m, tea.Batch(cmds...)
				}
				return m, nil
			}

			// Focused mode
			if key == "esc" {
				m.settingsView.BlurAll()
				return m, nil
			}
			if key == "tab" || key == "down" {
				m.settingsView.NextInput()
				return m, nil
			}
			if key == "shift+tab" || key == "up" {
				m.settingsView.PrevInput()
				return m, nil
			}
			if key == "enter" || key == "ctrl+s" {
				newCfg, _ := m.settingsView.GetValues()
				if err := config.Save(newCfg); err != nil {
					m.settingsView.SetStatus("Failed to save: "+err.Error(), true)
				} else {
					m.cfg = newCfg
					m.apiClient = client.New(newCfg.ServerURL, newCfg.APIToken)
					m.queueMgr.UpdateAPIToken(newCfg.APIToken)
					m.queueMgr.UpdateMaxConcurrent(newCfg.MaxConcurrentDownloads)
					m.settingsView.SetStatus("Settings saved!", false)
					cmds = append(cmds, m.fetchHistoryCmd(), m.fetchCloudCmd())
				}
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			_, cmd = m.settingsView.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	var s strings.Builder

	// ── Top Bar ──
	logo := StyleLogo.Render("⚡ DISBOX CLI")
	serverBadge := StyleBadgeBlue.Render(" " + m.cfg.ServerURL + " ")

	activeCount := m.queueMgr.ActiveCount()
	queueInfo := ""
	if activeCount > 0 {
		queueInfo = " " + StyleBadgeYellow.Render(fmt.Sprintf(" %d active ", activeCount))
	}

	headerRow := lipgloss.JoinHorizontal(lipgloss.Center, logo, " ", serverBadge, queueInfo)
	s.WriteString(headerRow + "\n\n")

	// ── Tabs (track widths for mouse hit detection) ──
	tabTitles := []string{
		"◀ [F1] 📥 Queue",
		"[F2] ➕ Add",
		"[F3] 📜 History",
		"[F4] ☁️  Cloud Keys",
		"[F5] ⚙️  Settings ▶",
	}
	if m.activeTab == 0 {
		tabTitles[0] = "  [F1] 📥 Queue"
	}
	if m.activeTab == 4 {
		tabTitles[4] = "[F5] ⚙️  Settings  "
	}

	var renderedTabs []string
	widths := make([]int, len(tabTitles))
	for i, title := range tabTitles {
		var rendered string
		if i == m.activeTab {
			rendered = StyleTabActive.Render(title)
		} else {
			rendered = StyleTabInactive.Render(title)
		}
		renderedTabs = append(renderedTabs, rendered)
		widths[i] = lipgloss.Width(rendered)
	}
	m.tabWidths = widths
	s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...) + "\n")

	// ── Content ──
	switch m.activeTab {
	case 0:
		s.WriteString(m.downloadsView.Render(m.queueMgr.GetTasks(), m.width))
	case 1:
		s.WriteString(m.addView.Render(m.width))
	case 2:
		s.WriteString(m.historyView.Render(m.width))
	case 3:
		s.WriteString(m.cloudView.Render(m.width))
	case 4:
		s.WriteString(m.settingsView.Render(m.width))
	}

	// ── Footer (context-aware) ──
	var footer string
	switch m.activeTab {
	case 0:
		footer = fmt.Sprintf(
			"%s %s • %s %s • %s %s • %s",
			StyleKey.Render("Click / F1-F5:"), StyleValue.Render("Tabs"),
			StyleKey.Render("←/→:"), StyleValue.Render("Switch Tab"),
			StyleKey.Render("c:"), StyleValue.Render("Clear finished"),
			StyleKey.Render("q / Ctrl+C: Quit"),
		)
	case 1:
		if m.addView.IsFocused() {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s",
				StyleKey.Render("Ctrl+S / Alt+Enter / Ctrl+D:"), StyleValue.Render("Submit All"),
				StyleKey.Render("Enter:"), StyleValue.Render("New line"),
				StyleKey.Render("Esc:"), StyleValue.Render("Unfocus / Navigate"),
				StyleKey.Render("Click / F1-F5: Tabs"),
			)
		} else {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s",
				StyleKey.Render("Click box / Enter:"), StyleValue.Render("Edit Links"),
				StyleKey.Render("←/→:"), StyleValue.Render("Switch Tab"),
				StyleKey.Render("Ctrl+T / Ctrl+G:"), StyleValue.Render("Toggle Auto-Down/Cloud"),
				StyleKey.Render("Click / F1-F5: Tabs"),
			)
		}
	case 2:
		if m.historyView.IsModalOpen() {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s",
				StyleKey.Render("1-6 / Enter:"), StyleValue.Render("Dispatch to Cloud"),
				StyleKey.Render("z:"), StyleValue.Render("Toggle ZIP"),
				StyleKey.Render("Esc:"), StyleValue.Render("Cancel"),
				StyleKey.Render("F1-F5: Tabs"),
			)
		} else if m.historyView.IsSearchFocused() {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s",
				StyleKey.Render("Type Query:"), StyleValue.Render("Filter History"),
				StyleKey.Render("Enter / Esc:"), StyleValue.Render("Finish Search"),
				StyleKey.Render("Ctrl+U: Clear"),
			)
		} else {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s %s • %s",
				StyleKey.Render("↑↓:"), StyleValue.Render("Select"),
				StyleKey.Render("d/Enter:"), StyleValue.Render("Download"),
				StyleKey.Render("c:"), StyleValue.Render("Send to Cloud"),
				StyleKey.Render("/:"), StyleValue.Render("Search"),
				StyleKey.Render("←/→: Tabs"),
			)
		}
	case 3:
		if m.cloudView.IsFocused() {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s",
				StyleKey.Render("Tab / ↑↓:"), StyleValue.Render("Fields"),
				StyleKey.Render("Enter / Ctrl+S:"), StyleValue.Render("Save"),
				StyleKey.Render("Esc:"), StyleValue.Render("Unfocus / Navigate"),
				StyleKey.Render("Click / F1-F5: Tabs"),
			)
		} else {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s",
				StyleKey.Render("Click / Tab / Enter:"), StyleValue.Render("Edit Fields"),
				StyleKey.Render("←/→:"), StyleValue.Render("Switch Tab"),
				StyleKey.Render("Enter / Ctrl+S:"), StyleValue.Render("Save"),
				StyleKey.Render("Click / F1-F5: Tabs"),
			)
		}
	case 4:
		if m.settingsView.IsFocused() {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s",
				StyleKey.Render("Tab / ↑↓:"), StyleValue.Render("Fields"),
				StyleKey.Render("Enter / Ctrl+S:"), StyleValue.Render("Save"),
				StyleKey.Render("Esc:"), StyleValue.Render("Unfocus / Navigate"),
				StyleKey.Render("Click / F1-F5: Tabs"),
			)
		} else {
			footer = fmt.Sprintf(
				"%s %s • %s %s • %s %s • %s",
				StyleKey.Render("Click / Tab / Enter:"), StyleValue.Render("Edit Fields"),
				StyleKey.Render("←/→:"), StyleValue.Render("Switch Tab"),
				StyleKey.Render("Enter / Ctrl+S:"), StyleValue.Render("Save"),
				StyleKey.Render("Click / F1-F5: Tabs"),
			)
		}
	}
	s.WriteString("\n\n" + StyleHelp.Render(footer))

	return StyleApp.Render(s.String())
}

// handleTabClick detects which tab was clicked based on mouse X and Y coordinates.
func (m *AppModel) handleTabClick(x, y int) []tea.Cmd {
	if y < 2 || y > 5 || len(m.tabWidths) == 0 {
		return nil
	}

	adjX := x - 2
	if adjX < 0 {
		return nil
	}

	cumulative := 0
	for i, w := range m.tabWidths {
		if adjX >= cumulative && adjX < cumulative+w {
			return m.switchTab(i)
		}
		cumulative += w
	}
	return nil
}

// handleContentClick handles clicks inside the active view (e.g. history list, pagination, inputs).
func (m *AppModel) handleContentClick(x, y int) []tea.Cmd {
	// History tab clicks
	if m.activeTab == 2 {
		if m.historyView.IsModalOpen() {
			if y >= 11 && y <= 16 {
				idx := y - 11
				m.historyView.SelectCloudProvider(idx)
				provider, zip := m.historyView.SelectedCloudProvider()
				if cmd := m.dispatchSelectedToCloudCmd(provider, zip); cmd != nil {
					return []tea.Cmd{cmd}
				}
				return nil
			}
			if y >= 17 && y <= 19 {
				m.historyView.ToggleZip()
				return nil
			}
			return nil
		}

		// Search bar click around Y=8..10
		if y >= 8 && y <= 10 && x < 55 {
			m.historyView.FocusSearch()
			return nil
		}

		// History items start around Y=11
		if y >= 11 && y < 21 {
			row := y - 11
			m.historyView.SelectRow(row)
			return nil
		}
		// Pagination line around Y=21..24
		if y >= 21 && y <= 24 {
			if x < m.width/2 {
				m.historyView.PrevPage()
			} else {
				m.historyView.NextPage()
			}
			return nil
		}
	}

	// Add tab clicks (box focus, toggle download, toggle cloud, change cloud provider, submit button)
	if m.activeTab == 1 {
		if y >= 10 && y <= 18 {
			m.addView.Focus()
			return nil
		}
		if y >= 19 && y <= 20 {
			m.addView.ToggleAutoDownload()
			return nil
		}
		if y >= 21 && y <= 22 {
			m.addView.ToggleAutoCloud()
			return nil
		}
		if y >= 23 && y <= 24 && m.addView.autoCloud {
			if x < 40 {
				m.addView.CycleCloudProvider()
			} else {
				m.addView.ToggleCloudZip()
			}
			return nil
		}
		if y >= 25 {
			if submitCmd := m.submitLinksCmd(); submitCmd != nil {
				return []tea.Cmd{submitCmd}
			}
			return nil
		}
	}

	// Cloud tab input focus
	if m.activeTab == 3 {
		if y >= 8 {
			idx := (y - 8) / 3
			if idx >= 0 && idx < 6 {
				m.cloudView.SetFocus(idx)
			}
			return nil
		}
	}

	// Settings tab input focus
	if m.activeTab == 4 {
		if y >= 8 {
			idx := (y - 8) / 3
			if idx >= 0 && idx < 4 {
				m.settingsView.SetFocus(idx)
			}
			return nil
		}
	}

	return nil
}
