package shortener

import (
	"fmt"
	"math/big"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator()

	if gen == nil {
		t.Fatal("NewGenerator() returned nil")
	}

	if gen.GetCodeLength() != defaultCodeLength {
		t.Errorf("Expected code length %d, got %d", defaultCodeLength, gen.GetCodeLength())
	}
}

func TestNewGeneratorWithLength(t *testing.T) {
	tests := []struct {
		name        string
		length      int
		expectError bool
	}{
		{"valid length 4", 4, false},
		{"valid length 7", 7, false},
		{"valid length 12", 12, false},
		{"invalid too small", 3, true},
		{"invalid too large", 13, true},
		{"invalid negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := NewGeneratorWithLength(tt.length)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for length %d, got none", tt.length)
				}
				if gen != nil {
					t.Errorf("Expected nil generator for invalid length, got non-nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid length %d: %v", tt.length, err)
				}
				if gen == nil {
					t.Errorf("Expected non-nil generator for valid length %d", tt.length)
				}
				if gen != nil && gen.GetCodeLength() != tt.length {
					t.Errorf("Expected code length %d, got %d", tt.length, gen.GetCodeLength())
				}
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	gen := NewGenerator()

	// Test basic generation
	code, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if len(code) != defaultCodeLength {
		t.Errorf("Expected code length %d, got %d", defaultCodeLength, len(code))
	}

	// Test that generated code contains only valid Base62 characters
	if !IsValidCode(code) {
		t.Errorf("Generated code %s contains invalid characters", code)
	}
}

func TestGenerateUniqueness(t *testing.T) {
	gen := NewGenerator()

	// Generate multiple codes and check for uniqueness
	const numCodes = 1000
	codes := make(map[string]bool)

	for i := 0; i < numCodes; i++ {
		code, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed on iteration %d: %v", i, err)
		}

		if codes[code] {
			t.Errorf("Generated duplicate code: %s", code)
		}
		codes[code] = true
	}

	if len(codes) != numCodes {
		t.Errorf("Expected %d unique codes, got %d", numCodes, len(codes))
	}
}

func TestGenerateConsistentLength(t *testing.T) {
	lengths := []int{4, 6, 8, 10, 12}

	for _, length := range lengths {
		t.Run(fmt.Sprintf("length_%d", length), func(t *testing.T) {
			gen, err := NewGeneratorWithLength(length)
			if err != nil {
				t.Fatalf("Failed to create generator with length %d: %v", length, err)
			}

			// Generate multiple codes and verify consistent length
			for i := 0; i < 50; i++ {
				code, err := gen.Generate()
				if err != nil {
					t.Fatalf("Generate() failed: %v", err)
				}

				if len(code) != length {
					t.Errorf("Expected length %d, got %d for code: %s", length, len(code), code)
				}
			}
		})
	}
}

func TestGenerateBatch(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name        string
		count       int
		expectError bool
	}{
		{"valid batch 1", 1, false},
		{"valid batch 10", 10, false},
		{"valid batch 100", 100, false},
		{"invalid zero", 0, true},
		{"invalid negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codes, err := gen.GenerateBatch(tt.count)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for count %d, got none", tt.count)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for count %d: %v", tt.count, err)
				return
			}

			if len(codes) != tt.count {
				t.Errorf("Expected %d codes, got %d", tt.count, len(codes))
				return
			}

			// Check uniqueness within batch
			seen := make(map[string]bool)
			for _, code := range codes {
				if seen[code] {
					t.Errorf("Found duplicate code in batch: %s", code)
				}
				seen[code] = true

				// Check validity
				if !IsValidCode(code) {
					t.Errorf("Invalid code in batch: %s", code)
				}
			}
		})
	}
}

func TestIsValidCode(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		valid bool
	}{
		{"valid 4 chars", "abcd", true},
		{"valid 7 chars", "abcDEF1", true},
		{"valid 12 chars", "abcDEF123456", true},
		{"valid mixed case", "aAbBcC1", true},
		{"invalid too short", "abc", false},
		{"invalid too long", "abcdefghijklm", false},
		{"invalid special char", "abc-def", false},
		{"invalid underscore", "abc_def", false},
		{"invalid space", "abc def", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidCode(tt.code)
			if result != tt.valid {
				t.Errorf("IsValidCode(%s) = %v, expected %v", tt.code, result, tt.valid)
			}
		})
	}
}

func TestToBase62(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name     string
		value    int64
		expected string
	}{
		{"zero", 0, "aaaaaaa"},
		{"one", 1, "aaaaaab"},
		{"base62-1", 61, "aaaaaa9"},
		{"base62", 62, "aaaaaba"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bigVal := big.NewInt(tt.value)
			result := gen.toBase62(bigVal)

			if result != tt.expected {
				t.Errorf("toBase62(%d) = %s, expected %s", tt.value, result, tt.expected)
			}
		})
	}
}

func TestGetKeyspaceSize(t *testing.T) {
	gen := NewGenerator()
	keyspaceSize := gen.GetKeyspaceSize()

	// For 7 characters: 62^7 = 3,521,614,606,208
	expected := new(big.Int)
	expected.Exp(big.NewInt(62), big.NewInt(7), nil)

	if keyspaceSize.Cmp(expected) != 0 {
		t.Errorf("GetKeyspaceSize() = %s, expected %s", keyspaceSize.String(), expected.String())
	}
}

// Benchmark tests
func BenchmarkGenerate(b *testing.B) {
	gen := NewGenerator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.Generate()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateBatch10(b *testing.B) {
	gen := NewGenerator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := gen.GenerateBatch(10)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIsValidCode(b *testing.B) {
	testCodes := []string{
		"abcDEF1",
		"invalid-code",
		"123456a",
		"ABCDEF0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, code := range testCodes {
			IsValidCode(code)
		}
	}
}

// Test that demonstrates the entropy and distribution
func TestEntropyDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping entropy test in short mode")
	}

	gen := NewGenerator()
	const sampleSize = 10000

	// Count character frequency
	charCount := make(map[rune]int)
	totalChars := 0

	for i := 0; i < sampleSize; i++ {
		code, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate() failed: %v", err)
		}

		for _, char := range code {
			charCount[char]++
			totalChars++
		}
	}

	// Check that we're using the full character set reasonably well
	// With proper randomness, each character should appear roughly equally
	expectedPerChar := float64(totalChars) / float64(len(base62Chars))
	minExpected := expectedPerChar * 0.5 // Allow 50% deviation

	usedChars := 0
	for _, count := range charCount {
		if float64(count) >= minExpected {
			usedChars++
		}
	}

	// We should use most of the character set
	if usedChars < len(base62Chars)/2 {
		t.Errorf("Poor character distribution: only %d/%d characters used significantly",
			usedChars, len(base62Chars))
	}

	t.Logf("Character distribution: %d/%d chars used significantly", usedChars, len(base62Chars))
}
