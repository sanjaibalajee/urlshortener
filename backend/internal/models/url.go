package models

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// URL represents a shortened URL in the system
type URL struct {
	ID        int64      `json:"-" db:"id"` // Don't expose ID in JSON
	ShortCode string     `json:"short_code" db:"short_code"`
	TargetURL string     `json:"target_url" db:"target_url"`
	IsActive  bool       `json:"is_active" db:"is_active"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// CreateURLRequest represents the request to create a new short URL
type CreateURLRequest struct {
	URL        string     `json:"url" validate:"required,url"`
	CustomCode string     `json:"custom_code,omitempty" validate:"omitempty,custom_code"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// CreateURLResponse represents the response after creating a short URL
type CreateURLResponse struct {
	ShortCode string     `json:"short_code"`
	ShortURL  string     `json:"short_url"`
	TargetURL string     `json:"target_url"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// URLInfoResponse represents the response for URL metadata
type URLInfoResponse struct {
	ShortCode   string     `json:"short_code"`
	TargetURL   string     `json:"target_url"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ClickCount  int64      `json:"click_count"`
	LastClicked *time.Time `json:"last_clicked,omitempty"`
}

// ClickEvent represents a click tracking event
type ClickEvent struct {
	ID          int64     `json:"id" db:"id"`
	URLID       int64     `json:"url_id" db:"url_id"`
	OccurredAt  time.Time `json:"occurred_at" db:"occurred_at"`
	IP          *string   `json:"ip,omitempty" db:"ip"`
	UserAgent   *string   `json:"user_agent,omitempty" db:"ua"`
	Referrer    *string   `json:"referrer,omitempty" db:"referrer"`
	UTMSource   *string   `json:"utm_source,omitempty" db:"utm_source"`
	UTMMedium   *string   `json:"utm_medium,omitempty" db:"utm_medium"`
	UTMCampaign *string   `json:"utm_campaign,omitempty" db:"utm_campaign"`
	UTMTerm     *string   `json:"utm_term,omitempty" db:"utm_term"`
	UTMContent  *string   `json:"utm_content,omitempty" db:"utm_content"`
	QueryParams *string   `json:"query_params,omitempty" db:"query_params"` // JSON string
}

// Validation constants
const (
	MaxURLLength        = 2048
	MinCustomCodeLength = 2
	MaxCustomCodeLength = 50
	MaxDescLength       = 500
)

// Validation errors
var (
	ErrInvalidURL         = errors.New("invalid URL format")
	ErrURLTooLong         = errors.New("URL is too long")
	ErrInvalidCustomCode  = errors.New("invalid custom code format")
	ErrCustomCodeTooShort = errors.New("custom code is too short")
	ErrCustomCodeTooLong  = errors.New("custom code is too long")
	ErrReservedCode       = errors.New("code is reserved")
	ErrMaliciousURL       = errors.New("potentially malicious URL detected")
)

// Regular expressions for validation
var (
	// Custom code can contain: letters, numbers, hyphens, underscores
	// No spaces, no special characters except - and _
	customCodeRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// Malicious URL patterns (basic detection)
	maliciousPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(javascript|data|vbscript):`), // Script injection
		regexp.MustCompile(`(?i)\.exe($|\?|#)`),               // Executable files
		regexp.MustCompile(`(?i)\.bat($|\?|#)`),               // Batch files
		regexp.MustCompile(`(?i)\.scr($|\?|#)`),               // Screen savers
		regexp.MustCompile(`(?i)\.zip($|\?|#)`),               // Zip files (can be suspicious)
	}
)

// ValidateURL validates a target URL for shortening
func ValidateURL(targetURL string) error {
	log.Printf("[VALIDATION] Validating URL: %s", targetURL)

	if targetURL == "" {
		log.Printf("[VALIDATION] ERROR: Empty URL provided")
		return ErrInvalidURL
	}

	if len(targetURL) > MaxURLLength {
		log.Printf("[VALIDATION] ERROR: URL too long: %d chars (max %d)", len(targetURL), MaxURLLength)
		return ErrURLTooLong
	}

	// Parse URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		log.Printf("[VALIDATION] ERROR: Failed to parse URL: %v", err)
		return ErrInvalidURL
	}

	// Check scheme
	if parsedURL.Scheme == "" {
		log.Printf("[VALIDATION] ERROR: URL missing scheme")
		return ErrInvalidURL
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		log.Printf("[VALIDATION] ERROR: Invalid URL scheme: %s", parsedURL.Scheme)
		return ErrInvalidURL
	}

	// Check host
	if parsedURL.Host == "" {
		log.Printf("[VALIDATION] ERROR: URL missing host")
		return ErrInvalidURL
	}

	// Basic malicious URL detection
	if err := checkMaliciousURL(targetURL); err != nil {
		log.Printf("[VALIDATION] ERROR: Malicious URL detected: %v", err)
		return err
	}

	log.Printf("[VALIDATION] SUCCESS: URL validation passed for %s", parsedURL.Host)
	return nil
}

// ValidateCustomCode validates a custom short code
func ValidateCustomCode(code string) error {
	log.Printf("[VALIDATION] Validating custom code: %s", code)

	if code == "" {
		return nil // Empty custom code is allowed (will generate random)
	}

	if len(code) < MinCustomCodeLength {
		log.Printf("[VALIDATION] ERROR: Custom code too short: %d chars (min %d)", len(code), MinCustomCodeLength)
		return ErrCustomCodeTooShort
	}

	if len(code) > MaxCustomCodeLength {
		log.Printf("[VALIDATION] ERROR: Custom code too long: %d chars (max %d)", len(code), MaxCustomCodeLength)
		return ErrCustomCodeTooLong
	}

	if !customCodeRegex.MatchString(code) {
		log.Printf("[VALIDATION] ERROR: Custom code contains invalid characters: %s", code)
		return ErrInvalidCustomCode
	}

	// Check for reserved patterns (case-insensitive)
	lowerCode := strings.ToLower(code)
	reservedPatterns := []string{"api", "www", "admin", "root", "null", "undefined"}
	for _, reserved := range reservedPatterns {
		if lowerCode == reserved {
			log.Printf("[VALIDATION] ERROR: Code '%s' matches reserved pattern '%s'", code, reserved)
			return ErrReservedCode
		}
	}

	log.Printf("[VALIDATION] SUCCESS: Custom code validation passed for: %s", code)
	return nil
}

// checkMaliciousURL performs basic malicious URL detection
func checkMaliciousURL(targetURL string) error {
	for i, pattern := range maliciousPatterns {
		if pattern.MatchString(targetURL) {
			return fmt.Errorf("%w: matched pattern %d", ErrMaliciousURL, i+1)
		}
	}
	return nil
}

// NormalizeURL normalizes a URL for consistent storage and comparison
func NormalizeURL(rawURL string) (string, error) {
	log.Printf("[NORMALIZE] Normalizing URL: %s", rawURL)

	// Add scheme if missing
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
		log.Printf("[NORMALIZE] Added default HTTPS scheme: %s", rawURL)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		log.Printf("[NORMALIZE] ERROR: Failed to parse URL: %v", err)
		return "", err
	}

	// Normalize host to lowercase
	parsedURL.Host = strings.ToLower(parsedURL.Host)

	// Remove trailing slash from path if it's just "/"
	if parsedURL.Path == "/" {
		parsedURL.Path = ""
	}

	// Remove default ports
	if (parsedURL.Scheme == "http" && strings.HasSuffix(parsedURL.Host, ":80")) ||
		(parsedURL.Scheme == "https" && strings.HasSuffix(parsedURL.Host, ":443")) {
		parsedURL.Host = strings.Split(parsedURL.Host, ":")[0]
		log.Printf("[NORMALIZE] Removed default port from host: %s", parsedURL.Host)
	}

	normalized := parsedURL.String()
	log.Printf("[NORMALIZE] SUCCESS: Normalized URL: %s", normalized)
	return normalized, nil
}

// IsExpired checks if a URL has expired
func (u *URL) IsExpired() bool {
	if u.ExpiresAt == nil {
		return false
	}
	expired := time.Now().After(*u.ExpiresAt)
	if expired {
		log.Printf("[EXPIRY] URL %s (ID: %d) has expired at %s", u.ShortCode, u.ID, u.ExpiresAt.Format(time.RFC3339))
	}
	return expired
}

// IsAccessible checks if a URL can be accessed (active and not expired)
func (u *URL) IsAccessible() bool {
	accessible := u.IsActive && !u.IsExpired()
	if !accessible {
		log.Printf("[ACCESS] URL %s (ID: %d) is not accessible - Active: %v, Expired: %v",
			u.ShortCode, u.ID, u.IsActive, u.IsExpired())
	}
	return accessible
}

// ToResponse converts URL model to API response format
func (u *URL) ToResponse(baseURL string) *CreateURLResponse {
	shortURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(baseURL, "/"), u.ShortCode)
	return &CreateURLResponse{
		ShortCode: u.ShortCode,
		ShortURL:  shortURL,
		TargetURL: u.TargetURL,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
		ExpiresAt: u.ExpiresAt,
	}
}

// ToInfoResponse converts URL model to info response format
func (u *URL) ToInfoResponse(clickCount int64, lastClicked *time.Time) *URLInfoResponse {
	return &URLInfoResponse{
		ShortCode:   u.ShortCode,
		TargetURL:   u.TargetURL,
		IsActive:    u.IsActive,
		CreatedAt:   u.CreatedAt,
		ExpiresAt:   u.ExpiresAt,
		ClickCount:  clickCount,
		LastClicked: lastClicked,
	}
}

// LogCreation logs the creation of a new URL
func (u *URL) LogCreation() {
	expiryInfo := "never"
	if u.ExpiresAt != nil {
		expiryInfo = u.ExpiresAt.Format(time.RFC3339)
	}

	log.Printf("[URL_CREATED] ID: %d, ShortCode: %s, TargetURL: %s, Expires: %s",
		u.ID, u.ShortCode, u.TargetURL, expiryInfo)
}

// LogAccess logs access to a URL
func (u *URL) LogAccess(ip string, userAgent string) {
	log.Printf("[URL_ACCESSED] ID: %d, ShortCode: %s, IP: %s, UA: %s",
		u.ID, u.ShortCode, ip, userAgent)
}

// Analytics statistics types
type DayStat struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

type ReferrerStat struct {
	Referrer string `json:"referrer"`
	Clicks   int64  `json:"clicks"`
}

type CountryStat struct {
	Country string `json:"country"`
	Clicks  int64  `json:"clicks"`
}

type BrowserStat struct {
	Browser string `json:"browser"`
	Clicks  int64  `json:"clicks"`
}
