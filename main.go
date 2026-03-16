package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	redis "github.com/redis/go-redis/v9"
)

// Version can be set at build time with -ldflags
var Version = "dev"

var (
	ctx    context.Context
	client *redis.Client
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using system environment variables")
	}

	// Parse command-line arguments
	args := os.Args[1:]

	// Handle special flags
	if len(args) > 0 {
		switch args[0] {
		case "-v", "--version":
			fmt.Printf("rediscli version %s\n", Version)
			return
		case "-h", "--help":
			printHelp()
			return
		}
	}

	// Initialize Redis connection
	initConnection()

	// Check the connection
	if !connectionStatus() {
		fmt.Println("Error trying to connect with Redis")
		os.Exit(1)
	}

	// If command-line arguments are provided, run in CLI mode
	if len(args) > 0 {
		// Initialize logger for CLI mode
		if err := InitLogger(false); err != nil {
			fmt.Printf("Warning: Failed to initialize logger: %v\n", err)
		}
		defer CloseLogger()

		success := executeCommand(args)
		if !success {
			os.Exit(1)
		}
		return
	}

	// Initialize logger for TUI mode
	if err := InitLogger(true); err != nil {
		fmt.Printf("Warning: Failed to initialize logger: %v\n", err)
	}
	defer CloseLogger()

	// Otherwise, run in TUI mode
	initialModel := model{
		Choice:         0,
		Chosen:         false,
		Frames:         0,
		Progress:       0,
		Loaded:         false,
		Quitting:       false,
		query:          "",
		queryMode:      false,
		queryResult:    "",
		err:            nil,
		iterationMode:  false,
		delayMode:      false,
		iterationInput: "",
		delayInput:     "",
		iterations:     0,
		delay:          0,
		currentIter:    0,
		selectedAction: 0,
		isProcessing:   false,
		processingMsg:  "",
		progressChan:   nil,
	}

	LogBanner("Redis CLI Starting in TUI Mode")

	p := tea.NewProgram(initialModel)
	if _, err := p.Run(); err != nil {
		fmt.Println("could not start program:", err)
		os.Exit(1)
	}

	fmt.Printf("\nSession log saved to: %s\n", GetLogFilePath())
}

func printHelp() {
	help := `
██████╗ ███████╗██████╗ ██╗███████╗ ██████╗██╗     ██╗
██╔══██╗██╔════╝██╔══██╗██║██╔════╝██╔════╝██║     ██║
██████╔╝█████╗  ██║  ██║██║███████╗██║     ██║     ██║
██╔══██╗██╔══╝  ██║  ██║██║╚════██║██║     ██║     ██║
██║  ██║███████╗██████╔╝██║███████║╚██████╗███████╗██║
╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝ ╚═════╝╚══════╝╚═╝
                                         By Carlos Guajardo C.

rediscli - Redis CLI Tool

USAGE:
    rediscli [command] [args...]
    rediscli                    (starts interactive TUI mode)

EXAMPLES:
    rediscli PING
    rediscli SET mykey "Hello Redis"
    rediscli GET mykey
    rediscli KEYS "*"
    rediscli HSET user:1 name "John Doe"
    rediscli LPUSH mylist item1 item2 item3

ENVIRONMENT VARIABLES:
    REDIS_HOST      Redis host (required)
    REDIS_PORT      Redis port (default: 6379)
    REDIS_PASSWORD  Redis password (optional)
    REDIS_DB        Redis database number (default: 0)

OPTIONS:
    -h, --help      Show this help message
    -v, --version   Show version information

INTERACTIVE MODE:
    When run without arguments, rediscli starts in interactive TUI mode.
    Navigate with j/k or up/down arrows, confirm with enter, cancel with esc.

    Menu options:

    1. Query Redis
       Execute any Redis command interactively and display the result.
       Example input: GET mykey   or   HGETALL user:1

    2. Publish Create
       Create a test record in Redis and publish it to a channel.
       Supports two modes:
         Default  - uses built-in place_code, service_name and custom_params.
         Custom   - prompts for:
                      place_code    (numeric, optional — defaults to "0")
                      service_name  (required)
                      custom_params (optional)
       After setting the mode you are asked for:
         - Number of iterations  (how many records to create/publish)
         - Delay between iterations (e.g. 1s, 500ms, 2m) — only when > 1 iteration
       While running, the progress screen shows the parameters entered above,
       a progress bar, iteration count, elapsed time and estimated time remaining.

    3. Publish Create & Delete
       Same flow as "Publish Create", but after each publish the key is deleted
       and a second publish is fired to signal the deletion.

    4. Redis Explorer
       A full-screen TUI browser for your Redis databases.

       DB Selector
         ↑/↓/←→  navigate the 4x4 grid of databases (DB 0-15)
         hjkl     vim-style navigation
         enter    open the selected database
         q/esc    return to the main menu

       Key-Value Table
         ↑/↓      move the row cursor
         enter    open the full value viewer for the selected key
         /        open the filter bar (filters by key or value, live)
         r        refresh — re-scan the current database
         D        jump back to the DB selector
         esc      clear active filter, or go back to the DB selector
         q        exit the explorer

       Filter Bar  (activated with /)
         type     narrow the table in real time (case-insensitive)
         enter    confirm and return focus to the table
         esc      cancel and clear the filter

       Value Viewer
         esc/enter/backspace   back to the table
         q                     exit the explorer

LOGGING:
    In TUI mode every session is logged to logs/rediscli_<timestamp>.log.
    The log file path is printed when the session ends.
`
	fmt.Print(help)
}

type (
	errMsg error
)
