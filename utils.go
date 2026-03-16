package main

import (
	"slices"
	"fmt"
	"strings"
	"time"
)

// ParseDuration parses a duration string with support for common formats
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// FormatBytes formats bytes into human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats a duration into human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// SanitizeKey removes invalid characters from Redis keys
func SanitizeKey(key string) string {
	// Remove leading/trailing spaces
	key = strings.TrimSpace(key)

	// Replace multiple spaces with single space
	key = strings.Join(strings.Fields(key), " ")

	return key
}

// ValidateCommand checks if a Redis command is valid
func ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("invalid command format")
	}

	// Check for dangerous commands that shouldn't be executed
	dangerousCommands := map[string]bool{
		"FLUSHDB":  true,
		"FLUSHALL": true,
		"SHUTDOWN": true,
		"CONFIG":   true,
		"DEBUG":    true,
	}

	cmdName := strings.ToUpper(parts[0])
	if dangerousCommands[cmdName] {
		return fmt.Errorf("dangerous command '%s' is not allowed in interactive mode", cmdName)
	}

	return nil
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// IsValidPort checks if a port number is valid
func IsValidPort(port string) bool {
	if port == "" {
		return false
	}

	// Simple validation - check if it's numeric and in valid range
	var p int
	_, err := fmt.Sscanf(port, "%d", &p)
	if err != nil {
		return false
	}

	return p > 0 && p <= 65535
}

// ParseKeyValuePairs parses key-value pairs from a string slice
func ParseKeyValuePairs(args []string) (map[string]string, error) {
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("key-value pairs must have even number of arguments")
	}

	result := make(map[string]string)
	for i := 0; i < len(args); i += 2 {
		result[args[i]] = args[i+1]
	}

	return result, nil
}

// ContainsString checks if a string exists in a slice
func ContainsString(slice []string, str string) bool {
	return slices.Contains(slice, str)
}

// IsInteractive checks if the application is running in interactive mode
func IsInteractive() bool {
	// Always return false for now - can be enhanced later
	return false
}

// ColorString wraps a string with ANSI color codes
func ColorString(s string, colorCode int) string {
	return fmt.Sprintf("\033[%dm%s\033[0m", colorCode, s)
}

// Common color codes
const (
	ColorReset   = 0
	ColorRed     = 31
	ColorGreen   = 32
	ColorYellow  = 33
	ColorBlue    = 34
	ColorMagenta = 35
	ColorCyan    = 36
	ColorWhite   = 37
)

// FormatError formats an error message with color
func FormatError(err error) string {
	return ColorString(fmt.Sprintf("Error: %v", err), ColorRed)
}

// FormatSuccess formats a success message with color
func FormatSuccess(msg string) string {
	return ColorString(msg, ColorGreen)
}

// FormatWarning formats a warning message with color
func FormatWarning(msg string) string {
	return ColorString(msg, ColorYellow)
}

// FormatInfo formats an info message with color
func FormatInfo(msg string) string {
	return ColorString(msg, ColorCyan)
}

// EscapeQuotes escapes quotes in a string for Redis commands
func EscapeQuotes(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// UnescapeQuotes unescapes quotes in a string from Redis responses
func UnescapeQuotes(s string) string {
	s = strings.ReplaceAll(s, "\\\"", "\"")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}

// ParseRedisURL parses a Redis URL (redis://host:port/db)
func ParseRedisURL(url string) (host, port string, db int, err error) {
	if !strings.HasPrefix(url, "redis://") {
		return "", "", 0, fmt.Errorf("invalid Redis URL format")
	}

	url = strings.TrimPrefix(url, "redis://")

	// Split by /
	parts := strings.Split(url, "/")
	hostPort := parts[0]

	// Parse database
	db = 0
	if len(parts) > 1 {
		_, err = fmt.Sscanf(parts[1], "%d", &db)
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid database number: %w", err)
		}
	}

	// Parse host and port
	if strings.Contains(hostPort, ":") {
		hostParts := strings.Split(hostPort, ":")
		if len(hostParts) != 2 {
			return "", "", 0, fmt.Errorf("invalid host:port format")
		}
		host = hostParts[0]
		port = hostParts[1]
	} else {
		host = hostPort
		port = "6379"
	}

	return host, port, db, nil
}

// RetryWithBackoff retries a function with exponential backoff
func RetryWithBackoff(maxRetries int, initialDelay time.Duration, fn func() error) error {
	var err error
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}

	return fmt.Errorf("max retries exceeded: %w", err)
}
