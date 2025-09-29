package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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
	commandBlocks  [][]string // commands for each slide
	notification   string
	notificationTimer int
}

type errMsg error

type tickMsg struct{}

type slideReloadedMsg struct {
	slideIndex    int
	content       string
	config        revealConfig
	path          string
	commandBlock  []string
}

type revealConfig struct {
	directiveLines []int
	items          [][]int
}

type commandBlock struct {
	commands []string
}

func (rc revealConfig) totalItems() int {
	return len(rc.items)
}

func loadTheme() string {
	// Try current directory first, then slides directory
	paths := []string{"_theme.md", "slides/_theme.md"}
	for _, path := range paths {
		if themeContent, err := os.ReadFile(path); err == nil {
			theme := strings.TrimSpace(string(themeContent))
			if theme != "" {
				return theme
			}
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
		commandBlocks:  [][]string{},
	}
}

func (m model) Init() tea.Cmd {
	return loadSlides
}

func loadSlides() tea.Msg {
	files, err := os.ReadDir(".")
	if err != nil {
		return errMsg(err)
	}

	var slides []string
	var filenames []string
	var title string
	var author string
	var configs []revealConfig
	var paths []string
	var commandBlocks [][]string

	// Load title from _title.md if it exists (check current dir first, then slides dir)
	titlePaths := []string{"_title.md", "slides/_title.md"}
	for _, path := range titlePaths {
		if titleContent, err := os.ReadFile(path); err == nil {
			title = strings.TrimSpace(string(titleContent))
			break
		}
	}

	// Load author from _author.md if it exists (check current dir first, then slides dir)
	authorPaths := []string{"_author.md", "slides/_author.md"}
	for _, path := range authorPaths {
		if authorContent, err := os.ReadFile(path); err == nil {
			author = strings.TrimSpace(string(authorContent))
			break
		}
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
		content, err := os.ReadFile(filename)
		if err != nil {
			return errMsg(err)
		}
		slide := string(content)
		slides = append(slides, slide)
		configs = append(configs, analyzeReveal(slide))
		paths = append(paths, filename)
		commandBlocks = append(commandBlocks, parseCommandBlocks(slide))
	}

	return slidesLoadedMsg{slides: slides, title: title, author: author, revealConfigs: configs, paths: paths, commandBlocks: commandBlocks}
}

func reloadSlide(slideIndex int) tea.Cmd {
	return func() tea.Msg {
		files, err := os.ReadDir(".")
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
		content, err := os.ReadFile(filenames[slideIndex])
		if err != nil {
			return errMsg(err)
		}

		slide := string(content)
		return slideReloadedMsg{slideIndex: slideIndex, content: slide, config: analyzeReveal(slide), path: filenames[slideIndex], commandBlock: parseCommandBlocks(slide)}
	}
}

type slidesLoadedMsg struct {
	slides        []string
	paths         []string
	title         string
	author        string
	revealConfigs []revealConfig
	commandBlocks [][]string
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
			m.editor.SetWidth(msg.Width)
			m.editor.SetHeight(msg.Height - 3)
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
				m.editor.Blur()
				m.showEditor = false
				m.editorPath = ""
				m.err = nil
				return m, reloadSlide(m.currentSlide)
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
		m.commandBlocks = msg.commandBlocks
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
			if len(m.commandBlocks) != len(m.slides) {
				newCommandBlocks := make([][]string, len(m.slides))
				copy(newCommandBlocks, m.commandBlocks)
				m.commandBlocks = newCommandBlocks
			}
			m.revealConfigs[msg.slideIndex] = msg.config
			m.commandBlocks[msg.slideIndex] = msg.commandBlock
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
			editor := textarea.New()
			editor.SetValue(m.slides[m.currentSlide])
			editor.Placeholder = "Edit slide markdown..."
			editor.Focus()
			editor.SetWidth(m.width)
			editor.SetHeight(m.height - 3)
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

		case "d":
			// Handle first command hotkey
			if m.currentSlide < len(m.commandBlocks) && len(m.commandBlocks[m.currentSlide]) > 0 {
				commands := m.commandBlocks[m.currentSlide]
				if len(commands) > 0 {
					if err := copyToClipboard(commands[0]); err != nil {
						m.notification = fmt.Sprintf("Copy error: %v", err)
					} else {
						// Truncate command text to fit notification bar
						displayCmd := commands[0]
						maxWidth := m.width - 12 // Reserve space for "Copied: " text and padding
						if len(displayCmd) > maxWidth {
							displayCmd = displayCmd[:maxWidth-3] + "..."
						}
						m.notification = fmt.Sprintf("Copied: %s", displayCmd)
					}
					m.notificationTimer = 3
					return m, doTick()
				}
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

		case "f", "g", "t", "y", "u", "i", "o", "p", "z":
			// Handle command hotkeys (only if current slide has commands)
			if m.currentSlide < len(m.commandBlocks) && len(m.commandBlocks[m.currentSlide]) > 0 {
				commands := m.commandBlocks[m.currentSlide]
				// Map keys to indices (d=0 is handled above, start from f=1)
				keyMap := map[string]int{
					"f": 1, "g": 2, "t": 3, "y": 4,
					"u": 5, "i": 6, "o": 7, "p": 8, "z": 9,
				}

				if cmdNum, exists := keyMap[msg.String()]; exists && cmdNum < len(commands) {
					if err := copyToClipboard(commands[cmdNum]); err != nil {
						m.notification = fmt.Sprintf("Copy error: %v", err)
					} else {
						// Truncate command text to fit notification bar
						displayCmd := commands[cmdNum]
						maxWidth := m.width - 12 // Reserve space for "Copied: " text and padding
						if len(displayCmd) > maxWidth {
							displayCmd = displayCmd[:maxWidth-3] + "..."
						}
						m.notification = fmt.Sprintf("Copied: %s", displayCmd)
					}
					m.notificationTimer = 3 // Show for 3 seconds
					return m, doTick()
				}
			}
			return m, nil
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	case tickMsg:
		if m.notificationTimer > 0 {
			m.notificationTimer--
			if m.notificationTimer <= 0 {
				m.notification = ""
			} else {
				return m, doTick()
			}
		}
		return m, nil
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

func doTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func copyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func stripCommandBlocks(content string) string {
	re := regexp.MustCompile("(?s)```commands\\s*\\n.*?\\n```")
	return re.ReplaceAllString(content, "")
}

func parseCommandBlocks(content string) []string {
	re := regexp.MustCompile("(?s)```commands\\s*\\n(.*?)\\n```")
	matches := re.FindAllStringSubmatch(content, -1)
	var commands []string

	for _, match := range matches {
		if len(match) > 1 {
			commandText := strings.TrimSpace(match[1])
			lines := strings.Split(commandText, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					commands = append(commands, line)
				}
			}
		}
	}

	return commands
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

func renderCommandHotkeys(commands []string, width int) []string {
	if len(commands) == 0 {
		return []string{}
	}

	keyLabels := []string{"d", "f", "g", "t", "y", "u", "i", "o", "p", "z"}

	var hotkeyLines []string
	for i, cmd := range commands {
		if i >= len(keyLabels) { // Only show first 10 commands
			break
		}
		// Truncate long commands to fit width
		displayCmd := cmd
		maxCmdWidth := width - 10 // Reserve space for key and padding
		if len(displayCmd) > maxCmdWidth {
			displayCmd = displayCmd[:maxCmdWidth-3] + "..."
		}

		// Style the key with darker background
		keyStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#1A602C")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			Render(keyLabels[i])

		hotkey := fmt.Sprintf("%s %s", keyStyle, displayCmd)

		// Style each hotkey line
		hotkeyLine := lipgloss.NewStyle().
			Background(lipgloss.Color("#162616")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(width).
			Padding(0, 1).
			Render(hotkey)

		hotkeyLines = append(hotkeyLines, hotkeyLine)
	}

	return hotkeyLines
}

func (m model) View() string {
	if m.showEditor {
		editorView := m.editor.View()

		pathLabel := m.editorPath
		if pathLabel == "" {
			pathLabel = "unsaved slide"
		} else {
			pathLabel = filepath.Base(pathLabel)
		}

		helpLines := []string{pathLabel, "esc to close - ctrl+s to save & exit"}
		if m.err != nil {
			helpLines = append(helpLines, fmt.Sprintf("error: %v", m.err))
		}

		helpText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Background(lipgloss.Color("#000000")).
			Width(m.width).
			Align(lipgloss.Left).
			Render(strings.Join(helpLines, " | "))

		statusBar := lipgloss.NewStyle().
			Background(lipgloss.Color("#1E3A8A")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(m.width).
			Padding(0, 1).
			Render("EDIT MODE")

		return lipgloss.JoinVertical(lipgloss.Left, statusBar, editorView, helpText)
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
	// Strip command blocks from rendered content
	slideContent = stripCommandBlocks(slideContent)
	rendered, err := m.renderer.Render(slideContent)
	if err != nil {
		rendered = "Error rendering markdown: " + err.Error()
	}

	// Calculate available height for content (reserve lines for bottom bars)
	contentHeight := m.height - 2 // status + progress
	var commandHotkeyLines []string
	if m.currentSlide < len(m.commandBlocks) && len(m.commandBlocks[m.currentSlide]) > 0 {
		commandHotkeyLines = renderCommandHotkeys(m.commandBlocks[m.currentSlide], m.width)
		contentHeight -= len(commandHotkeyLines) // reserve lines for each command hotkey
	}
	if m.notification != "" {
		contentHeight-- // additional line for notification
	}

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

	// Command hotkey lines were already rendered above in commandHotkeyLines

	// Create notification bar if there's a notification
	var notificationBar string
	if m.notification != "" {
		notificationBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#059669")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Width(m.width).
			Padding(0, 1).
			Render(m.notification)
	}

	// Build final layout
	result := content
	if notificationBar != "" {
		result += "\n" + notificationBar
	}
	// Add each command hotkey line
	for _, hotkeyLine := range commandHotkeyLines {
		result += "\n" + hotkeyLine
	}
	result += "\n" + statusLine + "\n" + progressBar
	return result
}

func initProject() error {
	// Check if slides directory already exists
	if _, err := os.Stat("slides"); err == nil {
		return fmt.Errorf("slides directory already exists")
	}

	// Create slides directory
	if err := os.MkdirAll("slides", 0755); err != nil {
		return fmt.Errorf("failed to create slides directory: %v", err)
	}

	// Create _title.md
	titleContent := "My Presentation"
	if err := os.WriteFile("slides/_title.md", []byte(titleContent), 0644); err != nil {
		return fmt.Errorf("failed to create _title.md: %v", err)
	}

	// Create _author.md
	authorContent := "Your Name"
	if err := os.WriteFile("slides/_author.md", []byte(authorContent), 0644); err != nil {
		return fmt.Errorf("failed to create _author.md: %v", err)
	}

	// Create example slides
	slide1 := `# Welcome to Slidetty

Welcome to your new presentation!

This is your first slide. You can edit this file and add more slides.

:reveal:
- Navigate with arrow keys or h/j/k/l
- Press 'q' to quit
- Press 'e' to edit current slide
- Press 'r' to reload slides`

	slide2 := `# Features

Slidetty supports many great features:

:reveal:
- **Markdown rendering** with beautiful syntax highlighting
- **Progressive reveal** for bullet points
- **Command hotkeys** for copying commands to clipboard
- **Live editing** of slides
- **Responsive design** that adapts to your terminal

Try pressing 'j' and 'k' to reveal items progressively!`

	slide3 := `# Getting Started

Here's how to work with Slidetty:

:reveal:
1. **Create slides** - Add numbered markdown files (01-slide.md, 02-slide.md, etc.)
2. **Edit content** - Press 'e' to edit the current slide
3. **Add commands** - Use ` + "```commands```" + ` blocks for copyable commands
4. **Customize theme** - Create _theme.md to set your preferred style

` + "```commands" + `
echo "Hello, World!"
ls -la
git status
` + "```" + `

Press 'd' to copy the first command above!`

	// Write example slides
	slides := map[string]string{
		"01-welcome.md":  slide1,
		"02-features.md": slide2,
		"03-getting-started.md": slide3,
	}

	for filename, content := range slides {
		if err := os.WriteFile(filepath.Join("slides", filename), []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create %s: %v", filename, err)
		}
	}

	fmt.Println("✅ Slideshow initialized successfully!")
	fmt.Println("\nCreated files:")
	fmt.Println("  slides/")
	fmt.Println("  ├── _title.md")
	fmt.Println("  ├── _author.md")
	fmt.Println("  ├── 01-welcome.md")
	fmt.Println("  ├── 02-features.md")
	fmt.Println("  └── 03-getting-started.md")
	fmt.Println("\nRun 'slidetty' to start your presentation!")

	return nil
}

func main() {
	// Check for init command
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := initProject(); err != nil {
			fmt.Printf("Error initializing project: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Run normal slideshow
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
