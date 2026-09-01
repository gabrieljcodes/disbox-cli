package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"disbox-cli/internal/downloader"
)

type DownloadsView struct {
	prog  progress.Model
	flash string
}

func NewDownloadsView() DownloadsView {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
		progress.WithWidth(26),
	)
	return DownloadsView{prog: p}
}

func (v DownloadsView) Render(tasks []downloader.DownloadTask, width, height int) string {
	var s strings.Builder

	s.WriteString(StyleHeader.Render("📥 Local Download Queue & Debrid Status") + "\n")

	if v.flash != "" {
		s.WriteString(StyleSuccess.Render("✓ "+v.flash) + "\n")
	}

	panelWidth := width - 6
	if panelWidth < 30 {
		panelWidth = 30
	}

	if len(tasks) == 0 {
		empty := StylePanel.Width(panelWidth).Render(
			"No active downloads in queue.\n\n" +
				"💡 Press " + StyleKey.Render("F2") + " or " + StyleKey.Render("→") + " to add new torrent / web links,\n" +
				"   or press " + StyleKey.Render("F3") + " to browse History and download previous files locally.",
		)
		s.WriteString(empty)
		return s.String()
	}

	// ── Stats Summary Bar ──
	var active, completed, failed int
	for _, t := range tasks {
		switch t.Status {
		case downloader.StatusDownloading, downloader.StatusCaching, downloader.StatusQueued:
			active++
		case downloader.StatusCompleted:
			completed++
		case downloader.StatusFailed:
			failed++
		}
	}

	summary := fmt.Sprintf(
		"%s %d active  %s %d done  %s %d failed  │  Press %s to clear",
		StyleBadgeBlue.Render("⬤"), active,
		StyleBadgeGreen.Render("⬤"), completed,
		StyleBadgeRed.Render("⬤"), failed,
		StyleKey.Render("c"),
	)

	var listBuilder strings.Builder
	listBuilder.WriteString(summary + "\n")

	divider := StyleHelp.Render(strings.Repeat("─", panelWidth-4))

	// Calculate how many tasks fit comfortably
	maxTasks := (height - 12) / 3
	if maxTasks < 2 {
		maxTasks = 2
	}
	displayTasks := tasks
	hiddenCount := 0
	if len(tasks) > maxTasks {
		displayTasks = tasks[:maxTasks]
		hiddenCount = len(tasks) - maxTasks
	}

	for i, task := range displayTasks {
		statusBadge := ""
		switch task.Status {
		case downloader.StatusDownloading:
			statusBadge = StyleBadgeBlue.Render(" DOWNLOADING ")
		case downloader.StatusCompleted:
			statusBadge = StyleBadgeGreen.Render(" COMPLETED ")
		case downloader.StatusCaching:
			statusBadge = StyleBadgeYellow.Render(" CACHING ")
		case downloader.StatusQueued:
			statusBadge = StyleBadgeYellow.Render(" QUEUED ")
		case downloader.StatusFailed:
			statusBadge = StyleBadgeRed.Render(" FAILED ")
		default:
			statusBadge = StyleBadgeYellow.Render(" " + string(task.Status) + " ")
		}

		pct := 0.0
		if task.TotalBytes > 0 {
			pct = float64(task.DownloadedBytes) / float64(task.TotalBytes)
		}
		if task.Status == downloader.StatusCompleted {
			pct = 1.0
		}

		// Truncate name if too long
		name := task.Name
		maxNameWidth := panelWidth - 40
		if maxNameWidth < 20 {
			maxNameWidth = 20
		}
		if len(name) > maxNameWidth {
			name = name[:maxNameWidth-3] + "..."
		}

		sizeStr := fmt.Sprintf("%s / %s", FormatBytes(task.DownloadedBytes), FormatBytes(task.TotalBytes))
		if task.TotalBytes <= 0 && task.DownloadedBytes > 0 {
			sizeStr = FormatBytes(task.DownloadedBytes)
		}

		// Line 1: [1] STATUS Name (Size)
		line1 := fmt.Sprintf(
			"[%d] %s %s %s",
			i+1,
			statusBadge,
			StyleHeader.Render(name),
			StyleValue.Render("("+sizeStr+")"),
		)

		// Line 2: Progress bar & Speed/ETA
		var line2 string
		if task.Status == downloader.StatusCaching {
			debridPct := task.DebridProgress
			debridBar := v.prog.ViewAs(debridPct / 100.0)
			line2 = fmt.Sprintf(
				"    %s  %s",
				debridBar,
				StyleValue.Render(fmt.Sprintf("Debrid: %.0f%%", debridPct)),
			)
		} else {
			progressBar := v.prog.ViewAs(pct)
			speedStr := FormatSpeed(task.SpeedBps)
			etaStr := FormatETA(task.ETA)
			line2 = fmt.Sprintf(
				"    %s  %s  %s  (ETA: %s)",
				progressBar,
				StyleValue.Render(fmt.Sprintf("%5.1f%%", pct*100)),
				StyleValue.Render(speedStr),
				StyleValue.Render(etaStr),
			)
		}

		// Line 3: Destination & Cloud status
		destLine := fmt.Sprintf("    📁 %s", StyleHelp.Render(task.DestDir))
		if task.CloudStatus != "" {
			destLine += fmt.Sprintf(" • %s", StyleBadgeYellow.Render(" "+task.CloudStatus+" "))
		}

		listBuilder.WriteString(line1 + "\n" + line2 + "\n" + destLine + "\n")

		if task.ErrorMessage != "" && task.Status == downloader.StatusFailed {
			listBuilder.WriteString("    " + StyleError.Render("❌ Error: "+task.ErrorMessage) + "\n")
		} else if task.ErrorMessage != "" && task.Status == downloader.StatusCaching {
			listBuilder.WriteString("    " + StyleValue.Render("⏳ "+task.ErrorMessage) + "\n")
		}

		// Add separator between tasks
		if i < len(displayTasks)-1 {
			listBuilder.WriteString(divider + "\n")
		}
	}

	if hiddenCount > 0 {
		listBuilder.WriteString("\n" + StyleHelp.Render(fmt.Sprintf("... and %d more task(s) in queue", hiddenCount)))
	}

	s.WriteString(StylePanel.Width(panelWidth).Render(listBuilder.String()))
	return s.String()
}
