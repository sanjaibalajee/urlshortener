package shortener

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"backend/internal/database"
	"backend/internal/models"
)

// Service defines the interface for URL shortening operations
type Service interface {
	// Core shortening operations
	CreateShortURL(ctx context.Context, req *CreateURLRequest) (*models.URL, error)

	// Access and redirect operations
	GetURLForRedirect(ctx context.Context, shortCode string, clickCtx *ClickContext) (*models.URL, error)

	// Management operations
	GetURLInfo(ctx context.Context, shortCode string) (*models.URLInfoResponse, error)
	UpdateURL(ctx context.Context, shortCode string, req *UpdateURLRequest) (*models.URL, error)
	DeactivateURL(ctx context.Context, shortCode string) error

	// Analytics operations
	RecordClick(ctx context.Context, shortCode string, clickCtx *ClickContext) error
	GetAnalytics(ctx context.Context, shortCode string, days int) (*AnalyticsResponse, error)

	// Utility operations
	ValidateCustomCode(ctx context.Context, code string) error
	GetRecentURLs(ctx context.Context, limit int) ([]*models.URL, error)
}

// service implements the Service interface
type service struct {
	repo      database.URLRepository
	generator *Generator
	config    *Config
}

// NewService creates a new shortener service
func NewService(repo database.URLRepository, config *Config) Service {
	log.Printf("[SHORTENER] Initializing shortener service")

	if config == nil {
		config = DefaultConfig()
		log.Printf("[SHORTENER] Using default configuration")
	}

	generator, err := NewGeneratorWithLength(config.DefaultCodeLength)
	if err != nil {
		log.Fatalf("[SHORTENER] FATAL: Failed to create generator: %v", err)
	}

	log.Printf("[SHORTENER] Service initialized - BaseURL: %s, CodeLength: %d, MaxRetries: %d",
		config.BaseURL, config.DefaultCodeLength, config.MaxRetries)

	return &service{
		repo:      repo,
		generator: generator,
		config:    config,
	}
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxRetries:          5,
		BaseURL:             "http://localhost:8080",
		DefaultCodeLength:   7,
		MaxCustomCodeLength: 50,
		CollisionThreshold:  3,
		ClickTimeout:        5 * time.Second,
		EnableAnalytics:     true,
		AnonymizeIPs:        true,
		RespectDNT:          true,
	}
}

// CreateShortURL creates a new short URL with collision handling
func (s *service) CreateShortURL(ctx context.Context, req *CreateURLRequest) (*models.URL, error) {
	log.Printf("[SHORTENER] Creating short URL for: %s", req.URL)

	// Validate and normalize the target URL
	if err := models.ValidateURL(req.URL); err != nil {
		log.Printf("[SHORTENER] ERROR: URL validation failed: %v", err)
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	normalizedURL, err := models.NormalizeURL(req.URL)
	if err != nil {
		log.Printf("[SHORTENER] ERROR: URL normalization failed: %v", err)
		return nil, fmt.Errorf("failed to normalize URL: %w", err)
	}

	// Handle custom code if provided
	var shortCode string
	if req.CustomCode != "" {
		shortCode, err = s.handleCustomCode(ctx, req.CustomCode)
		if err != nil {
			return nil, err
		}
	} else {
		shortCode, err = s.generateUniqueCode(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Create URL model
	url := &models.URL{
		ShortCode: shortCode,
		TargetURL: normalizedURL,
		IsActive:  true,
		ExpiresAt: req.ExpiresAt,
	}

	// Save to database
	if err := s.repo.CreateURL(ctx, url); err != nil {
		log.Printf("[SHORTENER] ERROR: Failed to create URL in database: %v", err)
		return nil, fmt.Errorf("failed to create URL: %w", err)
	}

	log.Printf("[SHORTENER] SUCCESS: Created short URL ID=%d, ShortCode=%s", url.ID, url.ShortCode)
	return url, nil
}

// GetURLForRedirect retrieves URL for redirection and records click
func (s *service) GetURLForRedirect(ctx context.Context, shortCode string, clickCtx *ClickContext) (*models.URL, error) {
	log.Printf("[SHORTENER] Getting URL for redirect: %s", shortCode)

	// Get URL from database
	url, err := s.repo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		log.Printf("[SHORTENER] ERROR: URL not found: %s", shortCode)
		return nil, ErrURLNotFound
	}

	// Check if URL is accessible
	if !url.IsAccessible() {
		if url.IsExpired() {
			log.Printf("[SHORTENER] ERROR: URL expired: %s", shortCode)
			return nil, ErrURLExpired
		}
		log.Printf("[SHORTENER] ERROR: URL inactive: %s", shortCode)
		return nil, ErrURLInactive
	}

	// Record click asynchronously (don't block redirect)
	if s.config.EnableAnalytics && clickCtx != nil {
		go func() {
			asyncCtx := context.Background() // Use background context for async operation
			if err := s.recordClickAsync(asyncCtx, url, clickCtx); err != nil {
				log.Printf("[SHORTENER] WARNING: Failed to record click: %v", err)
			}
		}()
	}

	log.Printf("[SHORTENER] SUCCESS: URL found for redirect - ID=%d, Target=%s", url.ID, url.TargetURL)
	return url, nil
}

// GetURLInfo retrieves URL information with analytics
func (s *service) GetURLInfo(ctx context.Context, shortCode string) (*models.URLInfoResponse, error) {
	log.Printf("[SHORTENER] Getting URL info: %s", shortCode)

	url, err := s.repo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		return nil, ErrURLNotFound
	}

	// Get analytics data
	clickCount, err := s.repo.GetClickCount(ctx, url.ID)
	if err != nil {
		log.Printf("[SHORTENER] WARNING: Failed to get click count: %v", err)
		clickCount = 0
	}

	lastClicked, err := s.repo.GetLastClicked(ctx, url.ID)
	if err != nil {
		log.Printf("[SHORTENER] WARNING: Failed to get last clicked: %v", err)
	}

	log.Printf("[SHORTENER] SUCCESS: URL info retrieved - Clicks: %d", clickCount)
	return url.ToInfoResponse(clickCount, lastClicked), nil
}

// UpdateURL updates an existing URL
func (s *service) UpdateURL(ctx context.Context, shortCode string, req *UpdateURLRequest) (*models.URL, error) {
	log.Printf("[SHORTENER] Updating URL: %s", shortCode)

	url, err := s.repo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		return nil, ErrURLNotFound
	}

	// Apply updates
	if req.TargetURL != "" {
		if err := models.ValidateURL(req.TargetURL); err != nil {
			return nil, fmt.Errorf("invalid target URL: %w", err)
		}
		normalized, err := models.NormalizeURL(req.TargetURL)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize URL: %w", err)
		}
		url.TargetURL = normalized
	}

	if req.IsActive != nil {
		url.IsActive = *req.IsActive
	}

	if req.ExpiresAt != nil {
		url.ExpiresAt = req.ExpiresAt
	}

	// Save changes
	if err := s.repo.UpdateURL(ctx, url); err != nil {
		return nil, fmt.Errorf("failed to update URL: %w", err)
	}

	log.Printf("[SHORTENER] SUCCESS: Updated URL: %s", shortCode)
	return url, nil
}

// DeactivateURL marks a URL as inactive
func (s *service) DeactivateURL(ctx context.Context, shortCode string) error {
	log.Printf("[SHORTENER] Deactivating URL: %s", shortCode)

	if err := s.repo.DeactivateURL(ctx, shortCode); err != nil {
		return fmt.Errorf("failed to deactivate URL: %w", err)
	}

	log.Printf("[SHORTENER] SUCCESS: Deactivated URL: %s", shortCode)
	return nil
}

// RecordClick manually records a click event
func (s *service) RecordClick(ctx context.Context, shortCode string, clickCtx *ClickContext) error {
	log.Printf("[SHORTENER] Recording click for: %s", shortCode)

	url, err := s.repo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		return ErrURLNotFound
	}

	return s.recordClickAsync(ctx, url, clickCtx)
}

// GetAnalytics retrieves analytics data for a URL
func (s *service) GetAnalytics(ctx context.Context, shortCode string, days int) (*AnalyticsResponse, error) {
	log.Printf("[SHORTENER] Getting analytics for: %s (last %d days)", shortCode, days)

	url, err := s.repo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		return nil, ErrURLNotFound
	}

	// Get basic click data
	clickCount, _ := s.repo.GetClickCount(ctx, url.ID)
	lastClicked, _ := s.repo.GetLastClicked(ctx, url.ID)

	// Calculate period
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	// Get detailed analytics
	clicksByDay, err := s.repo.GetClicksByDay(ctx, url.ID, days)
	if err != nil {
		log.Printf("[SHORTENER] WARNING: Failed to get clicks by day: %v", err)
		clicksByDay = []models.DayStat{} // Default to empty
	}
	
	topReferrers, err := s.repo.GetTopReferrers(ctx, url.ID, days, 10)
	if err != nil {
		log.Printf("[SHORTENER] WARNING: Failed to get top referrers: %v", err)
		topReferrers = []models.ReferrerStat{} // Default to empty
	}
	
	browserStats, err := s.repo.GetBrowserStats(ctx, url.ID, days, 10)
	if err != nil {
		log.Printf("[SHORTENER] WARNING: Failed to get browser stats: %v", err)
		browserStats = []models.BrowserStat{} // Default to empty
	}

	// Create analytics response
	analytics := &AnalyticsResponse{
		ShortCode:    shortCode,
		TargetURL:    url.TargetURL,
		TotalClicks:  clickCount,
		UniqueClicks: clickCount, // Simplified - in production, calculate unique IPs
		LastClicked:  lastClicked,
		CreatedAt:    url.CreatedAt,
		PeriodStart:  startTime,
		PeriodEnd:    endTime,
		ClicksByDay:  clicksByDay,
		TopReferrers: topReferrers,
		TopCountries: []models.CountryStat{}, // Would require GeoIP lookup
		BrowserStats: browserStats,
	}

	log.Printf("[SHORTENER] SUCCESS: Analytics retrieved - TotalClicks: %d", clickCount)
	return analytics, nil
}

// ValidateCustomCode validates a custom code for availability
func (s *service) ValidateCustomCode(ctx context.Context, code string) error {
	log.Printf("[SHORTENER] Validating custom code: %s", code)

	// Basic validation
	if err := models.ValidateCustomCode(code); err != nil {
		return err
	}

	// Check if reserved
	isReserved, err := s.repo.IsReservedCode(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to check reserved code: %w", err)
	}

	if isReserved {
		return models.ErrReservedCode
	}

	// Check if already taken
	_, err = s.repo.GetURLByShortCode(ctx, code)
	if err == nil {
		return ErrCustomCodeTaken
	}

	return nil
}

// GetRecentURLs retrieves recently created URLs
func (s *service) GetRecentURLs(ctx context.Context, limit int) ([]*models.URL, error) {
	log.Printf("[SHORTENER] Getting recent URLs (limit: %d)", limit)

	since := time.Now().AddDate(0, 0, -7) // Last 7 days
	urls, err := s.repo.GetURLsCreatedSince(ctx, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent URLs: %w", err)
	}

	log.Printf("[SHORTENER] SUCCESS: Retrieved %d recent URLs", len(urls))
	return urls, nil
}

// handleCustomCode processes custom code requests
func (s *service) handleCustomCode(ctx context.Context, customCode string) (string, error) {
	log.Printf("[SHORTENER] Processing custom code: %s", customCode)

	if err := s.ValidateCustomCode(ctx, customCode); err != nil {
		log.Printf("[SHORTENER] ERROR: Custom code validation failed: %v", err)
		return "", err
	}

	log.Printf("[SHORTENER] SUCCESS: Custom code validated: %s", customCode)
	return customCode, nil
}

// generateUniqueCode generates a unique short code with collision handling
func (s *service) generateUniqueCode(ctx context.Context) (string, error) {
	log.Printf("[SHORTENER] Generating unique code")

	var lastErr error
	collisionCount := 0

	for attempt := 0; attempt < s.config.MaxRetries; attempt++ {
		// Generate code
		code, err := s.generator.Generate()
		if err != nil {
			return "", fmt.Errorf("failed to generate code: %w", err)
		}

		// Check if code exists
		_, err = s.repo.GetURLByShortCode(ctx, code)
		if err != nil {
			// Code doesn't exist, we can use it
			if collisionCount > 0 {
				log.Printf("[SHORTENER] SUCCESS: Generated unique code after %d collisions: %s",
					collisionCount, code)
			} else {
				log.Printf("[SHORTENER] SUCCESS: Generated unique code: %s", code)
			}
			return code, nil
		}

		// Collision detected
		collisionCount++
		lastErr = fmt.Errorf("collision detected for code: %s", code)
		log.Printf("[SHORTENER] WARNING: Collision %d/%d for code: %s",
			attempt+1, s.config.MaxRetries, code)

		// If too many collisions, increase code length
		if collisionCount >= s.config.CollisionThreshold {
			log.Printf("[SHORTENER] WARNING: High collision rate, increasing code length")
			newGenerator, err := NewGeneratorWithLength(s.generator.GetCodeLength() + 1)
			if err == nil {
				s.generator = newGenerator
			}
		}
	}

	log.Printf("[SHORTENER] ERROR: Too many collisions after %d attempts", s.config.MaxRetries)
	return "", fmt.Errorf("%w: %v", ErrTooManyRetries, lastErr)
}

// recordClickAsync records a click event asynchronously
func (s *service) recordClickAsync(ctx context.Context, url *models.URL, clickCtx *ClickContext) error {
	if !s.config.EnableAnalytics || clickCtx == nil {
		return nil
	}

	// Respect Do Not Track
	if s.config.RespectDNT && clickCtx.DNTHeader {
		log.Printf("[SHORTENER] Skipping analytics due to DNT header")
		return nil
	}

	// Parse click context
	clickEvent := s.parseClickContext(url.ID, clickCtx)

	// Record in database
	if err := s.repo.RecordClick(ctx, clickEvent); err != nil {
		return fmt.Errorf("failed to record click: %w", err)
	}

	// Update sharded counters
	if err := s.repo.UpdateCounterShards(ctx, url.ID); err != nil {
		log.Printf("[SHORTENER] WARNING: Failed to update counter shards: %v", err)
		// Don't return error as main click recording succeeded
	}

	return nil
}

// parseClickContext parses HTTP request context into click event
func (s *service) parseClickContext(urlID int64, clickCtx *ClickContext) *models.ClickEvent {
	now := time.Now()

	click := &models.ClickEvent{
		URLID:      urlID,
		OccurredAt: now,
	}

	// Process IP address
	if clickCtx.IP != "" {
		ip := clickCtx.IP
		if s.config.AnonymizeIPs {
			ip = s.anonymizeIP(ip)
		}
		if ip != "" {
			click.IP = &ip
		}
	}

	// Process User Agent
	if clickCtx.UserAgent != "" {
		ua := clickCtx.UserAgent
		if len(ua) > 500 { // Truncate very long user agents
			ua = ua[:500]
		}
		click.UserAgent = &ua
	}

	// Process Referrer
	if clickCtx.Referrer != "" {
		referrer := clickCtx.Referrer
		if len(referrer) > 500 {
			referrer = referrer[:500]
		}
		click.Referrer = &referrer
	}

	// Process UTM parameters
	if len(clickCtx.UTMParams) > 0 {
		if source, ok := clickCtx.UTMParams["utm_source"]; ok {
			click.UTMSource = &source
		}
		if medium, ok := clickCtx.UTMParams["utm_medium"]; ok {
			click.UTMMedium = &medium
		}
		if campaign, ok := clickCtx.UTMParams["utm_campaign"]; ok {
			click.UTMCampaign = &campaign
		}
		if term, ok := clickCtx.UTMParams["utm_term"]; ok {
			click.UTMTerm = &term
		}
		if content, ok := clickCtx.UTMParams["utm_content"]; ok {
			click.UTMContent = &content
		}
	}

	// Process query parameters (store as JSON string)
	if len(clickCtx.QueryParams) > 0 {
		if queryJSON := s.encodeQueryParams(clickCtx.QueryParams); queryJSON != "" {
			click.QueryParams = &queryJSON
		}
	}

	return click
}

// anonymizeIP anonymizes an IP address for privacy
func (s *service) anonymizeIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	if ip.To4() != nil {
		// IPv4: Zero out last octet
		ip = ip.Mask(net.CIDRMask(24, 32))
	} else {
		// IPv6: Zero out last 80 bits
		ip = ip.Mask(net.CIDRMask(48, 128))
	}

	return ip.String()
}

// encodeQueryParams encodes query parameters as JSON string
func (s *service) encodeQueryParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	// Simple JSON-like encoding (could use json.Marshal in production)
	var parts []string
	for key, value := range params {
		if len(key) > 100 {
			key = key[:100]
		}
		if len(value) > 500 {
			value = value[:500]
		}
		parts = append(parts, fmt.Sprintf(`"%s":"%s"`, key, value))
	}

	result := "{" + strings.Join(parts, ",") + "}"
	if len(result) > 1000 { // Limit total size
		return result[:1000]
	}

	return result
}

// ParseClickContextFromRequest parses HTTP request into ClickContext
func ParseClickContextFromRequest(r *http.Request) *ClickContext {
	if r == nil {
		return nil
	}

	// Extract IP address
	ip := extractIPAddress(r)

	// Extract UTM parameters
	utmParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "utm_") && len(values) > 0 {
			utmParams[key] = values[0]
		}
	}

	// Extract all query parameters (for storage)
	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// Check DNT header
	dnt := r.Header.Get("DNT") == "1" || r.Header.Get("Sec-GPC") == "1"

	return &ClickContext{
		IP:          ip,
		UserAgent:   r.Header.Get("User-Agent"),
		Referrer:    r.Header.Get("Referer"),
		UTMParams:   utmParams,
		QueryParams: queryParams,
		DNTHeader:   dnt,
		Request:     r,
	}
}

// extractIPAddress extracts the real IP address from HTTP request
func extractIPAddress(r *http.Request) string {
	// Try X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP)
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}
