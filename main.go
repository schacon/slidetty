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

	// Calculate percentage
	percentage := float64(m.currentSlide+1) / float64(len(m.slides))

	// Create filled and empty parts of progress bar
	filledWidth := int(float64(m.width) * percentage)
	if filledWidth > m.width {
		filledWidth = m.width
	}

	progressBarFilled := lipgloss.NewStyle().
		Background(lipgloss.Color("39")). // Bright blue
		Width(filledWidth)

	progressBarEmpty := lipgloss.NewStyle().
		Background(lipgloss.Color("240")). // Dark gray
		Width(m.width - filledWidth)

	progressBar := lipgloss.JoinHorizontal(lipgloss.Left,
		progressBarFilled.Render(strings.Repeat(" ", filledWidth)),
		progressBarEmpty.Render(strings.Repeat(" ", m.width-filledWidth)))

	// Create status line
	slideInfo := fmt.Sprintf("Slide %d/%d", m.currentSlide+1, len(m.slides))
	percentageStr := fmt.Sprintf("%.0f%%", percentage*100)
	help := "← → navigate • q quit"

	statusStyle := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("15")).
		Padding(0, 1)

	// Create status content with proper alignment
	statusLeft := slideInfo
	statusCenter := percentageStr
	statusRight := help

	// Calculate spacing
	totalTextWidth := len(statusLeft) + len(statusCenter) + len(statusRight)
	availableSpace := m.width - 4 - totalTextWidth // Account for padding
	if availableSpace < 0 {
		availableSpace = 0
	}

	leftSpacing := availableSpace / 2
	rightSpacing := availableSpace - leftSpacing

	statusContent := fmt.Sprintf("%s%s%s%s%s",
		statusLeft,
		strings.Repeat(" ", leftSpacing),
		statusCenter,
		strings.Repeat(" ", rightSpacing),
		statusRight)

	statusLine := statusStyle.Render(statusContent)

	return content + "\n" + progressBar + statusLine
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}