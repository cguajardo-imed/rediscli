# Skill Registry — gns-cli

**Generated**: 2026-08-24
**Project**: gns-cli (Go + Bubbletea TUI)
**Scope**: User-level skills available in this session

---

## Skill Catalog

### Applicable to gns-cli

These skills are auto-loaded based on context detection (Go code, Bubbletea TUI, etc.):

| Skill | Purpose | Auto-load Trigger |
|-------|---------|-------------------|
| **go-testing** | Go testing patterns, Bubbletea TUI testing with teatest | Writing Go tests, Bubbletea testing |
| **branch-pr** | PR creation workflow following issue-first system | Creating pull requests |
| **issue-creation** | GitHub issue creation workflow | Reporting bugs, requesting features |

### General-Purpose Skills

| Skill | Purpose | When to Use |
|-------|---------|------------|
| **skill-creator** | Create new AI agent skills | Adding new agent instructions |
| **judgment-day** | Parallel adversarial review with dual judges | `judgment day`, `doble review` |
| **imed-changelog** | Generate CHANGELOG from commits | `generate changelog` |
| **imed-pr** | Generate PR description in Spanish | `generate PR description` |
| **imed-design-system-ui** | IMED design system UI components | IMED product projects (not this project) |

---

## Project-Specific Conventions

### Stack
- **Language**: Go 1.26.3
- **Framework**: Charmbracelet Bubbletea v1.3.10 (TUI)
- **Testing**: Go built-in + miniredis (Redis mock)
- **Build**: Makefile (Linux/macOS) + build.bat (Windows)

### Key Conventions
- **Tests**: Colocated with source files (`*_test.go`)
- **Test Runner**: `make test` → `go test -v -race -coverprofile=coverage.out ./...`
- **Linter**: `go fmt` (no golangci-lint configured)
- **Type Checking**: `go build` (compile-time only)
- **TDD Mode**: ✅ ENABLED (test runner + mocking framework available)

### Test Helpers Pattern
```go
func setupRedisContainer(t *testing.T) (*miniredis.Miniredis, *redis.Client, context.Context)
```
- Uses miniredis for isolated Redis testing
- Includes context for async operations
- Suppresses Redis client warnings during tests

### Subtests Pattern
```go
tests := []struct {
    name     string
    setup    func() error
    command  []interface{}
    validate func(result interface{}) error
}{}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

---

## How to Load Skills

**Auto-load** (by context):
- Writing Go tests → `go-testing` loads automatically
- Creating PRs → `branch-pr` loads automatically
- Reporting issues → `issue-creation` loads automatically

**Manual load**:
```
/skill go-testing
/skill branch-pr
```

---

## Notes

- No project-level skills found (`.claude/`, `.config/`, `.agent/`, `skills/` are empty)
- All skills are from user-level directories
- Skill registry is infrastructure-grade (independent of SDD mode)
- Use `skill-registry` command to regenerate this file when skills are added/removed
