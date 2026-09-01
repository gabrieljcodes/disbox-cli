package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"disbox-cli/internal/config"
	"disbox-cli/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	}

	app := tui.NewAppModel(cfg)
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running disbox-cli: %v\n", err)
		os.Exit(1)
	}
}
