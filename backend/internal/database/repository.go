package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	"backend/internal/models"
)

// Repository handles database operations for URLs and analytics
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository instance
func NewRepository(db *sql.DB) *Repository {
	log.Printf("[REPOSITORY] Initializing URL repository")
	return &Repository{db: db}
}

// URLRepository interface defines all URL-related database operations
type URLRepository interface {
	// Core URL operations
	CreateURL(ctx context.Context, url *models.URL) error
	GetURLByShortCode(ctx context.Context, shortCode string) (*models.URL, error)
	GetURLByID(ctx context.Context, id int64) (*models.URL, error)
	UpdateURL(ctx context.Context, url *models.URL) error
	DeactivateURL(ctx context.Context, shortCode string) error

	// Reserved codes
	IsReservedCode(ctx context.Context, code string) (bool, error)
	AddReservedCode(ctx context.Context, code, reason, description string) error

	// Analytics
	RecordClick(ctx context.Context, click *models.ClickEvent) error
	GetClickCount(ctx context.Context, urlID int64) (int64, error)
	GetLastClicked(ctx context.Context, urlID int64) (*time.Time, error)
	UpdateCounterShards(ctx context.Context, urlID int64) error
	
	// Detailed Analytics
	GetClicksByDay(ctx context.Context, urlID int64, days int) ([]models.DayStat, error)
	GetTopReferrers(ctx context.Context, urlID int64, days int, limit int) ([]models.ReferrerStat, error)
	GetBrowserStats(ctx context.Context, urlID int64, days int, limit int) ([]models.BrowserStat, error)

	// Maintenance
	CleanupExpiredURLs(ctx context.Context) (int64, error)
	GetURLsCreatedSince(ctx context.Context, since time.Time, limit int) ([]*models.URL, error)
}

// Ensure Repository implements URLRepository interface
var _ URLRepository = (*Repository)(nil)

// CreateURL inserts a new URL into the database
func (r *Repository) CreateURL(ctx context.Context, url *models.URL) error {
	log.Printf("[REPOSITORY] Creating URL: ShortCode=%s, TargetURL=%s", url.ShortCode, url.TargetURL)

	query := `
		INSERT INTO urls (short_code, target_url, is_active, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		url.ShortCode,
		url.TargetURL,
		url.IsActive,
		time.Now(),
		url.ExpiresAt,
	).Scan(&url.ID, &url.CreatedAt)

	if err != nil {
		// Check for unique constraint violation
		if isUniqueViolation(err) {
			log.Printf("[REPOSITORY] ERROR: Short code collision for %s: %v", url.ShortCode, err)
			return fmt.Errorf("short code already exists: %s", url.ShortCode)
		}
		log.Printf("[REPOSITORY] ERROR: Failed to create URL %s: %v", url.ShortCode, err)
		return fmt.Errorf("failed to create URL: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Created URL ID=%d, ShortCode=%s", url.ID, url.ShortCode)
	url.LogCreation()
	return nil
}

// GetURLByShortCode retrieves a URL by its short code
func (r *Repository) GetURLByShortCode(ctx context.Context, shortCode string) (*models.URL, error) {
	log.Printf("[REPOSITORY] Fetching URL by short code: %s", shortCode)

	query := `
		SELECT id, short_code, target_url, is_active, created_at, expires_at
		FROM urls
		WHERE short_code = $1`

	url := &models.URL{}
	err := r.db.QueryRowContext(ctx, query, shortCode).Scan(
		&url.ID,
		&url.ShortCode,
		&url.TargetURL,
		&url.IsActive,
		&url.CreatedAt,
		&url.ExpiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[REPOSITORY] URL not found for short code: %s", shortCode)
			return nil, fmt.Errorf("URL not found: %s", shortCode)
		}
		log.Printf("[REPOSITORY] ERROR: Failed to fetch URL %s: %v", shortCode, err)
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Found URL ID=%d, ShortCode=%s, Active=%v",
		url.ID, url.ShortCode, url.IsActive)
	return url, nil
}

// GetURLByID retrieves a URL by its database ID
func (r *Repository) GetURLByID(ctx context.Context, id int64) (*models.URL, error) {
	log.Printf("[REPOSITORY] Fetching URL by ID: %d", id)

	query := `
		SELECT id, short_code, target_url, is_active, created_at, expires_at
		FROM urls
		WHERE id = $1`

	url := &models.URL{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&url.ID,
		&url.ShortCode,
		&url.TargetURL,
		&url.IsActive,
		&url.CreatedAt,
		&url.ExpiresAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[REPOSITORY] URL not found for ID: %d", id)
			return nil, fmt.Errorf("URL not found: %d", id)
		}
		log.Printf("[REPOSITORY] ERROR: Failed to fetch URL ID %d: %v", id, err)
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Found URL ID=%d, ShortCode=%s", url.ID, url.ShortCode)
	return url, nil
}

// UpdateURL updates an existing URL
func (r *Repository) UpdateURL(ctx context.Context, url *models.URL) error {
	log.Printf("[REPOSITORY] Updating URL ID=%d, ShortCode=%s", url.ID, url.ShortCode)

	query := `
		UPDATE urls 
		SET target_url = $2, is_active = $3, expires_at = $4
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		url.ID,
		url.TargetURL,
		url.IsActive,
		url.ExpiresAt,
	)

	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to update URL ID %d: %v", url.ID, err)
		return fmt.Errorf("failed to update URL: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to get rows affected for URL ID %d: %v", url.ID, err)
		return fmt.Errorf("failed to verify update: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("[REPOSITORY] ERROR: No rows updated for URL ID %d (not found)", url.ID)
		return fmt.Errorf("URL not found: %d", url.ID)
	}

	log.Printf("[REPOSITORY] SUCCESS: Updated URL ID=%d", url.ID)
	return nil
}

// DeactivateURL marks a URL as inactive
func (r *Repository) DeactivateURL(ctx context.Context, shortCode string) error {
	log.Printf("[REPOSITORY] Deactivating URL: %s", shortCode)

	query := `UPDATE urls SET is_active = false WHERE short_code = $1`

	result, err := r.db.ExecContext(ctx, query, shortCode)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to deactivate URL %s: %v", shortCode, err)
		return fmt.Errorf("failed to deactivate URL: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to get rows affected for URL %s: %v", shortCode, err)
		return fmt.Errorf("failed to verify deactivation: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("[REPOSITORY] ERROR: URL not found for deactivation: %s", shortCode)
		return fmt.Errorf("URL not found: %s", shortCode)
	}

	log.Printf("[REPOSITORY] SUCCESS: Deactivated URL: %s", shortCode)
	return nil
}

// IsReservedCode checks if a code is in the reserved_codes table
func (r *Repository) IsReservedCode(ctx context.Context, code string) (bool, error) {
	log.Printf("[REPOSITORY] Checking if code is reserved: %s", code)

	query := `SELECT EXISTS(SELECT 1 FROM reserved_codes WHERE code = $1)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, code).Scan(&exists)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to check reserved code %s: %v", code, err)
		return false, fmt.Errorf("failed to check reserved code: %w", err)
	}

	if exists {
		log.Printf("[REPOSITORY] Code is reserved: %s", code)
	} else {
		log.Printf("[REPOSITORY] Code is available: %s", code)
	}

	return exists, nil
}

// AddReservedCode adds a new reserved code
func (r *Repository) AddReservedCode(ctx context.Context, code, reason, description string) error {
	log.Printf("[REPOSITORY] Adding reserved code: %s (reason: %s)", code, reason)

	query := `
		INSERT INTO reserved_codes (code, reason, description)
		VALUES ($1, $2, $3)`

	_, err := r.db.ExecContext(ctx, query, code, reason, description)
	if err != nil {
		if isUniqueViolation(err) {
			log.Printf("[REPOSITORY] ERROR: Reserved code already exists: %s", code)
			return fmt.Errorf("reserved code already exists: %s", code)
		}
		log.Printf("[REPOSITORY] ERROR: Failed to add reserved code %s: %v", code, err)
		return fmt.Errorf("failed to add reserved code: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Added reserved code: %s", code)
	return nil
}

// RecordClick inserts a click event
func (r *Repository) RecordClick(ctx context.Context, click *models.ClickEvent) error {
	log.Printf("[REPOSITORY] Recording click for URL ID=%d, IP=%s",
		click.URLID, safeString(click.IP))

	query := `
		INSERT INTO click_events (
			url_id, occurred_at, ip, ua, referrer, utm_source, utm_medium,
			utm_campaign, utm_term, utm_content, query_params
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	err := r.db.QueryRowContext(ctx, query,
		click.URLID,
		click.OccurredAt,
		click.IP,
		click.UserAgent,
		click.Referrer,
		click.UTMSource,
		click.UTMMedium,
		click.UTMCampaign,
		click.UTMTerm,
		click.UTMContent,
		click.QueryParams,
	).Scan(&click.ID)

	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to record click for URL ID %d: %v", click.URLID, err)
		return fmt.Errorf("failed to record click: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Recorded click ID=%d for URL ID=%d", click.ID, click.URLID)
	return nil
}

// GetClickCount gets total clicks for a URL
func (r *Repository) GetClickCount(ctx context.Context, urlID int64) (int64, error) {
	log.Printf("[REPOSITORY] Getting click count for URL ID=%d", urlID)

	// Try sharded counters first (faster)
	var totalClicks int64
	shardedQuery := `SELECT COALESCE(SUM(clicks), 0) FROM url_counters_live WHERE url_id = $1`

	err := r.db.QueryRowContext(ctx, shardedQuery, urlID).Scan(&totalClicks)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to get sharded click count for URL ID %d: %v", urlID, err)
		// Fallback to counting individual events
		return r.getClickCountFromEvents(ctx, urlID)
	}

	log.Printf("[REPOSITORY] SUCCESS: URL ID=%d has %d clicks (from shards)", urlID, totalClicks)
	return totalClicks, nil
}

// getClickCountFromEvents fallback method to count from events table
func (r *Repository) getClickCountFromEvents(ctx context.Context, urlID int64) (int64, error) {
	log.Printf("[REPOSITORY] Fallback: Counting clicks from events for URL ID=%d", urlID)

	query := `SELECT COUNT(*) FROM click_events WHERE url_id = $1`
	var count int64

	err := r.db.QueryRowContext(ctx, query, urlID).Scan(&count)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to count click events for URL ID %d: %v", urlID, err)
		return 0, fmt.Errorf("failed to count clicks: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: URL ID=%d has %d clicks (from events)", urlID, count)
	return count, nil
}

// GetLastClicked gets the most recent click timestamp for a URL
func (r *Repository) GetLastClicked(ctx context.Context, urlID int64) (*time.Time, error) {
	log.Printf("[REPOSITORY] Getting last clicked time for URL ID=%d", urlID)

	query := `
		SELECT occurred_at 
		FROM click_events 
		WHERE url_id = $1 
		ORDER BY occurred_at DESC 
		LIMIT 1`

	var lastClicked time.Time
	err := r.db.QueryRowContext(ctx, query, urlID).Scan(&lastClicked)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[REPOSITORY] No clicks found for URL ID=%d", urlID)
			return nil, nil // No clicks yet
		}
		log.Printf("[REPOSITORY] ERROR: Failed to get last clicked for URL ID %d: %v", urlID, err)
		return nil, fmt.Errorf("failed to get last clicked: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: URL ID=%d last clicked at %s", urlID, lastClicked.Format(time.RFC3339))
	return &lastClicked, nil
}

// UpdateCounterShards updates the sharded counters for a URL
func (r *Repository) UpdateCounterShards(ctx context.Context, urlID int64) error {
	log.Printf("[REPOSITORY] Updating counter shards for URL ID=%d", urlID)

	// Pick a random shard (0-63)
	shardID := rand.Intn(64)

	query := `
		INSERT INTO url_counters_live (url_id, shard_id, clicks, updated_at)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (url_id, shard_id)
		DO UPDATE SET clicks = url_counters_live.clicks + 1, updated_at = $3`

	_, err := r.db.ExecContext(ctx, query, urlID, shardID, time.Now())
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to update counter shards for URL ID %d: %v", urlID, err)
		return fmt.Errorf("failed to update counter shards: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Updated shard %d for URL ID=%d", shardID, urlID)
	return nil
}

// CleanupExpiredURLs marks expired URLs as inactive
func (r *Repository) CleanupExpiredURLs(ctx context.Context) (int64, error) {
	log.Printf("[REPOSITORY] Starting cleanup of expired URLs")

	query := `
		UPDATE urls 
		SET is_active = false 
		WHERE expires_at IS NOT NULL 
		AND expires_at < $1 
		AND is_active = true`

	result, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to cleanup expired URLs: %v", err)
		return 0, fmt.Errorf("failed to cleanup expired URLs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to get cleanup count: %v", err)
		return 0, fmt.Errorf("failed to get cleanup count: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Cleaned up %d expired URLs", rowsAffected)
	return rowsAffected, nil
}

// GetURLsCreatedSince gets URLs created since a given time
func (r *Repository) GetURLsCreatedSince(ctx context.Context, since time.Time, limit int) ([]*models.URL, error) {
	log.Printf("[REPOSITORY] Fetching URLs created since %s (limit: %d)", since.Format(time.RFC3339), limit)

	query := `
		SELECT id, short_code, target_url, is_active, created_at, expires_at
		FROM urls
		WHERE created_at >= $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, since, limit)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to fetch URLs since %s: %v", since.Format(time.RFC3339), err)
		return nil, fmt.Errorf("failed to fetch URLs: %w", err)
	}
	defer rows.Close()

	var urls []*models.URL
	for rows.Next() {
		url := &models.URL{}
		err := rows.Scan(
			&url.ID,
			&url.ShortCode,
			&url.TargetURL,
			&url.IsActive,
			&url.CreatedAt,
			&url.ExpiresAt,
		)
		if err != nil {
			log.Printf("[REPOSITORY] ERROR: Failed to scan URL row: %v", err)
			return nil, fmt.Errorf("failed to scan URL: %w", err)
		}
		urls = append(urls, url)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[REPOSITORY] ERROR: Row iteration error: %v", err)
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	log.Printf("[REPOSITORY] SUCCESS: Found %d URLs created since %s", len(urls), since.Format(time.RFC3339))
	return urls, nil
}

// Helper functions

// isUniqueViolation checks if an error is a unique constraint violation
func isUniqueViolation(err error) bool {
	// PostgreSQL unique violation error code: 23505
	// This is a simplified check - in production you might want to use
	// a database driver specific method
	return err != nil && (fmt.Sprintf("%v", err) == "pq: duplicate key value violates unique constraint \"urls_short_code_uniq\"" ||
		fmt.Sprintf("%v", err) == "ERROR: duplicate key value violates unique constraint \"urls_short_code_uniq\" (SQLSTATE 23505)" ||
		// Generic check for constraint violations
		fmt.Sprintf("%v", err) == "UNIQUE constraint failed")
}

// safeString safely logs a string pointer (handles nil)
func safeString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	// Truncate long strings for cleaner logs
	if len(*s) > 50 {
		return (*s)[:47] + "..."
	}
	return *s
}

// GetClicksByDay returns click statistics grouped by day
func (r *Repository) GetClicksByDay(ctx context.Context, urlID int64, days int) ([]models.DayStat, error) {
	log.Printf("[REPOSITORY] Getting clicks by day for URL ID %d (last %d days)", urlID, days)
	
	query := `
		SELECT DATE(occurred_at) as click_date, COUNT(*) as clicks
		FROM click_events 
		WHERE url_id = $1 
		AND occurred_at >= NOW() - $2 * INTERVAL '1 day'
		ORDER BY click_date DESC
	`
	
	rows, err := r.db.QueryContext(ctx, query, urlID, days)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to query clicks by day: %v", err)
		return nil, fmt.Errorf("failed to get clicks by day: %w", err)
	}
	defer rows.Close()
	
	var stats []models.DayStat
	for rows.Next() {
		var stat models.DayStat
		err := rows.Scan(&stat.Date, &stat.Clicks)
		if err != nil {
			log.Printf("[REPOSITORY] ERROR: Failed to scan day stat: %v", err)
			continue
		}
		stats = append(stats, stat)
	}
	
	log.Printf("[REPOSITORY] SUCCESS: Retrieved %d day stats", len(stats))
	return stats, nil
}

// GetTopReferrers returns top referrer statistics
func (r *Repository) GetTopReferrers(ctx context.Context, urlID int64, days int, limit int) ([]models.ReferrerStat, error) {
	log.Printf("[REPOSITORY] Getting top referrers for URL ID %d (last %d days, limit %d)", urlID, days, limit)
	
	query := `
		SELECT COALESCE(referrer, 'Direct') as referrer, COUNT(*) as clicks
		FROM click_events 
		WHERE url_id = $1 
		AND occurred_at >= NOW() - $2 * INTERVAL '1 day'
		GROUP BY referrer
		ORDER BY clicks DESC
		LIMIT $3
	`
	
	rows, err := r.db.QueryContext(ctx, query, urlID, days, limit)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to query top referrers: %v", err)
		return nil, fmt.Errorf("failed to get top referrers: %w", err)
	}
	defer rows.Close()
	
	var stats []models.ReferrerStat
	for rows.Next() {
		var stat models.ReferrerStat
		err := rows.Scan(&stat.Referrer, &stat.Clicks)
		if err != nil {
			log.Printf("[REPOSITORY] ERROR: Failed to scan referrer stat: %v", err)
			continue
		}
		stats = append(stats, stat)
	}
	
	log.Printf("[REPOSITORY] SUCCESS: Retrieved %d referrer stats", len(stats))
	return stats, nil
}

// GetBrowserStats returns browser statistics based on user agent parsing
func (r *Repository) GetBrowserStats(ctx context.Context, urlID int64, days int, limit int) ([]models.BrowserStat, error) {
	log.Printf("[REPOSITORY] Getting browser stats for URL ID %d (last %d days, limit %d)", urlID, days, limit)
	
	query := `
		SELECT 
			CASE 
				WHEN ua ILIKE '%%chrome%%' THEN 'Chrome'
				WHEN ua ILIKE '%%firefox%%' THEN 'Firefox'  
				WHEN ua ILIKE '%%safari%%' AND ua NOT ILIKE '%%chrome%%' THEN 'Safari'
				WHEN ua ILIKE '%%edge%%' THEN 'Edge'
				WHEN ua ILIKE '%%opera%%' THEN 'Opera'
				WHEN ua ILIKE '%%postman%%' THEN 'Postman'
				ELSE 'Other'
			END as browser,
			COUNT(*) as clicks
		FROM click_events 
		WHERE url_id = $1 
		AND occurred_at >= NOW() - $2 * INTERVAL '1 day'
		AND ua IS NOT NULL
		GROUP BY browser
		ORDER BY clicks DESC
		LIMIT $3
	`
	
	rows, err := r.db.QueryContext(ctx, query, urlID, days, limit)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Failed to query browser stats: %v", err)
		return nil, fmt.Errorf("failed to get browser stats: %w", err)
	}
	defer rows.Close()
	
	var stats []models.BrowserStat
	for rows.Next() {
		var stat models.BrowserStat
		err := rows.Scan(&stat.Browser, &stat.Clicks)
		if err != nil {
			log.Printf("[REPOSITORY] ERROR: Failed to scan browser stat: %v", err)
			continue
		}
		stats = append(stats, stat)
	}
	
	log.Printf("[REPOSITORY] SUCCESS: Retrieved %d browser stats", len(stats))
	return stats, nil
}

// Health check specific to repository
func (r *Repository) Health(ctx context.Context) error {
	log.Printf("[REPOSITORY] Performing health check")

	// Simple query to verify database connectivity
	query := `SELECT 1`
	var result int

	err := r.db.QueryRowContext(ctx, query).Scan(&result)
	if err != nil {
		log.Printf("[REPOSITORY] ERROR: Health check failed: %v", err)
		return fmt.Errorf("repository health check failed: %w", err)
	}

	log.Printf("[REPOSITORY] Health check passed")
	return nil
}
