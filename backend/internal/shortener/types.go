package shortener

import (
	"errors"
	"net/http"
	"time"
	
	"backend/internal/models"
)

// Config holds configuration for the shortener service
type Config struct {
	MaxRetries          int           `json:"max_retries"`
	BaseURL             string        `json:"base_url"`
	DefaultCodeLength   int           `json:"default_code_length"`
	MaxCustomCodeLength int           `json:"max_custom_code_length"`
	CollisionThreshold  int           `json:"collision_threshold"`
	ClickTimeout        time.Duration `json:"click_timeout"`
	EnableAnalytics     bool          `json:"enable_analytics"`
	AnonymizeIPs        bool          `json:"anonymize_ips"`
	RespectDNT          bool          `json:"respect_dnt"`
}

// Request types
type CreateURLRequest struct {
	URL        string     `json:"url" validate:"required"`
	CustomCode string     `json:"custom_code,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	UserID     *int64     `json:"user_id,omitempty"` // For future multi-tenant support
}

type UpdateURLRequest struct {
	TargetURL string     `json:"target_url,omitempty"`
	IsActive  *bool      `json:"is_active,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Context types
type ClickContext struct {
	IP          string            `json:"ip"`
	UserAgent   string            `json:"user_agent"`
	Referrer    string            `json:"referrer"`
	UTMParams   map[string]string `json:"utm_params"`
	QueryParams map[string]string `json:"query_params"`
	DNTHeader   bool              `json:"dnt_header"`
	Request     *http.Request     `json:"-"` // Original request for advanced parsing
}

// Response types
type AnalyticsResponse struct {
	ShortCode      string                  `json:"short_code"`
	TargetURL      string                  `json:"target_url"`
	TotalClicks    int64                   `json:"total_clicks"`
	UniqueClicks   int64                   `json:"unique_clicks"` // Estimated
	LastClicked    *time.Time              `json:"last_clicked"`
	CreatedAt      time.Time               `json:"created_at"`
	ClicksByDay    []models.DayStat        `json:"clicks_by_day"`
	TopReferrers   []models.ReferrerStat   `json:"top_referrers"`
	TopCountries   []models.CountryStat    `json:"top_countries"`
	BrowserStats   []models.BrowserStat    `json:"browser_stats"`
	PeriodStart    time.Time               `json:"period_start"`
	PeriodEnd      time.Time               `json:"period_end"`
}


// Service errors
var (
	ErrURLNotFound      = errors.New("URL not found")
	ErrURLExpired       = errors.New("URL has expired")
	ErrURLInactive      = errors.New("URL is inactive")
	ErrTooManyRetries   = errors.New("too many collision retries")
	ErrCustomCodeTaken  = errors.New("custom code already taken")
	ErrInvalidRequest   = errors.New("invalid request")
)