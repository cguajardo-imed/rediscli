package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	githubOwner   = "cguajardo-imed"
	githubRepo    = "rediscli"
	githubAPIBase = "https://api.github.com"
	releasesBase  = "https://github.com/" + githubOwner + "/" + githubRepo + "/releases/download"
)

// githubRelease represents the relevant fields from the GitHub releases API response.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

// UpdateResult holds the outcome of a self-update operation.
type UpdateResult struct {
	PreviousVersion string
	NewVersion      string
	AlreadyLatest   bool
}

// fetchLatestRelease calls the GitHub releases API and returns the latest release info.
func fetchLatestRelease() (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBase, githubOwner, githubRepo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "rediscli/"+Version)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	if release.TagName == "" {
		return nil, fmt.Errorf("no releases found in repository")
	}

	return &release, nil
}

// platformBinaryName returns the asset filename for the current OS/arch combination.
func platformBinaryName() (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return "rediscli-linux-amd64", nil
		case "arm64":
			return "rediscli-linux-arm64", nil
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "rediscli-darwin-amd64", nil
		case "arm64":
			return "rediscli-darwin-arm64", nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return "rediscli-windows-amd64.exe", nil
		}
	}

	return "", fmt.Errorf("unsupported platform: %s/%s", goos, goarch)
}

// downloadFile downloads the content at url and writes it to dest, returning
// the number of bytes written.
func downloadFile(url, dest string) (int64, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return 0, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("server returned status %d for %s", resp.StatusCode, url)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to write downloaded content: %w", err)
	}

	return n, nil
}

// fetchChecksum downloads the .sha256 file for the given asset and returns the
// expected hex digest (lowercase, 64 chars).
func fetchChecksum(checksumURL string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(checksumURL) //nolint:noctx
	if err != nil {
		return "", fmt.Errorf("failed to fetch checksum: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum file not available (status %d)", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read checksum file: %w", err)
	}

	// sha256sum format: "<hex>  <filename>\n"
	line := strings.TrimSpace(string(body))
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", fmt.Errorf("checksum file is empty or malformed")
	}

	digest := strings.ToLower(parts[0])
	if len(digest) != 64 {
		return "", fmt.Errorf("unexpected checksum length %d (expected 64)", len(digest))
	}

	return digest, nil
}

// verifySHA256 computes the SHA-256 digest of the file at path and compares it
// against expected (lowercase hex). Returns nil when they match.
func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file for verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}

	got := fmt.Sprintf("%x", h.Sum(nil))
	if got != expected {
		return fmt.Errorf("checksum mismatch\n  expected: %s\n  got:      %s", expected, got)
	}

	return nil
}

// selfExecutablePath returns the absolute path of the currently running binary.
func selfExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine current executable path: %w", err)
	}
	// Resolve symlinks so we replace the real file, not a symlink.
	return filepath.EvalSymlinks(exe)
}

// atomicReplace replaces the file at destPath with the file at srcPath in a
// way that is safe even if src and dest are on the same filesystem:
//  1. Rename the current binary to a backup name (so it can still run).
//  2. Move the downloaded binary into place.
//  3. Remove the backup on success (best-effort).
func atomicReplace(srcPath, destPath string) error {
	backupPath := destPath + ".bak"

	// Remove any stale backup from a previous failed update.
	_ = os.Remove(backupPath)

	// Back up the current binary.
	if err := os.Rename(destPath, backupPath); err != nil {
		return fmt.Errorf("failed to back up current binary: %w", err)
	}

	// Move the new binary into place.
	if err := os.Rename(srcPath, destPath); err != nil {
		// Try to restore the backup before giving up.
		_ = os.Rename(backupPath, destPath)
		return fmt.Errorf("failed to move new binary into place: %w", err)
	}

	// Ensure the binary is executable.
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permission: %w", err)
	}

	// Remove backup — best-effort, non-fatal.
	_ = os.Remove(backupPath)

	return nil
}

// SelfUpdate checks for a newer release on GitHub, downloads it for the
// current platform, verifies its checksum, and atomically replaces the running
// binary. It returns an UpdateResult describing what happened.
//
// progressFn is called with human-readable status messages while the update
// proceeds. Pass nil to suppress progress output.
func SelfUpdate(progressFn func(msg string)) (*UpdateResult, error) {
	progress := func(msg string) {
		if progressFn != nil {
			progressFn(msg)
		}
	}

	// ── 1. Resolve current executable ──────────────────────────
	progress("Resolving current executable path…")
	exePath, err := selfExecutablePath()
	if err != nil {
		return nil, err
	}

	// ── 2. Fetch latest release from GitHub ────────────────────
	progress("Fetching latest release from GitHub…")
	release, err := fetchLatestRelease()
	if err != nil {
		return nil, err
	}

	latestVersion := release.TagName
	currentVersion := Version

	// ── 3. Compare versions ────────────────────────────────────
	if latestVersion == currentVersion {
		return &UpdateResult{
			PreviousVersion: currentVersion,
			NewVersion:      latestVersion,
			AlreadyLatest:   true,
		}, nil
	}

	progress(fmt.Sprintf("New version available: %s (current: %s)", latestVersion, currentVersion))

	// ── 4. Determine platform asset name ───────────────────────
	binaryName, err := platformBinaryName()
	if err != nil {
		return nil, err
	}

	binaryURL := fmt.Sprintf("%s/%s/%s", releasesBase, latestVersion, binaryName)
	checksumURL := binaryURL + ".sha256"

	// ── 5. Fetch expected checksum ─────────────────────────────
	progress("Fetching checksum file…")
	expectedChecksum, err := fetchChecksum(checksumURL)
	if err != nil {
		// Non-fatal: warn and skip verification.
		progress(fmt.Sprintf("Warning: could not fetch checksum (%v) — skipping verification", err))
		expectedChecksum = ""
	}

	// ── 6. Download new binary to a temp file ─────────────────
	progress(fmt.Sprintf("Downloading %s…", binaryURL))

	tmpFile, err := os.CreateTemp(filepath.Dir(exePath), "rediscli-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close() // downloadFile will re-open it

	// Ensure cleanup on any failure path.
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	bytesDownloaded, err := downloadFile(binaryURL, tmpPath)
	if err != nil {
		return nil, err
	}

	progress(fmt.Sprintf("Downloaded %.1f KB", float64(bytesDownloaded)/1024))

	// ── 7. Verify checksum ─────────────────────────────────────
	if expectedChecksum != "" {
		progress("Verifying SHA-256 checksum…")
		if err := verifySHA256(tmpPath, expectedChecksum); err != nil {
			return nil, err
		}
		progress("Checksum verified.")
	}

	// ── 8. Atomically replace the binary ──────────────────────
	progress(fmt.Sprintf("Installing to %s…", exePath))
	if err := atomicReplace(tmpPath, exePath); err != nil {
		return nil, err
	}

	progress("Update complete. Restart rediscli to use the new version.")

	return &UpdateResult{
		PreviousVersion: currentVersion,
		NewVersion:      latestVersion,
		AlreadyLatest:   false,
	}, nil
}
