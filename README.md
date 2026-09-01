# Disbox CLI (TUI)

Interactive Terminal User Interface for **Disbox** built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Features

- 📥 **Interactive Queue Manager**: Concurrent downloads to local folder with smart readiness polling (TorBox / Debrid CDN).
- ➕ **Batch & Multi-Link Add**: Paste multiple magnets, torrents, and WebDL links with auto-download and auto-cloud dispatch.
- 📜 **Download History with Live Search**: Filter history in real time by file name or type, download directly, or dispatch to cloud.
- ☁️ **Cloud Integrations**: Send items directly to PixelDrain, GoFile, 1Fichier, Google Drive, Dropbox, or OneDrive.
- ⚙️ **Configurable Settings**: Configure server URL, API token, max concurrent downloads, and destination directory.
- 🖱️ **Full Mouse Support & Keyboard Navigation**: Clickable tabs, buttons, checkboxes, input focus, scroll wheel, and arrow key (`←` `→`) navigation.

## Installation

```bash
cd /home/baka/Projects/disbox-cli
go build -o bin/disbox-cli ./cmd/disbox-cli
sudo cp bin/disbox-cli /usr/local/bin/
```

## Usage

Simply run:

```bash
disbox-cli
```

### Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| `F1` - `F5` / `Alt+1` - `Alt+5` | Switch between tabs |
| `←` / `→` | Switch tabs (when inputs are unfocused) |
| `Click` | Click tabs, buttons, history items, checkboxes |
| `Ctrl+C` | Quit CLI |

#### History Tab (`F3`)
- `↑` / `↓` or `j` / `k` / `Mouse Wheel`: Select history item
- `d` / `Enter`: Download selected item to PC
- `c` / `p`: Open Cloud Dispatch selector (PixelDrain, GoFile, 1Fichier, GDrive, Dropbox, OneDrive)
- `/` or `s`: Open search bar to filter history in real time
- `PgUp` / `PgDown` or `[` / `]`: Navigate pages

#### Add Tab (`F2`)
- `Click Box` / `Enter`: Focus textarea to type/paste links
- `Ctrl+S` / `Ctrl+D`: Submit all links
- `Ctrl+T`: Toggle auto local download
- `Ctrl+G`: Toggle auto cloud dispatch
- `p`: Cycle auto-cloud destination provider
- `z`: Toggle cloud ZIP archive option
- `Esc`: Unfocus textarea to navigate tabs
