package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	slides       []string
	currentSlide int
	renderer     *glamour.TermRenderer
	width        int
	height       int
	err          error
}

type errMsg error

func initialModel() model {
	// Initialize glamour renderer with dark theme
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return model{
		slides:       []string{},
		currentSlide: 0,
		renderer:     r,
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

	// Collect markdown files
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".md" {
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

	return slidesLoadedMsg{slides: slides}
}

type slidesLoadedMsg struct {
	slides []string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update renderer word wrap based on terminal width
		if m.renderer != nil {
			r, _ := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(msg.Width-4), // Leave some margin
			)
			m.renderer = r
		}
		return m, nil

	case slidesLoadedMsg:
		m.slides = msg.slides
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "right", "l":
			if m.currentSlide < len(m.slides)-1 {
				m.currentSlide++
			}
			return m, nil

		case "left", "h":
			if m.currentSlide > 0 {
				m.currentSlide--
			}
			return m, nil
		}
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

	// Calculate available height for content (reserve 1 line for bottom bar)
	contentHeight := m.height - 1

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

	// Create bottom bar styles
	bottomBarStyle := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	// Calculate percentage
	percentage := float64(m.currentSlide+1) / float64(len(m.slides)) * 100

	// Create progress bar
	barWidth := m.width - 30 // Reserve space for text
	if barWidth < 0 {
		barWidth = 10
	}
	filled := int(float64(barWidth) * percentage / 100)
	if filled > barWidth {
		filled = barWidth
	}

	progressBar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// Format bottom bar content
	slideInfo := fmt.Sprintf("Slide %d/%d", m.currentSlide+1, len(m.slides))
	percentageStr := fmt.Sprintf("%.0f%%", percentage)
	help := "← → navigate • q quit"

	// Layout bottom bar with proper spacing
	bottomContent := fmt.Sprintf("%s  %s  %s  %s",
		slideInfo,
		progressBar,
		percentageStr,
		help)

	// Truncate if too long
	if len(bottomContent) > m.width {
		bottomContent = bottomContent[:m.width]
	}

	bottomBar := bottomBarStyle.Render(bottomContent)

	return content + bottomBar
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}