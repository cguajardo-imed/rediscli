package main

import (
	"testing"
)

// ───────────────────────────────────────────────────────────────────────────
// Fingerprint generation tests
// ───────────────────────────────────────────────────────────────────────────

// TestGenerateFingerprint_ValidFormat verifies that the generated fingerprint
// matches the documented SHA-512 format (128 hex characters).
func TestGenerateFingerprint_ValidFormat(t *testing.T) {
	fp := generateFingerprint()

	// SHA-512 produces 64 bytes = 128 hex characters
	if len(fp) != 128 {
		t.Errorf("Expected fingerprint length 128 (SHA-512), got %d", len(fp))
	}

	// Verify all characters are valid hex (0-9, a-f)
	for i, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Character at position %d is not lowercase hex: %q", i, c)
		}
	}
}

// TestGenerateFingerprint_Uniqueness verifies that multiple calls produce
// different fingerprints (due to UUID and timestamp components).
func TestGenerateFingerprint_Uniqueness(t *testing.T) {
	const iterations = 100
	seen := make(map[string]bool, iterations)

	for i := 0; i < iterations; i++ {
		fp := generateFingerprint()
		if seen[fp] {
			t.Errorf("Duplicate fingerprint detected at iteration %d: %s", i, fp)
		}
		seen[fp] = true
	}

	if len(seen) != iterations {
		t.Errorf("Expected %d unique fingerprints, got %d", iterations, len(seen))
	}
}

// TestGenerateFingerprint_NonEmpty verifies that the function never returns
// an empty string.
func TestGenerateFingerprint_NonEmpty(t *testing.T) {
	for i := 0; i < 10; i++ {
		fp := generateFingerprint()
		if fp == "" {
			t.Errorf("Iteration %d: fingerprint is empty", i)
		}
	}
}

// TestGenerateFingerprint_LowercaseHex verifies that the hash output is in
// lowercase hex format (not uppercase or mixed case).
func TestGenerateFingerprint_LowercaseHex(t *testing.T) {
	fp := generateFingerprint()

	for i, c := range fp {
		if c >= 'A' && c <= 'F' {
			t.Errorf("Character at position %d is uppercase hex: %q (expected lowercase)", i, c)
		}
	}
}

// TestGenerateFingerprint_Deterministic verifies that the function produces
// consistent output format across multiple calls (even though values differ).
func TestGenerateFingerprint_Deterministic(t *testing.T) {
	const iterations = 50

	for i := 0; i < iterations; i++ {
		fp := generateFingerprint()

		// All should be exactly 128 chars
		if len(fp) != 128 {
			t.Errorf("Iteration %d: length mismatch (expected 128, got %d)", i, len(fp))
		}

		// All should be valid hex
		for j, c := range fp {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Iteration %d, pos %d: invalid hex char %q", i, j, c)
				break
			}
		}
	}
}

// TestGenerateFingerprint_NoLeadingTrailingWhitespace verifies no whitespace
// padding exists in the output.
func TestGenerateFingerprint_NoLeadingTrailingWhitespace(t *testing.T) {
	fp := generateFingerprint()

	// Check first and last characters are valid hex (no whitespace)
	if fp[0] < '0' || (fp[0] > '9' && fp[0] < 'a') || fp[0] > 'f' {
		t.Errorf("First character is not valid hex: %q", fp[0])
	}
	if fp[127] < '0' || (fp[127] > '9' && fp[127] < 'a') || fp[127] > 'f' {
		t.Errorf("Last character is not valid hex: %q", fp[127])
	}
}

// TestGenerateFingerprint_SHA512Compliance verifies the output matches the
// expected SHA-512 hex encoding length.
func TestGenerateFingerprint_SHA512Compliance(t *testing.T) {
	fp := generateFingerprint()

	// SHA-512 = 512 bits = 64 bytes = 128 hex digits
	const expectedLength = 128
	if len(fp) != expectedLength {
		t.Errorf("SHA-512 fingerprint must be %d hex chars, got %d", expectedLength, len(fp))
	}
}
