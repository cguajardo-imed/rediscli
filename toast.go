package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToastType represents the type/severity of a toast notification
type ToastType string

const (
	ToastInfo    ToastType = "info"
	ToastSuccess ToastType = "success"
	ToastWarning ToastType = "warning"
	ToastError   ToastType = "error"
)

// Toast represents a single toast notification
type Toast struct {
	Message   string
	Type      ToastType
	CreatedAt time.Time
	Duration  time.Duration
	ID        int
}

// ToastManager manages the stack of active toasts
type ToastManager struct {
	toasts    []Toast
	nextID    int
	maxToasts int
}

// toastExpiredMsg is sent when a toast's duration expires
type toastExpiredMsg struct {
	id int
}

// Global toast manager instance
var globalToastManager = NewToastManager()

// NewToastManager creates a new toast manager
func NewToastManager() *ToastManager {
	return &ToastManager{
		toasts:    []Toast{},
		nextID:    0,
		maxToasts: 5, // Maximum number of toasts to show at once
	}
}

// Add adds a new toast to the stack
func (tm *ToastManager) Add(message string, toastType ToastType) tea.Cmd {
	toast := Toast{
		Message:   message,
		Type:      toastType,
		CreatedAt: time.Now(),
		Duration:  3 * time.Second, // Default 3 seconds
		ID:        tm.nextID,
	}
	tm.nextID++

	// Add to the beginning (newest on top)
	tm.toasts = append([]Toast{toast}, tm.toasts...)

	// Trim if we exceed max toasts
	if len(tm.toasts) > tm.maxToasts {
		tm.toasts = tm.toasts[:tm.maxToasts]
	}

	// Return a command to dismiss this toast after its duration
	return tea.Tick(toast.Duration, func(t time.Time) tea.Msg {
		return toastExpiredMsg{id: toast.ID}
	})
}

// Remove removes a toast by ID
func (tm *ToastManager) Remove(id int) {
	for i, toast := range tm.toasts {
		if toast.ID == id {
			tm.toasts = append(tm.toasts[:i], tm.toasts[i+1:]...)
			return
		}
	}
}

// Clear removes all toasts
func (tm *ToastManager) Clear() {
	tm.toasts = []Toast{}
}

// Count returns the number of active toasts
func (tm *ToastManager) Count() int {
	return len(tm.toasts)
}

// RemoveMostRecent removes the most recent (top) toast
func (tm *ToastManager) RemoveMostRecent() bool {
	if len(tm.toasts) == 0 {
		return false
	}
	tm.toasts = tm.toasts[1:]
	return true
}

// GetToastIDAt returns the toast ID at the given index (0 = most recent)
func (tm *ToastManager) GetToastIDAt(index int) int {
	if index < 0 || index >= len(tm.toasts) {
		return -1
	}
	return tm.toasts[index].ID
}

// View renders all active toasts
func (tm *ToastManager) View() string {
	if len(tm.toasts) == 0 {
		return ""
	}

	var rendered string
	for i, toast := range tm.toasts {
		if i > 0 {
			rendered += "\n"
		}
		rendered += tm.renderToast(toast)
	}

	return rendered
}

// renderToast renders a single toast with appropriate styling
// The rendered toast includes a clickable zone with the toast ID as the zone ID
func (tm *ToastManager) renderToast(toast Toast) string {
	var style lipgloss.Style

	// Base style for all toasts
	baseStyle := lipgloss.NewStyle().
		Padding(0, 2).
		MarginBottom(1).
		Width(50).
		Bold(true)

	// Apply type-specific styling
	switch toast.Type {
	case ToastInfo:
		style = baseStyle.
			Foreground(lipgloss.Color("39")). // Blue
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39"))
	case ToastSuccess:
		style = baseStyle.
			Foreground(lipgloss.Color("42")). // Green
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42"))
	case ToastWarning:
		style = baseStyle.
			Foreground(lipgloss.Color("214")). // Orange
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214"))
	case ToastError:
		style = baseStyle.
			Foreground(lipgloss.Color("196")). // Red
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196"))
	default:
		style = baseStyle.
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
	}

	// Add icon/prefix based on type
	var prefix string
	switch toast.Type {
	case ToastInfo:
		prefix = "ℹ "
	case ToastSuccess:
		prefix = "✓ "
	case ToastWarning:
		prefix = "⚠ "
	case ToastError:
		prefix = "✗ "
	}

	return style.Render(prefix + toast.Message)
}

// HandleToastExpired handles the toastExpiredMsg
func (tm *ToastManager) HandleToastExpired(msg toastExpiredMsg) {
	tm.Remove(msg.id)
}

// MakeToast is the main function to display a toast notification
// Usage: cmd := MakeToast("Operation successful", "success")
func MakeToast(message string, toastType string) tea.Cmd {
	var tt ToastType
	switch toastType {
	case "info":
		tt = ToastInfo
	case "success":
		tt = ToastSuccess
	case "warning":
		tt = ToastWarning
	case "error":
		tt = ToastError
	default:
		tt = ToastInfo // Default to info
	}

	return globalToastManager.Add(message, tt)
}

// MakeToastWithDuration creates a toast with a custom duration
func MakeToastWithDuration(message string, toastType string, duration time.Duration) tea.Cmd {
	var tt ToastType
	switch toastType {
	case "info":
		tt = ToastInfo
	case "success":
		tt = ToastSuccess
	case "warning":
		tt = ToastWarning
	case "error":
		tt = ToastError
	default:
		tt = ToastInfo
	}

	toast := Toast{
		Message:   message,
		Type:      tt,
		CreatedAt: time.Now(),
		Duration:  duration,
		ID:        globalToastManager.nextID,
	}
	globalToastManager.nextID++

	globalToastManager.toasts = append([]Toast{toast}, globalToastManager.toasts...)

	if len(globalToastManager.toasts) > globalToastManager.maxToasts {
		globalToastManager.toasts = globalToastManager.toasts[:globalToastManager.maxToasts]
	}

	return tea.Tick(toast.Duration, func(t time.Time) tea.Msg {
		return toastExpiredMsg{id: toast.ID}
	})
}

// GetGlobalToastManager returns the global toast manager instance
func GetGlobalToastManager() *ToastManager {
	return globalToastManager
}

// ClearAllToasts removes all active toasts
func ClearAllToasts() {
	globalToastManager.Clear()
}

// DismissMostRecentToast removes the most recent toast
func DismissMostRecentToast() bool {
	return globalToastManager.RemoveMostRecent()
}

// GetToastCount returns the number of active toasts
func GetToastCount() int {
	return globalToastManager.Count()
}

// ToastMessage represents a toast message to be displayed
type ToastMessage struct {
	Message string
	Type    string
}

// MakeToasts creates multiple toasts at once and returns a batch command
// Usage: MakeToasts(
//
//	ToastMessage{"First message", "success"},
//	ToastMessage{"Second message", "info"},
//
// )
func MakeToasts(messages ...ToastMessage) tea.Cmd {
	var cmds []tea.Cmd
	for _, msg := range messages {
		cmds = append(cmds, MakeToast(msg.Message, msg.Type))
	}
	return tea.Batch(cmds...)
}
