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
	err          error
}

type errMsg error

func initialModel() model {
	// Initialize glamour renderer with dark theme
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	// Initialize progress bar with gradient
	prog := progress.New(progress.WithDefaultGradient())

	return model{
		slides:       []string{},
		currentSlide: 0,
		renderer:     r,
		progress:     prog,
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
		// Update progress bar width
		m.progress.Width = msg.Width - 4 // Leave some margin
		return m, nil

	case slidesLoadedMsg:
		m.slides = msg.slides
		// Set initial progress percentage
		if len(m.slides) > 0 {
			percentage := float64(m.currentSlide+1) / float64(len(m.slides))
			m.progress.SetPercent(percentage)
		}
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

	// Calculate percentage
	percentage := float64(m.currentSlide+1) / float64(len(m.slides))

	// Get the animated gradient progress bar
	progressBar := m.progress.View()

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

	// Calculate available width (account for padding)
	availableWidth := m.width - 4 // Account for padding (2 on each side)
	totalTextWidth := len(statusLeft) + len(statusCenter) + len(statusRight)

	// If text is too long, truncate the help text
	if totalTextWidth > availableWidth {
		maxHelpWidth := availableWidth - len(statusLeft) - len(statusCenter) - 4 // Reserve 4 spaces for spacing
		if maxHelpWidth < 10 {
			statusRight = "q quit"
		} else if maxHelpWidth < len(statusRight) {
			statusRight = statusRight[:maxHelpWidth-3] + "..."
		}
	}

	// Recalculate after potential truncation
	totalTextWidth = len(statusLeft) + len(statusCenter) + len(statusRight)
	remainingSpace := availableWidth - totalTextWidth

	var statusContent string
	if remainingSpace > 0 {
		leftSpacing := remainingSpace / 2
		rightSpacing := remainingSpace - leftSpacing
		statusContent = fmt.Sprintf("%s%s%s%s%s",
			statusLeft,
			strings.Repeat(" ", leftSpacing+1), // Add 1 for minimum spacing
			statusCenter,
			strings.Repeat(" ", rightSpacing+1), // Add 1 for minimum spacing
			statusRight)
	} else {
		// Minimal spacing if very tight
		statusContent = fmt.Sprintf("%s %s %s", statusLeft, statusCenter, statusRight)
	}

	statusLine := statusStyle.Render(statusContent)

	return content + "\n" + statusLine + "\n" + progressBar
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
