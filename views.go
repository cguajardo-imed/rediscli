package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fogleman/ease"
	"github.com/lucasb-eyer/go-colorful"
)

type model struct {
	Choice         int
	Chosen         bool
	Frames         int
	Progress       float64
	Loaded         bool
	Quitting       bool
	query          string
	queryMode      bool
	queryResult    string
	err            error
	iterationMode  bool
	delayMode      bool
	iterationInput string
	delayInput     string
	iterations     int
	delay          time.Duration
	currentIter    int
	selectedAction int // 1 for publish create, 2 for publish create & delete
	isProcessing   bool
	processingMsg  string
	startTime      time.Time
	progressChan   chan progressMsg
}

type frameMsg time.Time
type progressMsg struct {
	iteration int
	total     int
	message   string
}

const (
	progressBarWidth  = 71
	progressFullChar  = "█"
	progressEmptyChar = "░"
	dotChar           = " • "
	fps               = 60
	maxTicks          = 10
)

// General stuff for styling the view
var (
	keywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ticksStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))
	checkboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	progressEmpty = subtleStyle.Render(progressEmptyChar)
	dotStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(dotChar)
	mainStyle     = lipgloss.NewStyle().MarginLeft(2)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	// Gradient colors for the progress bar
	ramp = makeRampStyles("#B14FFF", "#00FFA3", progressBarWidth)
)

// Init initializes the model
func (m model) Init() tea.Cmd {
	return frameCmd()
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quitting = true
			return m, tea.Quit

		case "esc":
			if m.queryMode {
				// Exit query mode and return to main menu
				m.queryMode = false
				m.query = ""
				m.Chosen = false
				return m, nil
			} else if m.iterationMode {
				// Exit iteration mode and return to main menu
				m.iterationMode = false
				m.iterationInput = ""
				m.Chosen = false
				return m, nil
			} else if m.delayMode {
				// Exit delay mode and return to iteration mode
				m.delayMode = false
				m.delayInput = ""
				m.iterationMode = true
				return m, nil
			} else if m.queryResult != "" {
				// Clear results and return to main menu
				m.queryResult = ""
				m.Chosen = false
				return m, nil
			} else {
				// Quit from main menu
				m.Quitting = true
				return m, tea.Quit
			}

		case "enter":
			if m.queryResult != "" {
				// Clear results and return to main menu
				m.queryResult = ""
				m.Chosen = false
				return m, nil
			}

			if m.queryMode {
				// Execute the query
				m.queryResult = executeQuery(m.query)
				m.query = ""
				m.queryMode = false
				return m, nil
			}

			if m.iterationMode {
				// Parse iteration count
				if m.iterationInput == "" {
					m.queryResult = "Error: Please enter a valid number of iterations"
					m.iterationMode = false
					m.Chosen = false
					return m, nil
				}

				iterations, err := strconv.Atoi(m.iterationInput)
				if err != nil || iterations < 1 {
					m.queryResult = "Error: Please enter a valid number (1 or greater)"
					m.iterationMode = false
					m.iterationInput = ""
					m.Chosen = false
					return m, nil
				}

				m.iterations = iterations
				m.iterationInput = ""

				// If more than 1 iteration, ask for delay
				if iterations > 1 {
					m.iterationMode = false
					m.delayMode = true
					return m, nil
				}

				// Execute once and return
				m.iterationMode = false
				return m, executeActionOnce(m.selectedAction)
			}

			if m.delayMode {
				// Parse delay
				if m.delayInput == "" {
					m.queryResult = "Error: Please enter a valid delay (e.g., 1s, 500ms, 2m)"
					m.delayMode = false
					m.Chosen = false
					return m, nil
				}

				delay, err := time.ParseDuration(m.delayInput)
				if err != nil {
					m.queryResult = "Error: Invalid delay format. Use format like: 1s, 500ms, 2m"
					m.delayMode = false
					m.delayInput = ""
					m.Chosen = false
					return m, nil
				}

				m.delay = delay
				m.delayInput = ""
				m.delayMode = false

				// Execute the action with iterations
				m.isProcessing = true
				m.startTime = time.Now()
				m.currentIter = 0
				m.progressChan = make(chan progressMsg, 10)

				// Start the operation
				go performIterations(m.selectedAction, m.iterations, m.delay, m.progressChan)

				// Start listening for progress
				return m, waitForProgress(m.progressChan)
			}

			m.Chosen = true
			switch m.Choice {
			case 0:
				// Query Redis mode
				m.queryMode = true
				m.Chosen = false
				return m, nil
			case 1:
				// Publish create - ask for iterations
				m.selectedAction = 1
				m.iterationMode = true
				m.Chosen = false
				return m, nil
			case 2:
				// Publish create & delete - ask for iterations
				m.selectedAction = 2
				m.iterationMode = true
				m.Chosen = false
				return m, nil
			}
			return m, nil

		case "up", "k":
			if !m.Chosen && !m.queryMode && m.queryResult == "" {
				if m.Choice > 0 {
					m.Choice--
				}
			}

		case "down", "j":
			if !m.Chosen && !m.queryMode && m.queryResult == "" {
				if m.Choice < 2 {
					m.Choice++
				}
			}

		case "backspace":
			if m.queryMode && len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
			} else if m.iterationMode && len(m.iterationInput) > 0 {
				m.iterationInput = m.iterationInput[:len(m.iterationInput)-1]
			} else if m.delayMode && len(m.delayInput) > 0 {
				m.delayInput = m.delayInput[:len(m.delayInput)-1]
			}

		default:
			if m.queryMode && !m.Chosen {
				// Filter out special keys
				key := msg.String()
				if len(key) == 1 || key == " " {
					m.query += key
				}
			} else if m.iterationMode && !m.Chosen {
				// Filter out special keys - only allow numbers
				key := msg.String()
				if len(key) == 1 && key >= "0" && key <= "9" {
					m.iterationInput += key
				}
			} else if m.delayMode && !m.Chosen {
				// Filter out special keys - allow numbers, letters (for s, m, h), and decimal point
				key := msg.String()
				if len(key) == 1 && (key >= "0" && key <= "9" || key >= "a" && key <= "z" || key >= "A" && key <= "Z" || key == ".") {
					m.delayInput += key
				}
			} else if m.queryResult != "" {
				// Any key press clears results
				m.queryResult = ""
				m.Chosen = false
				return m, nil
			}
		}

	case frameMsg:
		if !m.Loaded {
			m.Frames++
			m.Progress = ease.OutBounce(float64(m.Frames) / float64(fps*2))
			if m.Progress >= 1 {
				m.Progress = 1
				m.Loaded = true
				m.Frames = 0
				return m, nil
			}
			return m, frameCmd()
		}

	case progressMsg:
		m.currentIter = msg.iteration
		m.iterations = msg.total
		m.processingMsg = msg.message
		if msg.total > 0 {
			m.Progress = float64(msg.iteration) / float64(msg.total)
		}
		// Continue listening for more progress
		return m, waitForProgress(m.progressChan)

	case resultMsg:
		m.queryResult = msg.result
		m.Chosen = false
		m.isProcessing = false
		m.currentIter = 0
		return m, nil

	case tea.WindowSizeMsg:
		return m, nil
	}

	return m, nil
}

// View renders the UI
func (m model) View() string {
	if m.Quitting {
		return "Goodbye!\n"
	}

	if m.queryMode {
		return queryView(m)
	}

	if m.iterationMode {
		return iterationView(m)
	}

	if m.delayMode {
		return delayView(m)
	}

	if m.isProcessing {
		return iterationProgressView(m)
	}

	if m.Chosen && !m.Loaded {
		return progressView(m)
	}

	if m.queryResult != "" {
		return resultsView(m)
	}

	return mainStyle.Render(choicesView(m))
}

// The first view, where you're choosing a task
func choicesView(m model) string {
	c := m.Choice

	tpl := "What to do today?\n\n"
	tpl += "%s\n\n"
	tpl += subtleStyle.Render("j/k, up/down: select") + dotStyle +
		subtleStyle.Render("enter: choose") + dotStyle +
		subtleStyle.Render("q, esc: quit")

	choices := fmt.Sprintf(
		"%s\n%s\n%s",
		checkbox("Query Redis", c == 0),
		checkbox("Publish create", c == 1),
		checkbox("Publish create & delete", c == 2),
	)

	return fmt.Sprintf(tpl, choices)
}

// Query input view
func queryView(m model) string {
	tpl := "Enter Redis command:\n\n"
	tpl += "> %s\n\n"
	tpl += subtleStyle.Render("enter: execute") + dotStyle +
		subtleStyle.Render("esc: back")

	return mainStyle.Render(fmt.Sprintf(tpl, keywordStyle.Render(m.query)))
}

// Iteration input view
func iterationView(m model) string {
	tpl := "How many times should this action be repeated?\n\n"
	tpl += "> %s\n\n"
	tpl += subtleStyle.Render("enter: confirm") + dotStyle +
		subtleStyle.Render("esc: back")

	return mainStyle.Render(fmt.Sprintf(tpl, keywordStyle.Render(m.iterationInput)))
}

// Delay input view
func delayView(m model) string {
	tpl := "How much time to wait between iterations?\n"
	tpl += subtleStyle.Render("(Examples: 1s, 500ms, 2m, 1.5s)") + "\n\n"
	tpl += "> %s\n\n"
	tpl += subtleStyle.Render("enter: confirm") + dotStyle +
		subtleStyle.Render("esc: back")

	return mainStyle.Render(fmt.Sprintf(tpl, keywordStyle.Render(m.delayInput)))
}

// Results view
func resultsView(m model) string {
	tpl := "Query Result:\n\n%s\n\n"
	tpl += subtleStyle.Render("enter/any key: continue") + dotStyle +
		subtleStyle.Render("esc: main menu")

	result := successStyle.Render(m.queryResult)
	if m.err != nil {
		result = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	return mainStyle.Render(fmt.Sprintf(tpl, result))
}

// Iteration progress view
func iterationProgressView(m model) string {
	var b strings.Builder

	// Title
	operationName := "Publish Create"
	if m.selectedAction == 2 {
		operationName = "Publish Create & Delete"
	}
	b.WriteString(fmt.Sprintf("Operation: %s\n\n", keywordStyle.Render(operationName)))

	// Progress bar
	w := progressBarWidth
	completed := int(m.Progress * float64(w))

	var bar strings.Builder
	for i := 0; i < w; i++ {
		if i < completed {
			bar.WriteString(ramp[i].Render(progressFullChar))
		} else {
			bar.WriteString(progressEmpty)
		}
	}
	b.WriteString(bar.String())
	b.WriteString("\n\n")

	// Progress text
	percent := m.Progress * 100
	b.WriteString(fmt.Sprintf("Progress: %d/%d iterations (%.0f%%)\n", m.currentIter, m.iterations, percent))

	// Current message
	if m.processingMsg != "" {
		b.WriteString(fmt.Sprintf("\n%s\n", subtleStyle.Render(m.processingMsg)))
	}

	// Elapsed time
	if !m.startTime.IsZero() {
		elapsed := time.Since(m.startTime)
		b.WriteString(fmt.Sprintf("\nElapsed: %s\n", ticksStyle.Render(elapsed.Round(time.Millisecond).String())))

		// Estimated time remaining
		if m.currentIter > 0 && m.currentIter < m.iterations {
			avgTimePerIter := elapsed / time.Duration(m.currentIter)
			remaining := avgTimePerIter * time.Duration(m.iterations-m.currentIter)
			b.WriteString(fmt.Sprintf("Estimated remaining: %s\n", subtleStyle.Render(remaining.Round(time.Second).String())))
		}
	}

	b.WriteString("\n" + subtleStyle.Render("Please wait..."))

	return mainStyle.Render(b.String())
}

// Progress bar view
func progressView(m model) string {
	w := progressBarWidth
	fullSize := int(m.Progress * float64(w))

	var bar strings.Builder
	for i := range w {
		if i < fullSize {
			bar.WriteString(ramp[i].Render(progressFullChar))
		} else {
			bar.WriteString(progressEmpty)
		}
	}

	percent := m.Progress * 100

	return mainStyle.Render(fmt.Sprintf(
		"Loading...\n\n%s\n\n%.0f%%",
		bar.String(),
		percent,
	))
}

func checkbox(label string, checked bool) string {
	if checked {
		return checkboxStyle.Render("[x] " + label)
	}
	return fmt.Sprintf("[ ] %s", label)
}

// Generate a blend of colors
func makeRampStyles(colorA, colorB string, steps float64) (s []lipgloss.Style) {
	cA, _ := colorful.Hex(colorA)
	cB, _ := colorful.Hex(colorB)

	for i := 0.0; i < steps; i++ {
		c := cA.BlendLuv(cB, i/steps)
		s = append(s, lipgloss.NewStyle().Foreground(lipgloss.Color(colorToHex(c))))
	}
	return
}

// Convert a colorful.Color to a hexadecimal format
func colorToHex(c colorful.Color) string {
	return fmt.Sprintf("#%s%s%s", colorFloatToHex(c.R), colorFloatToHex(c.G), colorFloatToHex(c.B))
}

// Helper function for converting colors to hex. Assumes a value between 0 and 1
func colorFloatToHex(f float64) (s string) {
	s = strconv.FormatInt(int64(f*255), 16)
	if len(s) == 1 {
		s = "0" + s
	}
	return
}

// Commands for Bubble Tea
func frameCmd() tea.Cmd {
	return tea.Tick(time.Second/fps, func(t time.Time) tea.Msg {
		return frameMsg(t)
	})
}

// Execute action once
func executeActionOnce(action int) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		var result string
		var successful, failed int

		LogBanner("Starting Operation - Single Execution")

		switch action {
		case 1:
			// Publish create
			key, channel := fakeRecordWithIteration(1, 1)
			publishRecordWithIteration(key, channel, 1, 1)
			result = "Published create event successfully"
			successful++
		case 2:
			// Publish create & delete
			key, channel := fakeRecordWithIteration(1, 1)
			publishRecordWithIteration(key, channel, 1, 1)
			time.Sleep(2 * time.Second)
			err := client.Del(ctx, key).Err()
			if err != nil {
				LogRedisError("delete", key, err, 1, 1)
				failed++
			} else {
				LogRedisOperation("delete", key, "", 1, 1)
			}
			publishRecordWithIteration(key, channel, 1, 1)
			result = "Published create event successfully"
			successful++
		}

		duration := time.Since(startTime)
		operation := "Publish Create"
		if action == 2 {
			operation = "Publish Create & Delete"
		}
		LogSummary(operation, 1, successful, failed, duration)

		if IsTUIMode() {
			result += fmt.Sprintf("\n\nLog file: %s", GetLogFilePath())
		}

		return resultMsg{result: result}
	}
}

// Execute action with iterations and delay
func executeActionWithIterations(action int, iterations int, delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		progressChan := make(chan progressMsg)

		// Start the worker goroutine
		go performIterations(action, iterations, delay, progressChan)

		// Listen for progress updates
		return waitForProgress(progressChan)
	}
}

// Wait for progress messages
func waitForProgress(progressChan chan progressMsg) tea.Cmd {
	return func() tea.Msg {
		msg := <-progressChan
		if msg.iteration == -1 {
			// Final result message
			return resultMsg{result: msg.message}
		}
		// Progress update
		return msg
	}
}

// Listen for next progress update
func listenForProgress() tea.Cmd {
	return func() tea.Msg {
		// This will be replaced by channel-based updates
		return nil
	}
}

// Perform iterations with progress updates
func performIterations(action int, iterations int, delay time.Duration, progressChan chan progressMsg) {
	defer close(progressChan)

	startTime := time.Now()
	var results strings.Builder
	var successful, failed int

	operation := "Publish Create"
	if action == 2 {
		operation = "Publish Create & Delete"
	}

	LogBanner(fmt.Sprintf("Starting Operation - %s (%d iterations, %s delay)", operation, iterations, delay))

	// Send initial progress
	progressChan <- progressMsg{
		iteration: 0,
		total:     iterations,
		message:   "Starting operations...",
	}

	for i := 1; i <= iterations; i++ {
		iterStart := time.Now()

		// Send progress update for current iteration
		progressChan <- progressMsg{
			iteration: i,
			total:     iterations,
			message:   fmt.Sprintf("Processing iteration %d/%d...", i, iterations),
		}

		switch action {
		case 1:
			// Publish create
			key, channel := fakeRecordWithIteration(i, iterations)
			publishRecordWithIteration(key, channel, i, iterations)
			successful++
		case 2:
			// Publish create & delete
			key, channel := fakeRecordWithIteration(i, iterations)
			publishRecordWithIteration(key, channel, i, iterations)
			time.Sleep(2 * time.Second)
			err := client.Del(ctx, key).Err()
			if err != nil {
				LogRedisError("delete", key, err, i, iterations)
				failed++
			} else {
				LogRedisOperation("delete", key, "", i, iterations)
			}
			publishRecordWithIteration(key, channel, i, iterations)
			successful++
		}

		iterDuration := time.Since(iterStart)
		LogInfo(fmt.Sprintf("[Iteration %d/%d] Completed in %s", i, iterations, iterDuration))

		// Wait between iterations (except after last one)
		if i < iterations {
			LogInfo(fmt.Sprintf("[Iteration %d/%d] Waiting %s before next iteration...", i, iterations, delay))
			time.Sleep(delay)
		}
	}

	totalDuration := time.Since(startTime)
	LogSummary(operation, iterations, successful, failed, totalDuration)

	fmt.Fprintf(&results, "\n✓ Completed %d iterations successfully!", iterations)
	fmt.Fprintf(&results, "\n  Duration: %s", totalDuration)
	fmt.Fprintf(&results, "\n  Successful: %d | Failed: %d", successful, failed)

	if IsTUIMode() {
		fmt.Fprintf(&results, "\n\nLog file: %s", GetLogFilePath())
	}

	// Send final result
	progressChan <- progressMsg{
		iteration: -1,
		total:     iterations,
		message:   results.String(),
	}
}

type resultMsg struct {
	result string
}

// Execute a Redis query and return the result as a string
func executeQuery(query string) string {
	if query == "" {
		return "Empty query"
	}

	// Parse the query into command and arguments
	parts := strings.Fields(query)
	if len(parts) == 0 {
		return "Invalid query"
	}

	// Convert to []interface{} for Redis Do command
	args := make([]any, len(parts))
	for i, part := range parts {
		args[i] = part
	}

	// Execute the command
	res, err := client.Do(ctx, args...).Result()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	// Format the result based on its type
	switch val := res.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case []any:
		var builder strings.Builder
		for i, v := range val {
			builder.WriteString(fmt.Sprintf("%d) %v\n", i+1, v))
		}
		return builder.String()
	case int64:
		return fmt.Sprintf("%d", val)
	case nil:
		return "(nil)"
	default:
		return fmt.Sprintf("%v", val)
	}
}
