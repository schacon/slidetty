package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	slides       []string
	currentSlide int
	renderer     *glamour.TermRenderer
	progress     progress.Model
	width        int
	height       int
	title        string
	author       string
	err          error
}

type errMsg error

type slideReloadedMsg struct {
	slideIndex int
	content    string
}

func loadTheme() string {
	if themeContent, err := os.ReadFile("slides/_theme.md"); err == nil {
		theme := strings.TrimSpace(string(themeContent))
		if theme != "" {
			return theme
		}
	}
	return "auto" // fallback to auto if no theme file or empty
}

func initialModel() model {
	// Initialize glamour renderer with theme from _theme.md
	theme := loadTheme()
	var r *glamour.TermRenderer
	if theme == "auto" {
		r, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(80),
		)
	} else {
		r, _ = glamour.NewTermRenderer(
			glamour.WithStylePath(theme),
			glamour.WithWordWrap(80),
		)
	}

	// Initialize progress bar with gradient
	prog := progress.New(progress.WithDefaultGradient())

	return model{
		slides:       []string{},
		currentSlide: 0,
		renderer:     r,
		progress:     prog,
		title:        "",
		author:       "",
	}
}

func (m model) Init() tea.Cmd {
	return loadSlides
}

func loadSlides() tea.Msg {
	files, err := os.ReadDir("slides")
	if err != nil {
		return errMsg(err)
	}

	var slides []string
	var filenames []string
	var title string
	var author string

	// Load title from _title.md if it exists
	if titleContent, err := os.ReadFile("slides/_title.md"); err == nil {
		title = strings.TrimSpace(string(titleContent))
	}

	// Load author from _author.md if it exists
	if authorContent, err := os.ReadFile("slides/_author.md"); err == nil {
		author = strings.TrimSpace(string(authorContent))
	}

	// Collect markdown files (excluding files starting with underscore)
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".md" && !strings.HasPrefix(file.Name(), "_") {
			filenames = append(filenames, file.Name())
		}
	}

	// Sort filenames to ensure consistent order
	sort.Strings(filenames)

	// Read file contents
	for _, filename := range filenames {
		content, err := os.ReadFile(filepath.Join("slides", filename))
		if err != nil {
			return errMsg(err)
		}
		slides = append(slides, string(content))
	}

	return slidesLoadedMsg{slides: slides, title: title, author: author}
}

func reloadSlide(slideIndex int) tea.Cmd {
	return func() tea.Msg {
		files, err := os.ReadDir("slides")
		if err != nil {
			return errMsg(err)
		}

		var filenames []string

		// Collect markdown files (excluding files starting with underscore)
		for _, file := range files {
			if filepath.Ext(file.Name()) == ".md" && !strings.HasPrefix(file.Name(), "_") {
				filenames = append(filenames, file.Name())
			}
		}

		// Sort filenames to ensure consistent order
		sort.Strings(filenames)

		// Check if slideIndex is valid
		if slideIndex < 0 || slideIndex >= len(filenames) {
			return errMsg(fmt.Errorf("invalid slide index: %d", slideIndex))
		}

		// Read the specific slide content
		content, err := os.ReadFile(filepath.Join("slides", filenames[slideIndex]))
		if err != nil {
			return errMsg(err)
		}

		return slideReloadedMsg{slideIndex: slideIndex, content: string(content)}
	}
}

type slidesLoadedMsg struct {
	slides []string
	title  string
	author string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update renderer word wrap based on terminal width
		if m.renderer != nil {
			theme := loadTheme()
			var r *glamour.TermRenderer
			if theme == "auto" {
				r, _ = glamour.NewTermRenderer(
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(msg.Width-4), // Leave some margin
				)
			} else {
				r, _ = glamour.NewTermRenderer(
					glamour.WithStylePath(theme),
					glamour.WithWordWrap(msg.Width-4), // Leave some margin
				)
			}
			m.renderer = r
		}
		// Update progress bar width
		m.progress.Width = msg.Width - 4 // Leave some margin
		return m, nil

	case slidesLoadedMsg:
		m.slides = msg.slides
		m.title = msg.title
		m.author = msg.author
		// Set initial progress percentage
		if len(m.slides) > 0 {
			percentage := float64(m.currentSlide+1) / float64(len(m.slides))
			m.progress.SetPercent(percentage)
		}
		return m, nil

	case slideReloadedMsg:
		if msg.slideIndex == m.currentSlide && msg.slideIndex < len(m.slides) {
			m.slides[msg.slideIndex] = msg.content
		}
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "r":
			if len(m.slides) > 0 {
				return m, reloadSlide(m.currentSlide)
			}
			return m, nil

		case "right", "l":
			if m.currentSlide < len(m.slides)-1 {
				m.currentSlide++
				// Update progress bar
				percentage := float64(m.currentSlide+1) / float64(len(m.slides))
				cmd := m.progress.SetPercent(percentage)
				return m, cmd
			}
			return m, nil

		case "left", "h":
			if m.currentSlide > 0 {
				m.currentSlide--
				// Update progress bar
				percentage := float64(m.currentSlide+1) / float64(len(m.slides))
				cmd := m.progress.SetPercent(percentage)
				return m, cmd
			}
			return m, nil
		}

	// FrameMsg is sent when the progress bar wants to animate itself
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'q' to quit.", m.err)
	}

	if len(m.slides) == 0 {
		return "Loading slides...\n\nPress 'q' to quit."
	}

	// Render current slide with glamour
	rendered, err := m.renderer.Render(m.slides[m.currentSlide])
	if err != nil {
		rendered = "Error rendering markdown: " + err.Error()
	}

	// Calculate available height for content (reserve 2 lines for bottom bar)
	contentHeight := m.height - 2

	// Split rendered content into lines and fit to available height
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}

	// Pad content to fill the available height
	content := strings.Join(lines, "\n")
	contentLines := len(lines)
	if contentLines < contentHeight {
		padding := strings.Repeat("\n", contentHeight-contentLines)
		content += padding
	}

	// Get the animated gradient progress bar
	progressBar := m.progress.View()

	// Create three-section status line with chevrons
	slideInfo := fmt.Sprintf("Slide %d/%d", m.currentSlide+1, len(m.slides))

	titleText := m.title
	if titleText == "" {
		titleText = "Slidetty"
	}

	authorText := m.author
	if authorText == "" {
		authorText = "Unknown"
	}

	// Define styles for the three sections
	leftStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#000080")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	centerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1E3A8A")).
		Foreground(lipgloss.Color("15")).
		PaddingLeft(1).
		PaddingRight(0)

	rightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#000080")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	// Chevron styles
	leftChevronStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1E3A8A")).
		Foreground(lipgloss.Color("#000080"))

	rightChevronStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#000080")).
		Foreground(lipgloss.Color("#1E3A8A"))

	// Calculate section widths (approximate thirds)
	totalWidth := m.width
	chevronWidth := 1
	sectionWidth := (totalWidth - 2*chevronWidth) / 3

	// Adjust for any remaining width
	leftWidth := sectionWidth
	centerWidth := sectionWidth
	rightWidth := totalWidth - leftWidth - centerWidth - 2*chevronWidth

	// Truncate text if needed
	if len(slideInfo) > leftWidth-2 {
		slideInfo = slideInfo[:leftWidth-5] + "..."
	}
	if len(authorText) > centerWidth-2 {
		authorText = authorText[:centerWidth-5] + "..."
	}
	if len(titleText) > rightWidth-2 {
		titleText = titleText[:rightWidth-5] + "..."
	}

	// Create sections with proper width and alignment
	leftSection := leftStyle.Width(leftWidth).Render(slideInfo)
	centerSection := centerStyle.Width(centerWidth).Align(lipgloss.Center).Render(authorText)
	rightSection := rightStyle.Width(rightWidth).Align(lipgloss.Right).Render(titleText)

	// Create chevrons using Nerd Font Powerline symbol U+E0B2
	leftChevron := leftChevronStyle.Render("\uE0B0")
	rightChevron := rightChevronStyle.Render("\uE0B0")

	// Combine all sections
	statusLine := lipgloss.JoinHorizontal(lipgloss.Top, leftSection, leftChevron, centerSection, rightChevron, rightSection)

	return content + "\n" + statusLine + "\n" + progressBar
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
