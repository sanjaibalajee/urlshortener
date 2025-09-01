package models

import (
	"strings"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		errorType   error
	}{
		{"valid http URL", "http://example.com", false, nil},
		{"valid https URL", "https://example.com", false, nil},
		{"valid URL with path", "https://example.com/path", false, nil},
		{"valid URL with query", "https://example.com?q=test", false, nil},
		{"empty URL", "", true, ErrInvalidURL},
		{"no scheme", "example.com", true, ErrInvalidURL},
		{"invalid scheme", "ftp://example.com", true, ErrInvalidURL},
		{"no host", "https://", true, ErrInvalidURL},
		{"javascript scheme", "javascript:alert('xss')", true, ErrInvalidURL},
		{"data scheme", "data:text/html,<script>alert('xss')</script>", true, ErrInvalidURL},
		{"exe file", "https://example.com/malware.exe", true, ErrMaliciousURL},
		{"batch file", "https://example.com/script.bat", true, ErrMaliciousURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for URL: %s", tt.url)
					return
				}
				if tt.errorType != nil && err != tt.errorType {
					// For wrapped errors, check if the base error matches
					if !containsError(err, tt.errorType) {
						t.Errorf("Expected error containing %v, got %v", tt.errorType, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid URL %s: %v", tt.url, err)
				}
			}
		})
	}
}

func TestValidateCustomCode(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		expectError bool
		errorType   error
	}{
		{"empty code (allowed)", "", false, nil},
		{"valid short code", "abc", false, nil},
		{"valid with numbers", "abc123", false, nil},
		{"valid with hyphen", "my-link", false, nil},
		{"valid with underscore", "my_link", false, nil},
		{"too short", "a", true, ErrCustomCodeTooShort},
		{"too long", "this-is-a-very-long-custom-code-that-definitely-exceeds-the-fifty-character-limit", true, ErrCustomCodeTooLong},
		{"invalid space", "my link", true, ErrInvalidCustomCode},
		{"invalid special char", "my@link", true, ErrInvalidCustomCode},
		{"invalid dot", "my.link", true, ErrInvalidCustomCode},
		{"reserved api", "api", true, ErrReservedCode},
		{"reserved API (case insensitive)", "API", true, ErrReservedCode},
		{"reserved www", "www", true, ErrReservedCode},
		{"reserved admin", "admin", true, ErrReservedCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCustomCode(tt.code)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for code: %s", tt.code)
					return
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("Expected error %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for valid code %s: %v", tt.code, err)
				}
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			"add https scheme",
			"example.com",
			"https://example.com",
			false,
		},
		{
			"preserve http scheme",
			"http://example.com",
			"http://example.com",
			false,
		},
		{
			"preserve https scheme",
			"https://example.com",
			"https://example.com",
			false,
		},
		{
			"lowercase host",
			"https://EXAMPLE.COM",
			"https://example.com",
			false,
		},
		{
			"remove trailing slash",
			"https://example.com/",
			"https://example.com",
			false,
		},
		{
			"preserve path",
			"https://example.com/path",
			"https://example.com/path",
			false,
		},
		{
			"remove default http port",
			"http://example.com:80",
			"http://example.com",
			false,
		},
		{
			"remove default https port",
			"https://example.com:443",
			"https://example.com",
			false,
		},
		{
			"preserve non-default port",
			"https://example.com:8080",
			"https://example.com:8080",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeURL(tt.input)
			
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input: %s", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for input %s: %v", tt.input, err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("NormalizeURL(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestURL_IsExpired(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-time.Hour)
	futureTime := now.Add(time.Hour)
	
	tests := []struct {
		name      string
		url       *URL
		expired   bool
	}{
		{
			"no expiry",
			&URL{ExpiresAt: nil},
			false,
		},
		{
			"expired",
			&URL{ExpiresAt: &pastTime},
			true,
		},
		{
			"not expired",
			&URL{ExpiresAt: &futureTime},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.url.IsExpired()
			if result != tt.expired {
				t.Errorf("IsExpired() = %v, expected %v", result, tt.expired)
			}
		})
	}
}

func TestURL_IsAccessible(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-time.Hour)
	futureTime := now.Add(time.Hour)
	
	tests := []struct {
		name       string
		url        *URL
		accessible bool
	}{
		{
			"active and not expired",
			&URL{IsActive: true, ExpiresAt: nil},
			true,
		},
		{
			"active but expired",
			&URL{IsActive: true, ExpiresAt: &pastTime},
			false,
		},
		{
			"inactive but not expired",
			&URL{IsActive: false, ExpiresAt: &futureTime},
			false,
		},
		{
			"inactive and expired",
			&URL{IsActive: false, ExpiresAt: &pastTime},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.url.IsAccessible()
			if result != tt.accessible {
				t.Errorf("IsAccessible() = %v, expected %v", result, tt.accessible)
			}
		})
	}
}

func TestURL_ToResponse(t *testing.T) {
	now := time.Now()
	url := &URL{
		ID:        123,
		ShortCode: "abc123",
		TargetURL: "https://example.com",
		IsActive:  true,
		CreatedAt: now,
		ExpiresAt: nil,
	}
	
	baseURL := "https://short.ly"
	response := url.ToResponse(baseURL)
	
	expectedShortURL := "https://short.ly/abc123"
	if response.ShortURL != expectedShortURL {
		t.Errorf("ToResponse() ShortURL = %s, expected %s", response.ShortURL, expectedShortURL)
	}
	
	// Note: ID is no longer exposed in API responses for security reasons
	
	if response.ShortCode != url.ShortCode {
		t.Errorf("ToResponse() ShortCode = %s, expected %s", response.ShortCode, url.ShortCode)
	}
}

func TestURL_ToInfoResponse(t *testing.T) {
	now := time.Now()
	lastClicked := now.Add(-time.Hour)
	url := &URL{
		ID:        123,
		ShortCode: "abc123",
		TargetURL: "https://example.com",
		IsActive:  true,
		CreatedAt: now,
		ExpiresAt: nil,
	}
	
	clickCount := int64(42)
	response := url.ToInfoResponse(clickCount, &lastClicked)
	
	if response.ClickCount != clickCount {
		t.Errorf("ToInfoResponse() ClickCount = %d, expected %d", response.ClickCount, clickCount)
	}
	
	if response.LastClicked == nil || !response.LastClicked.Equal(lastClicked) {
		t.Errorf("ToInfoResponse() LastClicked = %v, expected %v", response.LastClicked, lastClicked)
	}
	
	// Note: ID is no longer exposed in API responses for security reasons
}

// Helper function to check if an error contains another error
func containsError(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	// Check if errors are the same
	if err == target {
		return true
	}
	// Check if error messages match
	if err.Error() == target.Error() {
		return true
	}
	// Check if error message contains target error message
	return strings.Contains(err.Error(), target.Error())
}

// Benchmark tests
func BenchmarkValidateURL(b *testing.B) {
	testURL := "https://example.com/path?query=value"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateURL(testURL)
	}
}

func BenchmarkValidateCustomCode(b *testing.B) {
	testCode := "my-custom-code"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateCustomCode(testCode)
	}
}

func BenchmarkNormalizeURL(b *testing.B) {
	testURL := "EXAMPLE.COM/PATH/"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizeURL(testURL)
	}
}