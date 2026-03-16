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
	Choice            int
	Chosen            bool
	Frames            int
	Progress          float64
	Loaded            bool
	Quitting          bool
	query             string
	queryMode         bool
	queryResult       string
	err               error
	iterationMode     bool
	delayMode         bool
	iterationInput    string
	delayInput        string
	iterations        int
	delay             time.Duration
	currentIter       int
	selectedAction    int // 1 for publish create, 2 for publish create & delete
	isProcessing      bool
	processingMsg     string
	startTime         time.Time
	progressChan      chan progressMsg
	publishModeChoice int  // 0 for default, 1 for custom
	publishModeSelect bool // true when selecting publish mode
	customInputMode   int  // 0: place_code, 1: service_name, 2: custom_params
	placeCodeInput    string
	serviceNameInput  string
	customParamsInput string
	useCustom         bool
	// Redis Explorer
	explorerActive bool
	explorer       explorerModel
	// Self-update
	updateMode      bool
	updateLines     []string // progress lines streamed from SelfUpdate
	updateDone      bool
	updateErr       error
	updateAvailable bool   // true when a newer version exists on GitHub
	latestVersion   string // the version tag fetched from GitHub
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
	return tea.Batch(frameCmd(), checkLatestVersionCmd())
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ── Delegate all events to the explorer when it is active ──
	if m.explorerActive {
		switch msg := msg.(type) {
		case explorerDoneMsg:
			m.explorerActive = false
			m.explorer = explorerModel{}
			return m, nil
		default:
			updated, cmd := m.explorer.Update(msg)
			m.explorer = updated.(explorerModel)
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quitting = true
			return m, tea.Quit

		case "esc":
			if m.updateMode {
				if m.updateDone {
					m.updateMode = false
					m.updateLines = nil
					m.updateDone = false
					m.updateErr = nil
					m.Chosen = false
				}
				return m, nil
			}
			if m.queryMode {
				// Exit query mode and return to main menu
				m.queryMode = false
				m.query = ""
				m.Chosen = false
				return m, nil
			} else if m.customInputMode >= 0 && m.useCustom {
				// Exit custom input and go back to mode selection
				switch m.customInputMode {
				case 0:
					m.customInputMode = -1
					m.placeCodeInput = ""
					m.publishModeSelect = true
					m.useCustom = false
				case 1:
					m.customInputMode = 0
					m.serviceNameInput = ""
				case 2:
					m.customInputMode = 1
					m.customParamsInput = ""
				}
				return m, nil
			} else if m.publishModeSelect {
				// Exit publish mode selection and return to main menu
				m.publishModeSelect = false
				m.publishModeChoice = 0
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

			if m.publishModeSelect {
				// User selected default or custom mode
				if m.publishModeChoice == 0 {
					// Default mode - proceed to iterations
					m.publishModeSelect = false
					m.useCustom = false
					m.iterationMode = true
					return m, nil
				} else {
					// Custom mode - ask for parameters
					m.publishModeSelect = false
					m.useCustom = true
					m.customInputMode = 0 // Start with place_code
					return m, nil
				}
			}

			if m.useCustom && m.customInputMode >= 0 {
				// Handle custom parameter inputs
				switch m.customInputMode {
				case 0:
					// Place code entered (can be empty, defaults to "0")
					m.customInputMode = 1 // Move to service_name
					return m, nil
				case 1:
					// Service name entered (mandatory)
					if m.serviceNameInput == "" {
						m.queryResult = "Error: Service name is mandatory"
						m.customInputMode = -1
						m.useCustom = false
						m.Chosen = false
						return m, nil
					}
					m.customInputMode = 2 // Move to custom_params
					return m, nil
				case 2:
					// Custom params entered (can be empty)
					m.customInputMode = -1
					m.useCustom = false
					m.iterationMode = true
					return m, nil
				}
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
				return m, executeActionOnce(m.selectedAction, m.placeCodeInput, m.serviceNameInput, m.customParamsInput)
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
				go performIterations(m.selectedAction, m.iterations, m.delay, m.placeCodeInput, m.serviceNameInput, m.customParamsInput, m.progressChan)

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
				// Publish create - ask for mode selection (default/custom)
				m.selectedAction = 1
				m.publishModeSelect = true
				m.publishModeChoice = 0
				m.Chosen = false
				return m, nil
			case 2:
				// Publish create & delete - ask for mode selection (default/custom)
				m.selectedAction = 2
				m.publishModeSelect = true
				m.publishModeChoice = 0
				m.Chosen = false
				return m, nil
			case 3:
				// Redis Explorer
				m.explorer = newExplorerModel()
				m.explorerActive = true
				m.Chosen = false
				return m, m.explorer.Init()
			case 4:
				// Self-update — only reachable when updateAvailable is true
				if m.updateAvailable {
					m.updateMode = true
					m.updateLines = []string{}
					m.updateDone = false
					m.updateErr = nil
					m.Chosen = false
					return m, runSelfUpdateCmd()
				}
			}
			return m, nil

		case "up", "k":
			if m.publishModeSelect {
				if m.publishModeChoice > 0 {
					m.publishModeChoice--
				}
			} else if !m.Chosen && !m.queryMode && m.queryResult == "" {
				if m.Choice > 0 {
					m.Choice--
				}
			}

		case "down", "j":
			if m.publishModeSelect {
				if m.publishModeChoice < 1 {
					m.publishModeChoice++
				}
			} else if !m.Chosen && !m.queryMode && m.queryResult == "" {
				maxChoice := 3
				if m.updateAvailable {
					maxChoice = 4
				}
				if m.Choice < maxChoice {
					m.Choice++
				}
			}

		case "backspace":
			if m.queryMode && len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
			} else if m.useCustom && m.customInputMode == 0 && len(m.placeCodeInput) > 0 {
				m.placeCodeInput = m.placeCodeInput[:len(m.placeCodeInput)-1]
			} else if m.useCustom && m.customInputMode == 1 && len(m.serviceNameInput) > 0 {
				m.serviceNameInput = m.serviceNameInput[:len(m.serviceNameInput)-1]
			} else if m.useCustom && m.customInputMode == 2 && len(m.customParamsInput) > 0 {
				m.customParamsInput = m.customParamsInput[:len(m.customParamsInput)-1]
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
			} else if m.useCustom && m.customInputMode == 0 {
				// Place code input - allow numbers only
				key := msg.String()
				if len(key) == 1 && key >= "0" && key <= "9" {
					m.placeCodeInput += key
				}
			} else if m.useCustom && m.customInputMode == 1 {
				// Service name input - allow alphanumeric and common symbols
				key := msg.String()
				if len(key) == 1 && (key >= "0" && key <= "9" || key >= "a" && key <= "z" || key >= "A" && key <= "Z" || key == "_" || key == "-" || key == ".") {
					m.serviceNameInput += key
				}
			} else if m.useCustom && m.customInputMode == 2 {
				// Custom params input - allow alphanumeric and common symbols
				key := msg.String()
				if len(key) == 1 && (key >= "0" && key <= "9" || key >= "a" && key <= "z" || key >= "A" && key <= "Z" || key == "_" || key == "-" || key == "." || key == ":" || key == "," || key == " ") {
					m.customParamsInput += key
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

	case updateProgressMsg:
		m.updateLines = append(m.updateLines, msg.line)
		// Keep draining the channel for the next progress line.
		return m, waitForUpdateMsg()

	case versionCheckMsg:
		if msg.latestVersion != "" && msg.latestVersion != Version {
			m.updateAvailable = true
			m.latestVersion = msg.latestVersion
		}
		return m, nil

	case updateDoneMsg:
		m.updateDone = true
		if msg.err != nil {
			m.updateErr = msg.err
			m.updateLines = append(m.updateLines, errorStyle.Render("Error: "+msg.err.Error()))
		} else if msg.result != nil && msg.result.AlreadyLatest {
			m.updateLines = append(m.updateLines, successStyle.Render(
				fmt.Sprintf("Already up to date (%s).", msg.result.NewVersion),
			))
		} else if msg.result != nil {
			m.updateLines = append(m.updateLines, successStyle.Render(
				fmt.Sprintf("Updated %s → %s. Restart to apply.", msg.result.PreviousVersion, msg.result.NewVersion),
			))
		}
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

	if m.explorerActive {
		return m.explorer.View()
	}

	if m.updateMode {
		return updateView(m)
	}

	if m.queryMode {
		return queryView(m)
	}

	if m.publishModeSelect {
		return publishModeView(m)
	}

	if m.useCustom && m.customInputMode >= 0 {
		return customInputView(m)
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

	var choices string
	if m.updateAvailable {
		choices = fmt.Sprintf(
			"%s\n%s\n%s\n%s\n%s",
			checkbox("Query Redis", c == 0),
			checkbox("Publish create", c == 1),
			checkbox("Publish create & delete", c == 2),
			checkbox("Redis Explorer", c == 3),
			checkbox(fmt.Sprintf("Update rediscli  %s → %s",
				Version, m.latestVersion), c == 4),
		)
	} else {
		choices = fmt.Sprintf(
			"%s\n%s\n%s\n%s",
			checkbox("Query Redis", c == 0),
			checkbox("Publish create", c == 1),
			checkbox("Publish create & delete", c == 2),
			checkbox("Redis Explorer", c == 3),
		)
	}

	return fmt.Sprintf(tpl, choices)
}

// Publish mode selection view
func publishModeView(m model) string {
	c := m.publishModeChoice

	tpl := "Select publish mode:\n\n"
	tpl += "%s\n\n"
	tpl += subtleStyle.Render("j/k, up/down: select") + dotStyle +
		subtleStyle.Render("enter: choose") + dotStyle +
		subtleStyle.Render("esc: back")

	choices := fmt.Sprintf(
		"%s\n%s",
		checkbox("Default (uses default notification data)", c == 0),
		checkbox("Custom (specify place_code, service_name, custom_params)", c == 1),
	)

	return fmt.Sprintf(tpl, choices)
}

// Custom input view for place_code, service_name, and custom_params
func customInputView(m model) string {
	var tpl string
	var input string
	var placeholder string

	switch m.customInputMode {
	case 0:
		tpl = "Enter place_code (press Enter to skip, defaults to '0'):\n\n"
		input = m.placeCodeInput
		placeholder = "e.g., 1234 or leave empty for '0'"
	case 1:
		tpl = "Enter service_name (mandatory):\n\n"
		input = m.serviceNameInput
		placeholder = "e.g., demo, alerts, notifications"
	case 2:
		tpl = "Enter custom_params (press Enter to skip):\n\n"
		input = m.customParamsInput
		placeholder = "e.g., param1,param2 or leave empty"
	}

	tpl += "> %s\n\n"
	tpl += subtleStyle.Render(placeholder) + "\n\n"
	tpl += subtleStyle.Render("enter: confirm") + dotStyle +
		subtleStyle.Render("esc: back")

	return fmt.Sprintf(tpl, input)
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
	fmt.Fprintf(&b, "Operation: %s\n", keywordStyle.Render(operationName))

	// Custom parameters panel — only shown when the user filled in custom values
	if m.placeCodeInput != "" || m.serviceNameInput != "" || m.customParamsInput != "" {
		b.WriteString(subtleStyle.Render("Parameters:") + "\n")

		placeCode := m.placeCodeInput
		if placeCode == "" {
			placeCode = "0 (default)"
		}
		fmt.Fprintf(&b, "  %s %s\n",
			subtleStyle.Render("place_code   :"),
			keywordStyle.Render(placeCode))

		serviceName := m.serviceNameInput
		if serviceName == "" {
			serviceName = "demo (default)"
		}
		fmt.Fprintf(&b, "  %s %s\n",
			subtleStyle.Render("service_name :"),
			keywordStyle.Render(serviceName))

		customParams := m.customParamsInput
		if customParams == "" {
			customParams = "(empty)"
		}
		fmt.Fprintf(&b, "  %s %s\n",
			subtleStyle.Render("custom_params:"),
			keywordStyle.Render(customParams))
	}

	b.WriteString("\n")

	// Progress bar
	w := progressBarWidth
	completed := int(m.Progress * float64(w))

	var bar strings.Builder
	for i := range w {
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
	fmt.Fprintf(&b, "Progress: %d/%d iterations (%.0f%%)\n", m.currentIter, m.iterations, percent)

	// Current message
	if m.processingMsg != "" {
		fmt.Fprintf(&b, "\n%s\n", subtleStyle.Render(m.processingMsg))
	}

	// Elapsed time
	if !m.startTime.IsZero() {
		elapsed := time.Since(m.startTime)
		fmt.Fprintf(&b, "\nElapsed: %s\n", ticksStyle.Render(elapsed.Round(time.Millisecond).String()))

		// Estimated time remaining
		if m.currentIter > 0 && m.currentIter < m.iterations {
			avgTimePerIter := elapsed / time.Duration(m.currentIter)
			remaining := avgTimePerIter * time.Duration(m.iterations-m.currentIter)
			fmt.Fprintf(&b, "Estimated remaining: %s\n", subtleStyle.Render(remaining.Round(time.Second).String()))
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
func executeActionOnce(action int, placeCode, serviceName, customParams string) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		var result string
		var successful, failed int

		LogBanner("Starting Operation - Single Execution")

		switch action {
		case 1:
			// Publish create
			key, channel := fakeRecordWithIterationAndParams(placeCode, serviceName, customParams, 1, 1)
			publishRecordWithIteration(key, channel, 1, 1)
			result = "Published create event successfully"
			successful++
		case 2:
			// Publish create & delete
			key, channel := fakeRecordWithIterationAndParams(placeCode, serviceName, customParams, 1, 1)
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
func executeActionWithIterations(action int, iterations int, delay time.Duration, placeCode, serviceName, customParams string) tea.Cmd {
	return func() tea.Msg {
		progressChan := make(chan progressMsg)

		// Start the worker goroutine
		go performIterations(action, iterations, delay, placeCode, serviceName, customParams, progressChan)

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
func performIterations(action, iterations int, delay time.Duration, placeCode, serviceName, customParams string, progressChan chan progressMsg) {
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
			key, channel := fakeRecordWithIterationAndParams(placeCode, serviceName, customParams, i, iterations)
			publishRecordWithIteration(key, channel, i, iterations)
			successful++
		case 2:
			// Publish create & delete
			key, channel := fakeRecordWithIterationAndParams(placeCode, serviceName, customParams, i, iterations)
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

// ── Self-update view ──────────────────────────────────────────────────────────

func updateView(m model) string {
	var b strings.Builder

	b.WriteString(keywordStyle.Render("Update rediscli"))
	b.WriteString("\n\n")

	for _, line := range m.updateLines {
		b.WriteString(line + "\n")
	}

	if !m.updateDone {
		b.WriteString("\n" + subtleStyle.Render("Please wait…"))
	} else {
		b.WriteString("\n" + subtleStyle.Render("Press esc to return to the menu."))
	}

	return mainStyle.Render(b.String())
}

// ── Version check ─────────────────────────────────────────────────────────────

type versionCheckMsg struct{ latestVersion string }

// checkLatestVersionCmd silently queries the GitHub API on startup.
// It never blocks the UI — on any error it simply returns an empty tag.
func checkLatestVersionCmd() tea.Cmd {
	return func() tea.Msg {
		release, err := fetchLatestRelease()
		if err != nil || release == nil {
			return versionCheckMsg{}
		}
		return versionCheckMsg{latestVersion: release.TagName}
	}
}

// ── Self-update async messages & command ─────────────────────────────────────

type updateProgressMsg struct{ line string }
type updateDoneMsg struct {
	result *UpdateResult
	err    error
}

// updateMsgChan carries progress and done messages from the updater goroutine.
var updateMsgChan chan tea.Msg

// runSelfUpdateCmd kicks off the self-update in a background goroutine and
// returns the first cmd that begins draining the channel, mirroring the same
// pattern used by performIterations / waitForProgress.
func runSelfUpdateCmd() tea.Cmd {
	updateMsgChan = make(chan tea.Msg, 32)

	go func() {
		result, err := SelfUpdate(func(line string) {
			updateMsgChan <- updateProgressMsg{line: line}
		})
		updateMsgChan <- updateDoneMsg{result: result, err: err}
		close(updateMsgChan)
	}()

	return waitForUpdateMsg()
}

// waitForUpdateMsg returns a Cmd that reads one message from updateMsgChan.
// After each updateProgressMsg the TUI calls this again to keep draining.
func waitForUpdateMsg() tea.Cmd {
	return func() tea.Msg {
		return <-updateMsgChan
	}
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
			fmt.Fprintf(&builder, "%d) %v\n", i+1, v)
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
