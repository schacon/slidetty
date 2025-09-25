package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	slides         []string
	slidePaths     []string
	currentSlide   int
	renderer       *glamour.TermRenderer
	progress       progress.Model
	width          int
	height         int
	title          string
	author         string
	err            error
	revealConfigs  []revealConfig
	revealProgress map[int]int
	showEditor     bool
	editor         textarea.Model
	editorPath     string
}

type errMsg error

type slideReloadedMsg struct {
	slideIndex int
	content    string
	config     revealConfig
	path       string
}

type revealConfig struct {
	directiveLines []int
	items          [][]int
}

func (rc revealConfig) totalItems() int {
	return len(rc.items)
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
		slides:         []string{},
		slidePaths:     []string{},
		currentSlide:   0,
		renderer:       r,
		progress:       prog,
		title:          "",
		author:         "",
		revealConfigs:  nil,
		revealProgress: make(map[int]int),
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
	var configs []revealConfig
	var paths []string

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
		fullPath := filepath.Join("slides", filename)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return errMsg(err)
		}
		slide := string(content)
		slides = append(slides, slide)
		configs = append(configs, analyzeReveal(slide))
		paths = append(paths, fullPath)
	}

	return slidesLoadedMsg{slides: slides, title: title, author: author, revealConfigs: configs, paths: paths}
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

		slide := string(content)
		return slideReloadedMsg{slideIndex: slideIndex, content: slide, config: analyzeReveal(slide), path: filepath.Join("slides", filenames[slideIndex])}
	}
}

type slidesLoadedMsg struct {
	slides        []string
	paths         []string
	title         string
	author        string
	revealConfigs []revealConfig
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showEditor {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			if m.renderer != nil {
				theme := loadTheme()
				var r *glamour.TermRenderer
				if theme == "auto" {
					r, _ = glamour.NewTermRenderer(
						glamour.WithAutoStyle(),
						glamour.WithWordWrap(msg.Width-4),
					)
				} else {
					r, _ = glamour.NewTermRenderer(
						glamour.WithStylePath(theme),
						glamour.WithWordWrap(msg.Width-4),
					)
				}
				m.renderer = r
			}
			m.progress.Width = msg.Width - 4
			_, _, editorW, editorH := editorGeometry(msg.Width, msg.Height)
			m.editor.SetWidth(editorW)
			m.editor.SetHeight(editorH)
			return m, nil

		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				m.editor.Blur()
				m.showEditor = false
				m.editorPath = ""
				return m, nil
			case tea.KeyCtrlS:
				content := m.editor.Value()
				if m.editorPath != "" {
					if err := os.WriteFile(m.editorPath, []byte(content), 0o644); err != nil {
						m.err = err
						return m, nil
					}
				}
				if m.currentSlide >= 0 && m.currentSlide < len(m.slides) {
					m.slides[m.currentSlide] = content
					if len(m.revealConfigs) != len(m.slides) {
						newConfigs := make([]revealConfig, len(m.slides))
						copy(newConfigs, m.revealConfigs)
						m.revealConfigs = newConfigs
					}
					cfg := analyzeReveal(content)
					m.revealConfigs[m.currentSlide] = cfg
					if cfg.totalItems() > 0 {
						if m.revealProgress == nil {
							m.revealProgress = make(map[int]int)
						}
						m.revealProgress[m.currentSlide] = clampRevealProgress(m.revealProgress[m.currentSlide], cfg.totalItems())
					} else {
						delete(m.revealProgress, m.currentSlide)
					}
				}
				if m.currentSlide >= 0 && m.currentSlide < len(m.slidePaths) && m.editorPath != "" {
					m.slidePaths[m.currentSlide] = m.editorPath
				}
				m.err = nil
				return m, nil
			}
			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(msg)
			return m, cmd
		case errMsg:
			m.err = msg
			return m, nil
		default:
			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(msg)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.renderer != nil {
			theme := loadTheme()
			var r *glamour.TermRenderer
			if theme == "auto" {
				r, _ = glamour.NewTermRenderer(
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(msg.Width-4),
				)
			} else {
				r, _ = glamour.NewTermRenderer(
					glamour.WithStylePath(theme),
					glamour.WithWordWrap(msg.Width-4),
				)
			}
			m.renderer = r
		}
		m.progress.Width = msg.Width - 4
		return m, nil

	case slidesLoadedMsg:
		m.slides = msg.slides
		m.slidePaths = msg.paths
		m.title = msg.title
		m.author = msg.author
		m.revealConfigs = msg.revealConfigs
		m.revealProgress = make(map[int]int, len(msg.revealConfigs))
		for idx, cfg := range msg.revealConfigs {
			if cfg.totalItems() > 0 {
				m.revealProgress[idx] = 1
			}
		}
		if len(m.slides) == 0 {
			return m, nil
		}
		if m.currentSlide >= len(m.slides) {
			m.currentSlide = len(m.slides) - 1
		}
		percentage := float64(m.currentSlide+1) / float64(len(m.slides))
		m.progress.SetPercent(percentage)
		return m, nil

	case slideReloadedMsg:
		if msg.slideIndex >= 0 && msg.slideIndex < len(m.slides) {
			m.slides[msg.slideIndex] = msg.content
			if len(m.revealConfigs) != len(m.slides) {
				newConfigs := make([]revealConfig, len(m.slides))
				copy(newConfigs, m.revealConfigs)
				m.revealConfigs = newConfigs
			}
			if len(m.slidePaths) != len(m.slides) {
				newPaths := make([]string, len(m.slides))
				copy(newPaths, m.slidePaths)
				m.slidePaths = newPaths
			}
			m.revealConfigs[msg.slideIndex] = msg.config
			if msg.path != "" {
				m.slidePaths[msg.slideIndex] = msg.path
			}
			current, ok := m.revealProgress[msg.slideIndex]
			total := msg.config.totalItems()
			minVisible := 0
			if total > 0 {
				minVisible = 1
			}
			if !ok {
				current = minVisible
			}
			if current < minVisible {
				current = minVisible
			}
			if current > total {
				current = total
			}
			if total == 0 {
				delete(m.revealProgress, msg.slideIndex)
			} else {
				m.revealProgress[msg.slideIndex] = current
			}
		}
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "e":
			if len(m.slides) == 0 || m.currentSlide < 0 || m.currentSlide >= len(m.slides) {
				return m, nil
			}
			_, _, editorW, editorH := editorGeometry(m.width, m.height)
			editor := textarea.New()
			editor.SetValue(m.slides[m.currentSlide])
			editor.Placeholder = "Edit slide markdown..."
			editor.Focus()
			editor.SetWidth(editorW)
			editor.SetHeight(editorH)
			m.editor = editor
			if m.currentSlide >= 0 && m.currentSlide < len(m.slidePaths) {
				m.editorPath = m.slidePaths[m.currentSlide]
			} else {
				m.editorPath = ""
			}
			m.showEditor = true
			return m, textarea.Blink

		case "r":
			if len(m.slides) > 0 {
				return m, reloadSlide(m.currentSlide)
			}
			return m, nil

		case "down", "j":
			if adjustReveal(&m, m.currentSlide, 1) {
				return m, nil
			}
			if m.currentSlide < len(m.slides)-1 {
				m.currentSlide++
				percentage := float64(m.currentSlide+1) / float64(len(m.slides))
				cmd := m.progress.SetPercent(percentage)
				return m, cmd
			}
			return m, nil

		case "up", "k":
			if adjustReveal(&m, m.currentSlide, -1) {
				return m, nil
			}
			if m.currentSlide > 0 {
				m.currentSlide--
				percentage := float64(m.currentSlide+1) / float64(len(m.slides))
				cmd := m.progress.SetPercent(percentage)
				return m, cmd
			}
			return m, nil

		case "right", "l":
			if m.currentSlide < len(m.slides)-1 {
				m.currentSlide++
				percentage := float64(m.currentSlide+1) / float64(len(m.slides))
				cmd := m.progress.SetPercent(percentage)
				return m, cmd
			}
			return m, nil

		case "left", "h":
			if m.currentSlide > 0 {
				m.currentSlide--
				percentage := float64(m.currentSlide+1) / float64(len(m.slides))
				cmd := m.progress.SetPercent(percentage)
				return m, cmd
			}
			return m, nil
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func adjustReveal(m *model, slideIndex, delta int) bool {
	if slideIndex < 0 || slideIndex >= len(m.revealConfigs) {
		return false
	}
	cfg := m.revealConfigs[slideIndex]
	total := cfg.totalItems()
	if m.revealProgress == nil {
		m.revealProgress = make(map[int]int)
	}
	current, ok := m.revealProgress[slideIndex]
	minVisible := 0
	if total > 0 {
		minVisible = 1
	}
	if !ok {
		current = minVisible
		if total > 0 {
			m.revealProgress[slideIndex] = current
		}
	}
	next := current + delta
	if next < minVisible {
		next = minVisible
	}
	if next > total {
		next = total
	}
	if next == current {
		return false
	}
	if total == 0 {
		return false
	}
	m.revealProgress[slideIndex] = next
	return true
}

func clampRevealProgress(current, total int) int {
	if total <= 0 {
		return 0
	}
	if current < 1 {
		return 1
	}
	if current > total {
		return total
	}
	return current
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func editorGeometry(width, height int) (panelWidth, panelHeight, editorWidth, editorHeight int) {
	const (
		minPanelWidth  = 28
		minPanelHeight = 10
		panelHorizPad  = 4
		panelVertPad   = 4
	)

	if width < minPanelWidth+2 {
		width = minPanelWidth + 2
	}
	if height < minPanelHeight+2 {
		height = minPanelHeight + 2
	}

	targetPanelWidth := int(math.Round(float64(width) * 0.7))
	maxPanelWidth := width - 4
	if maxPanelWidth < minPanelWidth {
		maxPanelWidth = width - 2
		if maxPanelWidth < minPanelWidth {
			maxPanelWidth = width
		}
	}
	panelWidth = clampInt(targetPanelWidth, minPanelWidth, maxPanelWidth)

	targetPanelHeight := int(math.Round(float64(height) * 0.4))
	maxPanelHeight := height - 12
	if maxPanelHeight < minPanelHeight {
		maxPanelHeight = height - 8
		if maxPanelHeight < minPanelHeight {
			maxPanelHeight = height - 4
		}
	}
	panelHeight = clampInt(targetPanelHeight, minPanelHeight, maxPanelHeight)

	editorWidth = clampInt(panelWidth-panelHorizPad, 12, panelWidth-2)
	editorHeight = clampInt(panelHeight-panelVertPad, 6, panelHeight-2)

	return panelWidth, panelHeight, editorWidth, editorHeight
}

func applyReveal(content string, cfg revealConfig, count int) string {
	total := cfg.totalItems()
	if total == 0 && len(cfg.directiveLines) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	hide := make(map[int]struct{}, len(cfg.directiveLines))
	for _, idx := range cfg.directiveLines {
		hide[idx] = struct{}{}
	}
	visible := count
	if visible < 0 {
		visible = 0
	}
	if total > 0 && visible == 0 {
		visible = 1
	}
	if visible > len(cfg.items) {
		visible = len(cfg.items)
	}
	for _, item := range cfg.items[visible:] {
		for _, idx := range item {
			hide[idx] = struct{}{}
		}
	}
	filtered := make([]string, 0, len(lines)+1)
	for i, line := range lines {
		if _, hidden := hide[i]; hidden {
			continue
		}
		filtered = append(filtered, line)
	}
	if total > 0 && visible < len(cfg.items) {
		nextIndices := cfg.items[visible]
		if len(nextIndices) > 0 && nextIndices[0] < len(lines) {
			filtered = append(filtered, ellipsisLine(lines[nextIndices[0]]))
		} else {
			filtered = append(filtered, "...")
		}
	}
	return strings.Join(filtered, "\n")
}

func ellipsisLine(line string) string {
	trimmed := strings.TrimLeft(line, " 	")
	indent := line[:len(line)-len(trimmed)]
	switch {
	case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "), strings.HasPrefix(trimmed, "+ "):
		return indent + trimmed[:2] + "..."
	default:
		digits := 0
		for digits < len(trimmed) && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
			digits++
		}
		if digits > 0 && digits < len(trimmed) && trimmed[digits] == '.' {
			if digits+1 < len(trimmed) && trimmed[digits+1] == ' ' {
				return indent + trimmed[:digits+2] + "..."
			}
		}
	}
	return indent + "..."
}

func analyzeReveal(content string) revealConfig {
	lines := strings.Split(content, "\n")
	var directive []int
	var items [][]int

	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) != ":reveal:" {
			i++
			continue
		}
		directive = append(directive, i)
		i++
		for i < len(lines) {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				i++
				continue
			}
			if !isListItem(lines[i]) {
				break
			}
			itemIndices := []int{i}
			i++
			for i < len(lines) {
				line := lines[i]
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine == "" {
					itemIndices = append(itemIndices, i)
					i++
					break
				}
				if isListItem(line) {
					break
				}
				if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "	") {
					itemIndices = append(itemIndices, i)
					i++
					continue
				}
				break
			}
			items = append(items, itemIndices)
		}
	}

	return revealConfig{directiveLines: directive, items: items}
}

func isListItem(line string) bool {
	trimmed := strings.TrimLeft(line, " 	")
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	idx := 0
	for idx < len(trimmed) && trimmed[idx] >= '0' && trimmed[idx] <= '9' {
		idx++
	}
	if idx == 0 {
		return false
	}
	if idx < len(trimmed) && trimmed[idx] == '.' {
		if idx+1 < len(trimmed) && trimmed[idx+1] == ' ' {
			return true
		}
	}
	return false
}

func (m model) View() string {
	if m.showEditor {
		panelWidth, panelHeight, editorW, editorH := editorGeometry(m.width, m.height)
		editorCopy := m.editor
		editorCopy.SetWidth(editorW)
		editorCopy.SetHeight(editorH)
		editorView := editorCopy.View()
		pathLabel := m.editorPath
		if pathLabel == "" {
			pathLabel = "unsaved slide"
		} else {
			pathLabel = filepath.Base(pathLabel)
		}
		helpLines := []string{pathLabel, "esc to close - ctrl+s to save"}
		if m.err != nil {
			helpLines = append(helpLines, fmt.Sprintf("error: %v", m.err))
		}
		helpText := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render(strings.Join(helpLines, "\n"))
		body := lipgloss.JoinVertical(lipgloss.Left, editorView, helpText)
		panelStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7C3AED")).
			Background(lipgloss.Color("#0F172A")).
			Padding(1, 2).
			Width(panelWidth).
			Height(panelHeight)
		panel := panelStyle.Render(body)

		floating := lipgloss.Place(
			m.height,
			m.width,
			lipgloss.Center,
			lipgloss.Center,
			panel,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("#111827")),
		)

		return floating
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'q' to quit.", m.err)
	}

	if len(m.slides) == 0 {
		return "Loading slides...\n\nPress 'q' to quit."
	}

	// Render current slide with glamour
	slideContent := m.slides[m.currentSlide]
	if m.currentSlide < len(m.revealConfigs) {
		slideContent = applyReveal(slideContent, m.revealConfigs[m.currentSlide], m.revealProgress[m.currentSlide])
	}
	rendered, err := m.renderer.Render(slideContent)
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
