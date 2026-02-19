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
	help := `rediscli - Redis CLI Tool

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
    When run without arguments, rediscli starts in interactive TUI mode
    with the following options:
    - Query Redis: Execute custom Redis commands
    - Publish create: Create and publish a test record
    - Publish create & delete: Create, publish, then delete a test record
`
	fmt.Print(help)
}

type (
	errMsg error
)
