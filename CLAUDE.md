# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Slidetty is a TUI (Terminal User Interface) slideshow application built in Go using the Bubble Tea framework. It renders markdown slides with beautiful styling via Glamour and provides an intuitive presentation interface.

## Architecture

### Core Components
- **Single File Architecture**: The entire application is contained in `main.go` (~255 lines)
- **Bubble Tea Model**: Uses the Model-View-Update (MVU) pattern via `github.com/charmbracelet/bubbletea`
- **Markdown Rendering**: Powered by `github.com/charmbracelet/glamour` with auto-styling
- **Progress Bar**: Animated gradient progress bar using `github.com/charmbracelet/bubbles/progress`
- **Styling**: UI styling via `github.com/charmbracelet/lipgloss`

### Key Data Structures
- `model` struct contains slides array, current slide index, glamour renderer, progress bar, window dimensions, and title
- Slide loading is asynchronous using Tea's command pattern (`loadSlides()` function)
- Window size changes trigger renderer updates with dynamic word wrapping

## Development Commands

### Build
```bash
go build -o slidetty main.go
```

### Run
```bash
./slidetty
```

### Install Dependencies
```bash
go mod download
```

### Clean Build
```bash
rm -f slidetty && go build -o slidetty main.go
```

## Slide Management

### Slide Directory Structure
- All slides live in `slides/` directory
- Regular slides: `##-name.md` (e.g., `01-welcome.md`, `02-navigation.md`)
- Special files:
  - `_title.md`: Contains presentation title (appears in status bar)
  - `_author.md`: Author information (not currently used by app)
  - Files starting with `_` are excluded from slide sequence

### Slide Loading Logic
- Slides are loaded alphabetically by filename
- Only `.md` files are processed
- Files beginning with underscore are skipped in main sequence
- Content is read into memory at startup

## Key Navigation
- `→` or `l`: Next slide
- `←` or `h`: Previous slide
- `q` or `Ctrl+C`: Quit application

## UI Layout
- **Content Area**: Dynamically sized based on terminal dimensions
- **Status Bar**: Shows "Slide X/Y" and presentation title with blue background
- **Progress Bar**: Animated gradient bar at bottom showing presentation progress
- **Responsive**: Word wrapping and layout adjust to terminal size changes

## Dependencies
The application uses several Charm libraries:
- `bubbletea`: TUI framework and MVU architecture
- `glamour`: Markdown rendering with syntax highlighting
- `lipgloss`: Styling and layout
- `bubbles/progress`: Animated progress bar component