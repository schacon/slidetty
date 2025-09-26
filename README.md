# Slidetty

A beautiful TUI slideshow application built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Glamour](https://github.com/charmbracelet/glamour).

## Features

- 🎨 Beautiful markdown rendering with syntax highlighting
- ⌨️ Intuitive keyboard navigation
- 📱 Responsive terminal UI that adapts to window size
- 🚀 Fast and lightweight

## Usage

### Installation

```bash
go build -o slidetty main.go
```

### Initializing

You can run `slidetty init` to create a `slides` directory with example slides.

### Running

Place your markdown slides in the `slides/` directory and run:

```bash
./slidetty
```

The application will automatically load all `.md` files from the `slides/` directory in alphabetical order.

### Controls

- `→` or `l` - Next slide
- `←` or `h` - Previous slide
- `q` or `Ctrl+C` - Quit

### Slide Format

Each slide is a separate markdown file in the `slides/` directory. The application supports full markdown syntax including:

- Headers
- **Bold** and *italic* text
- Code blocks with syntax highlighting
- Lists
- And more!

Example slide structure:
```
slides/
├── 01-welcome.md
├── 02-features.md
└── 03-conclusion.md
```

## Dependencies

- [github.com/charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [github.com/charmbracelet/glamour](https://github.com/charmbracelet/glamour) - Markdown renderer
- [github.com/charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions
