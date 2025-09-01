package shortener

import (
	"crypto/rand"
	"errors"
	"math/big"
)

const (
	// Base62 character set: a-z, A-Z, 0-9
	base62Chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	base62Base  = int64(len(base62Chars))

	// Default short code length (7 chars = ~41 bits entropy with base62)
	// 62^7 = 3,521,614,606,208 possible combinations
	defaultCodeLength = 7

	// Minimum and maximum allowed code lengths
	minCodeLength = 4
	maxCodeLength = 12
)

var (
	ErrInvalidCodeLength = errors.New("code length must be between 4 and 12 characters")
	ErrRandomGeneration  = errors.New("failed to generate cryptographically secure random number")
)

// Generator handles short code generation using CSPRNG and Base62 encoding
type Generator struct {
	codeLength int
}

// NewGenerator creates a new generator with default settings
func NewGenerator() *Generator {
	return &Generator{
		codeLength: defaultCodeLength,
	}
}

// NewGeneratorWithLength creates a new generator with custom code length
func NewGeneratorWithLength(length int) (*Generator, error) {
	if length < minCodeLength || length > maxCodeLength {
		return nil, ErrInvalidCodeLength
	}
	return &Generator{
		codeLength: length,
	}, nil
}

// Generate creates a cryptographically secure random short code
// Uses crypto/rand for entropy and Base62 encoding for URL-safe output
func (g *Generator) Generate() (string, error) {
	// Calculate the maximum value for our code length
	// This ensures uniform distribution across the keyspace
	maxValue := new(big.Int)
	maxValue.Exp(big.NewInt(base62Base), big.NewInt(int64(g.codeLength)), nil)

	// Generate cryptographically secure random number
	randomValue, err := rand.Int(rand.Reader, maxValue)
	if err != nil {
		return "", ErrRandomGeneration
	}

	// Convert to Base62
	return g.toBase62(randomValue), nil
}

// GenerateBatch creates multiple unique short codes in one call
// Useful for pre-generating codes or batch operations
func (g *Generator) GenerateBatch(count int) ([]string, error) {
	if count <= 0 {
		return nil, errors.New("count must be positive")
	}

	codes := make([]string, 0, count)
	seen := make(map[string]bool)

	for len(codes) < count {
		code, err := g.Generate()
		if err != nil {
			return nil, err
		}

		// Ensure uniqueness within the batch
		if !seen[code] {
			seen[code] = true
			codes = append(codes, code)
		}
	}

	return codes, nil
}

// toBase62 converts a big integer to Base62 string representation
func (g *Generator) toBase62(value *big.Int) string {
	if value.Sign() == 0 {
		// Handle zero case - pad to required length
		result := string(base62Chars[0])
		for len(result) < g.codeLength {
			result = string(base62Chars[0]) + result
		}
		return result
	}

	// Convert to base62
	result := ""
	base := big.NewInt(base62Base)
	zero := big.NewInt(0)
	remainder := &big.Int{}

	// Create a copy to avoid modifying the original
	num := new(big.Int).Set(value)

	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, remainder)
		result = string(base62Chars[remainder.Int64()]) + result
	}

	// Pad with leading characters if necessary to maintain consistent length
	for len(result) < g.codeLength {
		result = string(base62Chars[0]) + result
	}

	return result
}

// IsValidCode checks if a string is a valid Base62 code
func IsValidCode(code string) bool {
	if len(code) < minCodeLength || len(code) > maxCodeLength {
		return false
	}

	for _, char := range code {
		valid := false
		for _, validChar := range base62Chars {
			if char == validChar {
				valid = true
				break
			}
		}
		if !valid {
			return false
		}
	}
	return true
}

// GetCodeLength returns the current code length setting
func (g *Generator) GetCodeLength() int {
	return g.codeLength
}

// GetKeyspaceSize returns the total number of possible codes for current length
func (g *Generator) GetKeyspaceSize() *big.Int {
	result := new(big.Int)
	return result.Exp(big.NewInt(base62Base), big.NewInt(int64(g.codeLength)), nil)
}