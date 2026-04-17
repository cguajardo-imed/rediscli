package main

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// browserFingerprint represents the structured data before final hashing
type browserFingerprint struct {
	Origin            string `json:"origin"`
	UserAgent         string `json:"userAgent"`
	Language          string `json:"language"`
	ScreenResolution  string `json:"screenResolution"`
	ColorDepth        string `json:"colorDepth"`
	PixelRatio        string `json:"pixelRatio"`
	TimeZoneOffset    string `json:"timeZoneOffset"`
	CanvasFingerprint string `json:"canvasFingerprint"`
	AudioFingerprint  string `json:"audioFingerprint"`
	Webdriver         bool   `json:"webdriver"`
}

// generateFingerprint creates a SHA-512 based fingerprint (128 hex characters).
// This simulates the browser fingerprint generation documented in docs/fingerprint.md.
func generateFingerprint() string {
	// Random source for variation
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Simulate origin (common local/dev/production origins)
	origins := []string{
		"http://localhost:3000",
		"http://localhost:8080",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"https://app.example.com",
		"https://staging.example.com",
		"https://www.example.com",
		"https://dashboard.example.com",
		"https://api.example.com",
	}
	origin := origins[rng.Intn(len(origins))]

	// Simulate user agents (common browsers and platforms)
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
	}
	userAgent := userAgents[rng.Intn(len(userAgents))]

	// Simulate languages (common locale codes)
	languages := []string{
		"en-US",
		"en-GB",
		"es-ES",
		"es-MX",
		"fr-FR",
		"de-DE",
		"ja-JP",
		"pt-BR",
		"it-IT",
		"zh-CN",
		"ko-KR",
		"ru-RU",
	}
	language := languages[rng.Intn(len(languages))]

	// Simulate screen resolutions (common display sizes)
	resolutions := []string{
		"1920x1080", // Full HD
		"2560x1440", // QHD
		"1366x768",  // HD
		"1536x864",  // HD+
		"1440x900",  // WXGA+
		"3840x2160", // 4K
		"2880x1800", // Retina
		"1280x720",  // HD
		"1600x900",  // HD+
	}
	screenResolution := resolutions[rng.Intn(len(resolutions))]

	// Simulate color depth (common values)
	colorDepths := []string{"24", "32", "30"}
	colorDepth := colorDepths[rng.Intn(len(colorDepths))]

	// Simulate pixel ratio (common device pixel ratios)
	pixelRatios := []string{"1", "1.25", "1.5", "2", "2.5", "3"}
	pixelRatio := pixelRatios[rng.Intn(len(pixelRatios))]

	// Simulate timezone offsets (common UTC offsets in minutes)
	offsets := []string{
		"-480", // PST (UTC-8)
		"-420", // MST (UTC-7)
		"-360", // CST (UTC-6)
		"-300", // EST (UTC-5)
		"-180", // BRT (UTC-3)
		"0",    // UTC/GMT
		"60",   // CET (UTC+1)
		"120",  // EET (UTC+2)
		"180",  // MSK (UTC+3)
		"330",  // IST (UTC+5:30)
		"480",  // CST (UTC+8)
		"540",  // JST (UTC+9)
	}
	timeZoneOffset := offsets[rng.Intn(len(offsets))]

	// Simulate canvas fingerprint (hash of UUID to simulate canvas rendering)
	canvasHash := sha512.Sum512(fmt.Appendf(nil, "canvas:%s", uuid.New().String()))
	canvasFingerprint := hex.EncodeToString(canvasHash[:])

	// Simulate audio fingerprint (hash of UUID to simulate audio context)
	audioHash := sha512.Sum512(fmt.Appendf(nil, "audio:%s", uuid.New().String()))
	audioFingerprint := hex.EncodeToString(audioHash[:])

	// Webdriver is typically false for regular browsers
	webdriver := false

	// Build the browser fingerprint structure
	fp := browserFingerprint{
		Origin:            origin,
		UserAgent:         userAgent,
		Language:          language,
		ScreenResolution:  screenResolution,
		ColorDepth:        colorDepth,
		PixelRatio:        pixelRatio,
		TimeZoneOffset:    timeZoneOffset,
		CanvasFingerprint: canvasFingerprint,
		AudioFingerprint:  audioFingerprint,
		Webdriver:         webdriver,
	}

	// Marshal to JSON
	payload, err := json.Marshal(fp)
	if err != nil {
		// Fallback to simple UUID-based fingerprint if marshaling fails
		fallbackHash := sha512.Sum512([]byte(uuid.New().String()))
		return hex.EncodeToString(fallbackHash[:])
	}

	// Hash the complete payload with SHA-512
	hash := sha512.Sum512(payload)
	return hex.EncodeToString(hash[:])
}
