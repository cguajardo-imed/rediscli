package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	redis "github.com/redis/go-redis/v9"
)

// ─────────────────────────────────────────────────────────────
// Styles
// ─────────────────────────────────────────────────────────────

var (
	exTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	exSubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	exSelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("79"))

	exActiveDBStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214"))

	exHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238"))

	exErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	exStatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	exBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	exTableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212")).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				BorderBottom(true)

	exTableCellStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	exTableSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57"))

	exMargin = lipgloss.NewStyle().MarginLeft(2)
)

// ─────────────────────────────────────────────────────────────
// Explorer phases
// ─────────────────────────────────────────────────────────────

type explorerPhase int

const (
	phaseDBSelect  explorerPhase = iota // choose a Redis DB (0-15)
	phaseKeyTable                       // browse keys in the selected DB
	phaseValueView                      // full-screen value for a selected key
)

// ─────────────────────────────────────────────────────────────
// Async message types
// ─────────────────────────────────────────────────────────────

// explorerDoneMsg is sent to the parent program when the user exits the explorer.
type explorerDoneMsg struct{}

// keysLoadedMsg carries the result of a full key scan on a DB.
type keysLoadedMsg struct {
	db      int
	entries []kvEntry
	err     error
}

// dbSwitchedMsg signals that a dedicated explorer Redis client has been created
// for the requested DB.
type dbSwitchedMsg struct {
	db  int
	err error
}

// ─────────────────────────────────────────────────────────────
// Data model
// ─────────────────────────────────────────────────────────────

// kvEntry holds all metadata for a single Redis key.
type kvEntry struct {
	key     string
	keyType string
	ttl     string
	value   string // full value (may be large)
}

// explorerRedisClient is a *redis.Client dedicated to the explorer so that
// switching DBs (SELECT) never affects the global client used by the rest of
// the application.
var explorerRedisClient *redis.Client

// ─────────────────────────────────────────────────────────────
// Explorer model
// ─────────────────────────────────────────────────────────────

// explorerModel is a fully self-contained Bubble Tea model.
// The parent (views.go) embeds it and delegates updates when
// model.explorerActive is true.
type explorerModel struct {
	phase           explorerPhase
	dbCursor        int         // currently highlighted DB in the selector grid
	currentDB       int         // DB that is currently loaded (-1 = none)
	entries         []kvEntry   // all keys loaded for currentDB (never mutated after load)
	filteredEntries []kvEntry   // entries after applying the current filter
	tbl             table.Model // bubbles/table for key browsing
	winWidth        int
	winHeight       int
	loading         bool
	err             error
	statusMsg       string // transient one-line message
	valueKey        string // key shown in phaseValueView
	valueBody       string // full value shown in phaseValueView
	filterMode      bool   // true while the user is typing a filter query
	filterQuery     string // current filter input string
}

// newExplorerModel returns an explorer ready to show the DB selector.
func newExplorerModel() explorerModel {
	m := explorerModel{
		phase:     phaseDBSelect,
		dbCursor:  0,
		currentDB: -1,
		winWidth:  80,
		winHeight: 24,
	}
	m.tbl = buildExplorerTable(nil, m.winWidth, m.winHeight)
	m.filteredEntries = nil
	return m
}

// applyFilter recomputes filteredEntries from entries using filterQuery.
// An empty query means "show everything". Matching is case-insensitive and
// checks both the key and the full value.
func (m *explorerModel) applyFilter() {
	if m.filterQuery == "" {
		m.filteredEntries = m.entries
		return
	}
	q := strings.ToLower(m.filterQuery)
	filtered := make([]kvEntry, 0, len(m.entries))
	for _, e := range m.entries {
		if strings.Contains(strings.ToLower(e.key), q) ||
			strings.Contains(strings.ToLower(e.value), q) {
			filtered = append(filtered, e)
		}
	}
	m.filteredEntries = filtered
}

// visibleEntries returns the slice that should currently be shown in the table.
func (m *explorerModel) visibleEntries() []kvEntry {
	if m.filteredEntries != nil {
		return m.filteredEntries
	}
	return m.entries
}

// ─────────────────────────────────────────────────────────────
// Init
// ─────────────────────────────────────────────────────────────

func (m explorerModel) Init() tea.Cmd {
	return nil
}

// ─────────────────────────────────────────────────────────────
// Update – main dispatcher
// ─────────────────────────────────────────────────────────────

func (m explorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.winWidth = msg.Width
		m.winHeight = msg.Height
		if m.phase == phaseKeyTable {
			m.tbl = resizeExplorerTable(m.tbl, m.entries, m.winWidth, m.winHeight)
		}
		return m, nil

	case keysLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.statusMsg = ""
			return m, nil
		}
		m.err = nil
		m.entries = msg.entries
		m.currentDB = msg.db
		// Re-apply any existing filter to the fresh data
		m.applyFilter()
		visible := m.visibleEntries()
		m.tbl = buildExplorerTable(visible, m.winWidth, m.winHeight)
		if len(msg.entries) == 0 {
			m.statusMsg = fmt.Sprintf("DB %d is empty", msg.db)
		} else {
			m.statusMsg = explorerStatusLine(m.filterQuery, len(visible), len(msg.entries))
		}
		m.phase = phaseKeyTable
		return m, nil

	case dbSwitchedMsg:
		if msg.err != nil {
			m.loading = false
			m.err = msg.err
			m.statusMsg = ""
			return m, nil
		}
		// Client is ready; now fetch all keys
		return m, explorerLoadKeysCmd(msg.db)

	case tea.KeyMsg:
		switch m.phase {
		case phaseDBSelect:
			return m.updateDBSelect(msg)
		case phaseKeyTable:
			return m.updateKeyTable(msg)
		case phaseValueView:
			return m.updateValueView(msg)
		}
	}

	// Always forward non-key messages to the table when browsing
	// (but not while the user is typing a filter — the table must not steal input)
	if m.phase == phaseKeyTable && !m.filterMode {
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ─────────────────────────────────────────────────────────────
// Phase-level Update helpers
// ─────────────────────────────────────────────────────────────

func (m explorerModel) updateDBSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		closeExplorerClient()
		return m, func() tea.Msg { return explorerDoneMsg{} }

	case "up", "k":
		if m.dbCursor-4 >= 0 {
			m.dbCursor -= 4
		}
	case "down", "j":
		if m.dbCursor+4 <= 15 {
			m.dbCursor += 4
		}
	case "left", "h":
		if m.dbCursor > 0 {
			m.dbCursor--
		}
	case "right", "l":
		if m.dbCursor < 15 {
			m.dbCursor++
		}

	case "enter", " ":
		m.loading = true
		m.err = nil
		m.statusMsg = fmt.Sprintf("Connecting to DB %d…", m.dbCursor)
		return m, explorerSwitchDBCmd(m.dbCursor)
	}

	return m, nil
}

func (m explorerModel) updateKeyTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Filter input mode ──────────────────────────────────────
	if m.filterMode {
		switch msg.String() {
		case "ctrl+c":
			closeExplorerClient()
			return m, func() tea.Msg { return explorerDoneMsg{} }

		case "esc":
			// Cancel filter — restore full list
			m.filterMode = false
			m.filterQuery = ""
			m.applyFilter()
			visible := m.visibleEntries()
			m.tbl = buildExplorerTable(visible, m.winWidth, m.winHeight)
			m.statusMsg = explorerStatusLine("", len(visible), len(m.entries))
			return m, nil

		case "enter":
			// Confirm filter and return focus to the table
			m.filterMode = false
			return m, nil

		case "backspace":
			if len(m.filterQuery) > 0 {
				m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
				m.applyFilter()
				visible := m.visibleEntries()
				_, _, _, valW := explorerColumnWidths(m.winWidth)
				m.tbl.SetRows(explorerEntriesToRows(visible, valW))
				m.statusMsg = explorerStatusLine(m.filterQuery, len(visible), len(m.entries))
			}
			return m, nil

		default:
			// Accumulate printable characters into the filter query
			key := msg.String()
			if len(key) == 1 {
				m.filterQuery += key
				m.applyFilter()
				visible := m.visibleEntries()
				_, _, _, valW := explorerColumnWidths(m.winWidth)
				m.tbl.SetRows(explorerEntriesToRows(visible, valW))
				m.statusMsg = explorerStatusLine(m.filterQuery, len(visible), len(m.entries))
			}
			return m, nil
		}
	}

	// ── Normal table navigation mode ───────────────────────────
	switch msg.String() {
	case "ctrl+c", "q":
		closeExplorerClient()
		return m, func() tea.Msg { return explorerDoneMsg{} }

	case "esc":
		// If a filter is active, clear it first; otherwise go back to DB selector
		if m.filterQuery != "" {
			m.filterQuery = ""
			m.applyFilter()
			visible := m.visibleEntries()
			m.tbl = buildExplorerTable(visible, m.winWidth, m.winHeight)
			m.statusMsg = explorerStatusLine("", len(visible), len(m.entries))
			return m, nil
		}
		m.phase = phaseDBSelect
		m.statusMsg = ""
		m.err = nil
		return m, nil

	case "D":
		// Change DB — jump back to selector
		m.phase = phaseDBSelect
		m.filterQuery = ""
		m.filteredEntries = nil
		m.statusMsg = ""
		m.err = nil
		return m, nil

	case "/":
		// Activate filter input mode
		m.filterMode = true
		return m, nil

	case "ctrl+f":
		// Alternative shortcut to activate filter
		m.filterMode = true
		return m, nil

	case "r":
		// Refresh current DB (keep filter query)
		if m.currentDB < 0 {
			return m, nil
		}
		m.loading = true
		m.err = nil
		m.statusMsg = fmt.Sprintf("Refreshing DB %d…", m.currentDB)
		return m, explorerLoadKeysCmd(m.currentDB)

	case "enter", " ":
		// Open value viewer for the selected row
		visible := m.visibleEntries()
		if len(visible) == 0 {
			return m, nil
		}
		row := m.tbl.SelectedRow()
		if row == nil {
			return m, nil
		}
		selectedKey := row[0]
		for _, e := range visible {
			if e.key == selectedKey {
				m.valueKey = e.key
				m.valueBody = e.value
				m.phase = phaseValueView
				return m, nil
			}
		}

	default:
		// Delegate navigation to the table (↑↓ PgUp PgDn Home End)
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m explorerModel) updateValueView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		closeExplorerClient()
		return m, func() tea.Msg { return explorerDoneMsg{} }
	case "esc", "enter", "backspace":
		m.phase = phaseKeyTable
		m.valueKey = ""
		m.valueBody = ""
	}
	return m, nil
}

// ─────────────────────────────────────────────────────────────
// View
// ─────────────────────────────────────────────────────────────

func (m explorerModel) View() string {
	switch m.phase {
	case phaseDBSelect:
		return m.viewDBSelect()
	case phaseKeyTable:
		return m.viewKeyTable()
	case phaseValueView:
		return m.viewValueView()
	}
	return ""
}

// ── DB selector ───────────────────────────────────────────────

func (m explorerModel) viewDBSelect() string {
	var b strings.Builder

	b.WriteString(exTitleStyle.Render("Redis Explorer") + "  ")
	b.WriteString(exSubtitleStyle.Render("Select a database"))
	b.WriteString("\n\n")

	// 4 columns × 4 rows = 16 databases
	const cols = 4
	for row := 0; row < 4; row++ {
		for col := 0; col < cols; col++ {
			db := row*cols + col
			cell := fmt.Sprintf(" DB %-2d ", db)

			switch {
			case db == m.dbCursor && db == m.currentDB:
				// Highlighted AND currently loaded
				b.WriteString(exSelectedStyle.Render("►[" + cell + "]"))
			case db == m.dbCursor:
				b.WriteString(exSelectedStyle.Render("► " + cell + " "))
			case db == m.currentDB:
				b.WriteString(exActiveDBStyle.Render("  " + cell + "✓"))
			default:
				b.WriteString(exSubtitleStyle.Render("  " + cell + "  "))
			}

			if col < cols-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.loading {
		b.WriteString(exStatusStyle.Render(m.statusMsg))
	} else if m.err != nil {
		b.WriteString(exErrorStyle.Render("Error: " + m.err.Error()))
	} else if m.statusMsg != "" {
		b.WriteString(exSubtitleStyle.Render(m.statusMsg))
	}

	b.WriteString("\n\n")
	b.WriteString(exHelpStyle.Render(
		"↑↓←→ / hjkl: navigate   enter/space: open   q/esc: back to menu",
	))

	return exMargin.Render(b.String())
}

// ── Key-value table ───────────────────────────────────────────

var (
	exFilterLabelStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	exFilterInputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
	exFilterActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))
	exFilterEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

func (m explorerModel) viewKeyTable() string {
	var b strings.Builder

	header := fmt.Sprintf("Redis Explorer  —  DB %d", m.currentDB)
	b.WriteString(exTitleStyle.Render(header))
	b.WriteString("\n")

	// ── Filter bar ────────────────────────────────────────────
	if m.filterMode {
		cursor := exFilterInputStyle.Render("█")
		query := exFilterInputStyle.Render(m.filterQuery)
		b.WriteString(exFilterLabelStyle.Render("Filter: ") + query + cursor + "\n")
	} else if m.filterQuery != "" {
		b.WriteString(exFilterLabelStyle.Render("Filter: ") +
			exFilterActiveStyle.Render(m.filterQuery) +
			exSubtitleStyle.Render("  (/ to edit  •  esc to clear)") + "\n")
	} else {
		b.WriteString(exSubtitleStyle.Render("Press / to filter by key or value") + "\n")
	}

	// ── Table ─────────────────────────────────────────────────
	if m.loading {
		b.WriteString(exStatusStyle.Render(m.statusMsg) + "\n")
	} else if m.err != nil {
		b.WriteString(exErrorStyle.Render("Error: "+m.err.Error()) + "\n")
	} else if len(m.entries) == 0 {
		b.WriteString(exSubtitleStyle.Render("(database is empty)") + "\n")
	} else if len(m.visibleEntries()) == 0 {
		b.WriteString(exFilterEmptyStyle.Render("No keys match the current filter.") + "\n")
	} else {
		b.WriteString(m.tbl.View() + "\n")
		if m.statusMsg != "" {
			b.WriteString(exSubtitleStyle.Render(m.statusMsg) + "\n")
		}
	}

	// ── Help bar ──────────────────────────────────────────────
	b.WriteString("\n")
	if m.filterMode {
		b.WriteString(exHelpStyle.Render(
			"type to filter   enter: confirm   esc: cancel   backspace: delete char",
		))
	} else {
		b.WriteString(exHelpStyle.Render(
			"↑/↓: navigate   enter: view value   /: filter   r: refresh   D: change DB   esc/q: exit",
		))
	}

	return exMargin.Render(b.String())
}

// ── Full value viewer ─────────────────────────────────────────

func (m explorerModel) viewValueView() string {
	var b strings.Builder

	b.WriteString(exTitleStyle.Render("Redis Explorer  —  Value Viewer"))
	b.WriteString("\n")
	b.WriteString(exSubtitleStyle.Render("Key: ") + exSelectedStyle.Render(m.valueKey))
	b.WriteString("\n\n")

	maxW := m.winWidth - 8
	if maxW < 20 {
		maxW = 20
	}
	wrapped := wordWrap(m.valueBody, maxW)
	b.WriteString(exBorderStyle.Render(wrapped))
	b.WriteString("\n\n")
	b.WriteString(exHelpStyle.Render(
		"esc / enter / backspace: back to table   q: exit explorer",
	))

	return exMargin.Render(b.String())
}

// ─────────────────────────────────────────────────────────────
// bubbles/table helpers
// ─────────────────────────────────────────────────────────────

func explorerColumnWidths(totalWidth int) (keyW, typeW, ttlW, valW int) {
	// exMargin is MarginLeft(2). Our custom cell/header styles have no padding,
	// so the rendered table width == sum of column widths exactly.
	// Subtract 2 so the columns fill the terminal edge-to-edge.
	usable := totalWidth - 2
	if usable < 50 {
		usable = 50
	}
	typeW = 10
	ttlW = 9
	keyW = usable * 32 / 100
	// valW gets every remaining character so the total == usable == termWidth-2.
	valW = usable - keyW - typeW - ttlW
	if valW < 12 {
		valW = 12
	}
	return
}

func buildExplorerTable(entries []kvEntry, w, h int) table.Model {
	keyW, typeW, ttlW, valW := explorerColumnWidths(w)

	cols := []table.Column{
		{Title: "Key", Width: keyW},
		{Title: "Type", Width: typeW},
		{Title: "TTL", Width: ttlW},
		{Title: "Value (preview)", Width: valW},
	}

	rows := explorerEntriesToRows(entries, valW)

	tableHeight := h - 10
	if tableHeight < 3 {
		tableHeight = 3
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight),
	)

	s := table.DefaultStyles()
	s.Header = exTableHeaderStyle
	s.Cell = exTableCellStyle
	s.Selected = exTableSelectedStyle
	t.SetStyles(s)

	return t
}

func resizeExplorerTable(t table.Model, entries []kvEntry, w, h int) table.Model {
	keyW, typeW, ttlW, valW := explorerColumnWidths(w)

	t.SetColumns([]table.Column{
		{Title: "Key", Width: keyW},
		{Title: "Type", Width: typeW},
		{Title: "TTL", Width: ttlW},
		{Title: "Value (preview)", Width: valW},
	})
	t.SetRows(explorerEntriesToRows(entries, valW))

	// +2 for the filter bar line added to the key-table view
	tableHeight := h - 12
	if tableHeight < 3 {
		tableHeight = 3
	}
	t.SetHeight(tableHeight)

	return t
}

// explorerStatusLine builds the status message shown under the table.
func explorerStatusLine(query string, visible, total int) string {
	if query == "" {
		if total == 0 {
			return ""
		}
		return fmt.Sprintf("%d key(s)  (max 500)", total)
	}
	return fmt.Sprintf("%d / %d key(s) match  \"%s\"", visible, total, query)
}

func explorerEntriesToRows(entries []kvEntry, previewWidth int) []table.Row {
	rows := make([]table.Row, len(entries))
	for i, e := range entries {
		preview := strings.ReplaceAll(e.value, "\n", " ")
		preview = strings.ReplaceAll(preview, "\r", "")
		preview = TruncateString(preview, previewWidth)
		rows[i] = table.Row{e.key, e.keyType, e.ttl, preview}
	}
	return rows
}

// ─────────────────────────────────────────────────────────────
// Async Bubble Tea commands
// ─────────────────────────────────────────────────────────────

// explorerSwitchDBCmd creates a dedicated Redis client pointing at `db` and
// pings it. The global `client` is never modified.
func explorerSwitchDBCmd(db int) tea.Cmd {
	return func() tea.Msg {
		// Close any previous explorer client
		closeExplorerClient()

		opts := *client.Options() // copy current global options
		opts.DB = db
		// Use a small pool — explorer only needs one connection
		opts.PoolSize = 3
		opts.MinIdleConns = 1

		c := redis.NewClient(&opts)
		if err := c.Ping(context.Background()).Err(); err != nil {
			_ = c.Close()
			return dbSwitchedMsg{db: db, err: err}
		}
		explorerRedisClient = c
		return dbSwitchedMsg{db: db}
	}
}

// explorerLoadKeysCmd scans all keys (up to 500) in the selected DB and
// retrieves their type, TTL, and value.
func explorerLoadKeysCmd(db int) tea.Cmd {
	return func() tea.Msg {
		c := explorerRedisClient
		if c == nil {
			return keysLoadedMsg{db: db, err: fmt.Errorf("explorer client not initialised")}
		}

		bgCtx := context.Background()

		// Scan keys (capped at 500 to keep the UI responsive)
		const maxKeys = 500
		var keys []string
		iter := c.Scan(bgCtx, 0, "*", 100).Iterator()
		for iter.Next(bgCtx) {
			keys = append(keys, iter.Val())
			if len(keys) >= maxKeys {
				break
			}
		}
		if err := iter.Err(); err != nil {
			return keysLoadedMsg{db: db, err: err}
		}

		// Sort for a stable, predictable display order
		sort.Strings(keys)

		entries := make([]kvEntry, 0, len(keys))
		for _, k := range keys {
			e := kvEntry{key: k}

			// Type
			kType, err := c.Type(bgCtx, k).Result()
			if err != nil {
				kType = "?"
			}
			e.keyType = kType

			// TTL
			dur, err := c.TTL(bgCtx, k).Result()
			switch {
			case err != nil:
				e.ttl = "err"
			case dur == -1:
				e.ttl = "∞" // no expiry
			case dur == -2:
				e.ttl = "gone" // key disappeared between SCAN and TTL
			default:
				e.ttl = FormatDuration(dur)
			}

			// Value — best-effort for the five main Redis types
			e.value = explorerFetchValue(bgCtx, c, k, kType)

			entries = append(entries, e)
		}

		return keysLoadedMsg{db: db, entries: entries}
	}
}

// ─────────────────────────────────────────────────────────────
// Value fetcher – handles all Redis data types
// ─────────────────────────────────────────────────────────────

// explorerFetchValue retrieves a human-readable representation of a Redis key's
// value regardless of its type.
func explorerFetchValue(ctx context.Context, c *redis.Client, key, keyType string) string {
	switch keyType {
	case "string":
		val, err := c.Get(ctx, key).Result()
		if err != nil {
			return fmt.Sprintf("(error: %v)", err)
		}
		return val

	case "hash":
		fields, err := c.HGetAll(ctx, key).Result()
		if err != nil {
			return fmt.Sprintf("(error: %v)", err)
		}
		// Sort field names for deterministic output
		names := make([]string, 0, len(fields))
		for f := range fields {
			names = append(names, f)
		}
		sort.Strings(names)
		var sb strings.Builder
		for _, f := range names {
			sb.WriteString(fmt.Sprintf("%s: %s\n", f, fields[f]))
		}
		return strings.TrimRight(sb.String(), "\n")

	case "list":
		// Show up to 50 elements
		items, err := c.LRange(ctx, key, 0, 49).Result()
		if err != nil {
			return fmt.Sprintf("(error: %v)", err)
		}
		return strings.Join(items, "\n")

	case "set":
		members, err := c.SMembers(ctx, key).Result()
		if err != nil {
			return fmt.Sprintf("(error: %v)", err)
		}
		sort.Strings(members)
		return strings.Join(members, "\n")

	case "zset":
		// Sorted-set: show member + score pairs (up to 50)
		zs, err := c.ZRangeWithScores(ctx, key, 0, 49).Result()
		if err != nil {
			return fmt.Sprintf("(error: %v)", err)
		}
		var sb strings.Builder
		for _, z := range zs {
			sb.WriteString(fmt.Sprintf("%v  (score: %g)\n", z.Member, z.Score))
		}
		return strings.TrimRight(sb.String(), "\n")

	default:
		return fmt.Sprintf("(unsupported type: %s)", keyType)
	}
}

// ─────────────────────────────────────────────────────────────
// Word-wrap helper (used in the value viewer)
// ─────────────────────────────────────────────────────────────

// wordWrap breaks s into lines of at most maxWidth runes. It breaks on spaces
// when possible, otherwise hard-wraps mid-word.
func wordWrap(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}

	var result strings.Builder
	lines := strings.Split(s, "\n")

	for li, line := range lines {
		if li > 0 {
			result.WriteByte('\n')
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		lineLen := 0
		for wi, word := range words {
			// Hard-wrap words that are longer than maxWidth
			for len(word) > maxWidth {
				if lineLen > 0 {
					result.WriteByte('\n')
					lineLen = 0
				}
				result.WriteString(word[:maxWidth])
				result.WriteByte('\n')
				word = word[maxWidth:]
			}
			if len(word) == 0 {
				continue
			}

			needed := len(word)
			if lineLen > 0 {
				needed++ // space before the word
			}

			if lineLen+needed > maxWidth && lineLen > 0 {
				result.WriteByte('\n')
				lineLen = 0
			}

			if lineLen > 0 {
				result.WriteByte(' ')
				lineLen++
			}
			result.WriteString(word)
			lineLen += len(word)
			_ = wi
		}
	}

	return result.String()
}

// ─────────────────────────────────────────────────────────────
// Lifecycle helpers
// ─────────────────────────────────────────────────────────────

// closeExplorerClient gracefully closes the explorer-scoped Redis client.
func closeExplorerClient() {
	if explorerRedisClient != nil {
		_ = explorerRedisClient.Close()
		explorerRedisClient = nil
	}
}

// newRedisClientFromOptions creates a new redis.Client from the given options.
// Kept here so connection.go does not need to be modified.
func newRedisClientFromOptions(opts *redis.Options) *redis.Client {
	return redis.NewClient(opts)
}

// Ensure time import is used (TTL formatting uses time.Duration via FormatDuration)
var _ = time.Second
