package tui

import (
	"strings"
	"testing"

	"disbox-cli/internal/config"
)

func TestAppModel_ViewHeights(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NewAppModel(cfg)

	testCases := []struct {
		width  int
		height int
	}{
		{width: 80, height: 24},
		{width: 100, height: 30},
		{width: 80, height: 20},
		{width: 120, height: 40},
		{width: 60, height: 22},
	}

	for _, tc := range testCases {
		app.width = tc.width
		app.height = tc.height

		for tab := 0; tab < 5; tab++ {
			app.activeTab = tab
			rendered := app.View()
			lines := strings.Split(rendered, "\n")
			if len(lines) > tc.height {
				t.Errorf("For size %dx%d on tab %d, rendered line count %d exceeded height %d",
					tc.width, tc.height, tab, len(lines), tc.height)
			}
		}
	}
}

func TestAppModel_TabClicks(t *testing.T) {
	cfg := config.DefaultConfig()
	app := NewAppModel(cfg)

	tabWidths := app.getTabWidths()
	if len(tabWidths) != 5 {
		t.Fatalf("expected 5 tab widths, got %d", len(tabWidths))
	}

	cumulative := 1
	for i, w := range tabWidths {
		clickX := cumulative + (w / 2)
		clickY := 2 // middle line of tab border

		app.handleTabClick(clickX, clickY)
		if app.activeTab != i {
			t.Errorf("Clicking at X=%d, Y=%d did not switch to tab %d (current activeTab=%d)", clickX, clickY, i, app.activeTab)
		}
		cumulative += w
	}
}
