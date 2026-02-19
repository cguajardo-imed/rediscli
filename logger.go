package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile     *os.File
	logger      *log.Logger
	logMutex    sync.Mutex
	isTUIMode   bool
	logFilePath string
)

// LogLevel represents the severity of a log message
type LogLevel string

const (
	LogLevelInfo    LogLevel = "INFO"
	LogLevelSuccess LogLevel = "SUCCESS"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
	LogLevelDebug   LogLevel = "DEBUG"
)

// InitLogger initializes the logger based on the mode
func InitLogger(tuiMode bool) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	isTUIMode = tuiMode

	if tuiMode {
		// Create logs directory if it doesn't exist
		logsDir := "logs"
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}

		// Create log file with timestamp
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		logFilePath = filepath.Join(logsDir, fmt.Sprintf("rediscli_%s.log", timestamp))

		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		// Set logger to write to file only in TUI mode
		logger = log.New(logFile, "", 0)
	} else {
		// In CLI mode, write to stdout
		logger = log.New(os.Stdout, "", 0)
	}

	return nil
}

// CloseLogger closes the log file
func CloseLogger() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// LogMessage logs a message with the specified level
func LogMessage(level LogLevel, message string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logger == nil {
		return
	}

	timestamp := time.Now().Format("2006/01/02 15:04:05")
	formattedMsg := fmt.Sprintf("%s [%s] %s", timestamp, level, message)

	logger.Println(formattedMsg)
}

// LogInfo logs an informational message
func LogInfo(message string) {
	LogMessage(LogLevelInfo, message)
}

// LogSuccess logs a success message
func LogSuccess(message string) {
	LogMessage(LogLevelSuccess, message)
}

// LogWarning logs a warning message
func LogWarning(message string) {
	LogMessage(LogLevelWarning, message)
}

// LogError logs an error message
func LogError(message string) {
	LogMessage(LogLevelError, message)
}

// LogDebug logs a debug message
func LogDebug(message string) {
	LogMessage(LogLevelDebug, message)
}

// LogWithIteration logs a message with iteration information
func LogWithIteration(level LogLevel, iteration, total int, message string) {
	if total > 1 {
		message = fmt.Sprintf("[Iteration %d/%d] %s", iteration, total, message)
	}
	LogMessage(level, message)
}

// LogIterationSuccess logs a success message with iteration information
func LogIterationSuccess(iteration, total int, message string) {
	LogWithIteration(LogLevelSuccess, iteration, total, message)
}

// LogIterationError logs an error message with iteration information
func LogIterationError(iteration, total int, message string) {
	LogWithIteration(LogLevelError, iteration, total, message)
}

// GetLogFilePath returns the current log file path
func GetLogFilePath() string {
	logMutex.Lock()
	defer logMutex.Unlock()
	return logFilePath
}

// IsTUIMode returns whether the logger is in TUI mode
func IsTUIMode() bool {
	logMutex.Lock()
	defer logMutex.Unlock()
	return isTUIMode
}

// WriteToLogFile writes directly to the log file (bypassing logger)
func WriteToLogFile(message string) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile == nil {
		return fmt.Errorf("log file not initialized")
	}

	timestamp := time.Now().Format("2006/01/02 15:04:05")
	formattedMsg := fmt.Sprintf("%s %s\n", timestamp, message)

	_, err := io.WriteString(logFile, formattedMsg)
	return err
}

// LogRedisOperation logs a Redis operation with details
func LogRedisOperation(operation, key, channel string, iteration, total int) {
	var message string
	switch operation {
	case "create":
		message = fmt.Sprintf("Created record with key: %s", key)
	case "publish":
		message = fmt.Sprintf("Published key '%s' to channel '%s'", key, channel)
	case "delete":
		message = fmt.Sprintf("Deleted key: %s", key)
	default:
		message = fmt.Sprintf("Operation '%s' on key: %s", operation, key)
	}

	LogIterationSuccess(iteration, total, message)
}

// LogRedisError logs a Redis error with details
func LogRedisError(operation, key string, err error, iteration, total int) {
	message := fmt.Sprintf("Failed to %s key '%s': %v", operation, key, err)
	LogIterationError(iteration, total, message)
}

// LogConnectionInfo logs connection information
func LogConnectionInfo(addr string, db int) {
	message := fmt.Sprintf("Connected to Redis at %s (DB: %d)", addr, db)
	LogInfo(message)
}

// LogSummary logs a summary of operations
func LogSummary(operation string, total, successful, failed int, duration time.Duration) {
	message := fmt.Sprintf(
		"Summary - Operation: %s | Total: %d | Successful: %d | Failed: %d | Duration: %s",
		operation, total, successful, failed, duration,
	)
	LogInfo(message)
	LogInfo("=" + repeat("=", 80))
}

// Helper function to repeat a string
func repeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

// LogBanner logs a banner message
func LogBanner(message string) {
	separator := repeat("=", 80)
	LogInfo(separator)
	LogInfo(fmt.Sprintf("  %s", message))
	LogInfo(separator)
}

// LogConfig logs configuration information
func LogConfig(config *Config) {
	LogBanner("Redis Configuration")
	LogInfo(fmt.Sprintf("Host: %s", config.Host))
	LogInfo(fmt.Sprintf("Port: %s", config.Port))
	LogInfo(fmt.Sprintf("Database: %d", config.DB))
	LogInfo(fmt.Sprintf("Pool Size: %d", config.PoolSize))
	LogInfo(fmt.Sprintf("Min Idle Connections: %d", config.MinIdleConns))
	LogInfo(repeat("=", 80))
}
