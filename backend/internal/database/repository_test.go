package database

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"backend/internal/models"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Note: These are integration tests that require a test database
// To run them, you need to have PostgreSQL running and set up test database
// For unit testing, you would mock the database interface

func setupTestDB() (*sql.DB, error) {
	// This would typically use environment variables for test database
	connStr := "postgres://postgres@localhost:5432/url_test?sslmode=disable"
	return sql.Open("pgx", connStr)
}

func setupTestRepository(t *testing.T) (*Repository, func()) {
	t.Helper()

	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	db, err := setupTestDB()
	if err != nil {
		t.Skip("Test database not available, skipping integration tests")
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		t.Skip("Cannot connect to test database, skipping integration tests")
	}

	repo := NewRepository(db)

	// Cleanup function
	cleanup := func() {
		// Clean up test data
		db.Exec("DELETE FROM click_events WHERE url_id IN (SELECT id FROM urls WHERE short_code LIKE 'test%')")
		db.Exec("DELETE FROM url_counters_live WHERE url_id IN (SELECT id FROM urls WHERE short_code LIKE 'test%')")
		db.Exec("DELETE FROM urls WHERE short_code LIKE 'test%'")
		db.Exec("DELETE FROM reserved_codes WHERE code LIKE 'test%'")
		db.Close()
	}

	return repo, cleanup
}

func TestRepository_CreateURL(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name    string
		url     *models.URL
		wantErr bool
	}{
		{
			name: "valid URL",
			url: &models.URL{
				ShortCode: "test123",
				TargetURL: "https://example.com",
				IsActive:  true,
			},
			wantErr: false,
		},
		{
			name: "URL with expiry",
			url: &models.URL{
				ShortCode: "test456",
				TargetURL: "https://example.com/path",
				IsActive:  true,
				ExpiresAt: func() *time.Time { t := time.Now().Add(time.Hour); return &t }(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.CreateURL(ctx, tt.url)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateURL() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("CreateURL() unexpected error: %v", err)
				return
			}

			// Verify URL was created
			if tt.url.ID == 0 {
				t.Errorf("CreateURL() did not set ID")
			}

			if tt.url.CreatedAt.IsZero() {
				t.Errorf("CreateURL() did not set CreatedAt")
			}

			// Verify we can retrieve it
			retrieved, err := repo.GetURLByShortCode(ctx, tt.url.ShortCode)
			if err != nil {
				t.Errorf("Failed to retrieve created URL: %v", err)
				return
			}

			if retrieved.ShortCode != tt.url.ShortCode {
				t.Errorf("Retrieved URL ShortCode = %s, expected %s", retrieved.ShortCode, tt.url.ShortCode)
			}
		})
	}
}

func TestRepository_CreateURL_Collision(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create first URL
	url1 := &models.URL{
		ShortCode: "testcollision",
		TargetURL: "https://example.com/first",
		IsActive:  true,
	}

	err := repo.CreateURL(ctx, url1)
	if err != nil {
		t.Fatalf("Failed to create first URL: %v", err)
	}

	// Try to create second URL with same short code
	url2 := &models.URL{
		ShortCode: "testcollision",
		TargetURL: "https://example.com/second",
		IsActive:  true,
	}

	err = repo.CreateURL(ctx, url2)
	if err == nil {
		t.Errorf("CreateURL() should have failed with collision, but succeeded")
	}

	// Verify error message contains collision info
	expectedMsg := "short code already exists"
	if err != nil && len(err.Error()) > 0 && err.Error()[:len(expectedMsg)] != expectedMsg {
		t.Errorf("Expected collision error message, got: %v", err)
	}
}

func TestRepository_GetURLByShortCode(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test URL
	testURL := &models.URL{
		ShortCode: "testget",
		TargetURL: "https://example.com/get",
		IsActive:  true,
	}

	err := repo.CreateURL(ctx, testURL)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}

	tests := []struct {
		name      string
		shortCode string
		wantErr   bool
	}{
		{"existing URL", "testget", false},
		{"non-existent URL", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := repo.GetURLByShortCode(ctx, tt.shortCode)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetURLByShortCode() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("GetURLByShortCode() unexpected error: %v", err)
				return
			}

			if url.ShortCode != tt.shortCode {
				t.Errorf("GetURLByShortCode() ShortCode = %s, expected %s", url.ShortCode, tt.shortCode)
			}
		})
	}
}

func TestRepository_UpdateURL(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test URL
	testURL := &models.URL{
		ShortCode: "testupdate",
		TargetURL: "https://example.com/original",
		IsActive:  true,
	}

	err := repo.CreateURL(ctx, testURL)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}

	// Update the URL
	testURL.TargetURL = "https://example.com/updated"
	testURL.IsActive = false

	err = repo.UpdateURL(ctx, testURL)
	if err != nil {
		t.Errorf("UpdateURL() unexpected error: %v", err)
		return
	}

	// Verify update
	retrieved, err := repo.GetURLByShortCode(ctx, testURL.ShortCode)
	if err != nil {
		t.Errorf("Failed to retrieve updated URL: %v", err)
		return
	}

	if retrieved.TargetURL != "https://example.com/updated" {
		t.Errorf("UpdateURL() TargetURL = %s, expected %s", retrieved.TargetURL, "https://example.com/updated")
	}

	if retrieved.IsActive != false {
		t.Errorf("UpdateURL() IsActive = %v, expected %v", retrieved.IsActive, false)
	}
}

func TestRepository_IsReservedCode(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Add a reserved code
	err := repo.AddReservedCode(ctx, "testreserved", "test", "Test reserved code")
	if err != nil {
		t.Fatalf("Failed to add reserved code: %v", err)
	}

	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"reserved code", "testreserved", true},
		{"non-reserved code", "notreserved", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isReserved, err := repo.IsReservedCode(ctx, tt.code)
			if err != nil {
				t.Errorf("IsReservedCode() unexpected error: %v", err)
				return
			}

			if isReserved != tt.expected {
				t.Errorf("IsReservedCode() = %v, expected %v", isReserved, tt.expected)
			}
		})
	}
}

func TestRepository_RecordClick(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test URL
	testURL := &models.URL{
		ShortCode: "testclick",
		TargetURL: "https://example.com/click",
		IsActive:  true,
	}

	err := repo.CreateURL(ctx, testURL)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}

	// Record a click
	ip := "192.168.1.1"
	ua := "Mozilla/5.0"
	click := &models.ClickEvent{
		URLID:      testURL.ID,
		OccurredAt: time.Now(),
		IP:         &ip,
		UserAgent:  &ua,
	}

	err = repo.RecordClick(ctx, click)
	if err != nil {
		t.Errorf("RecordClick() unexpected error: %v", err)
		return
	}

	if click.ID == 0 {
		t.Errorf("RecordClick() did not set click ID")
	}

	// Verify click count
	count, err := repo.GetClickCount(ctx, testURL.ID)
	if err != nil {
		t.Errorf("GetClickCount() unexpected error: %v", err)
		return
	}

	if count != 1 {
		t.Errorf("GetClickCount() = %d, expected 1", count)
	}
}

func TestRepository_GetLastClicked(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test URL
	testURL := &models.URL{
		ShortCode: "testlast",
		TargetURL: "https://example.com/last",
		IsActive:  true,
	}

	err := repo.CreateURL(ctx, testURL)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}

	// No clicks yet
	lastClicked, err := repo.GetLastClicked(ctx, testURL.ID)
	if err != nil {
		t.Errorf("GetLastClicked() unexpected error: %v", err)
		return
	}

	if lastClicked != nil {
		t.Errorf("GetLastClicked() = %v, expected nil for no clicks", lastClicked)
	}

	// Record a click
	clickTime := time.Now()
	ip := "192.168.1.1"
	click := &models.ClickEvent{
		URLID:      testURL.ID,
		OccurredAt: clickTime,
		IP:         &ip,
	}

	err = repo.RecordClick(ctx, click)
	if err != nil {
		t.Fatalf("Failed to record click: %v", err)
	}

	// Check last clicked
	lastClicked, err = repo.GetLastClicked(ctx, testURL.ID)
	if err != nil {
		t.Errorf("GetLastClicked() unexpected error: %v", err)
		return
	}

	if lastClicked == nil {
		t.Errorf("GetLastClicked() = nil, expected timestamp")
		return
	}

	// Allow small time difference due to processing
	timeDiff := lastClicked.Sub(clickTime)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("GetLastClicked() time difference too large: %v", timeDiff)
	}
}

func TestRepository_UpdateCounterShards(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create test URL
	testURL := &models.URL{
		ShortCode: "testshard",
		TargetURL: "https://example.com/shard",
		IsActive:  true,
	}

	err := repo.CreateURL(ctx, testURL)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}

	// Update counter shards multiple times
	for i := 0; i < 5; i++ {
		err = repo.UpdateCounterShards(ctx, testURL.ID)
		if err != nil {
			t.Errorf("UpdateCounterShards() iteration %d unexpected error: %v", i, err)
		}
	}

	// Check total count from shards
	count, err := repo.GetClickCount(ctx, testURL.ID)
	if err != nil {
		t.Errorf("GetClickCount() unexpected error: %v", err)
		return
	}

	if count != 5 {
		t.Errorf("GetClickCount() = %d, expected 5", count)
	}
}

func TestRepository_CleanupExpiredURLs(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	// Create expired URL
	pastTime := time.Now().Add(-time.Hour)
	expiredURL := &models.URL{
		ShortCode: "testexpired",
		TargetURL: "https://example.com/expired",
		IsActive:  true,
		ExpiresAt: &pastTime,
	}

	err := repo.CreateURL(ctx, expiredURL)
	if err != nil {
		t.Fatalf("Failed to create expired URL: %v", err)
	}

	// Create non-expired URL
	futureTime := time.Now().Add(time.Hour)
	activeURL := &models.URL{
		ShortCode: "testactive",
		TargetURL: "https://example.com/active",
		IsActive:  true,
		ExpiresAt: &futureTime,
	}

	err = repo.CreateURL(ctx, activeURL)
	if err != nil {
		t.Fatalf("Failed to create active URL: %v", err)
	}

	// Cleanup expired URLs
	cleaned, err := repo.CleanupExpiredURLs(ctx)
	if err != nil {
		t.Errorf("CleanupExpiredURLs() unexpected error: %v", err)
		return
	}

	if cleaned != 1 {
		t.Errorf("CleanupExpiredURLs() cleaned %d URLs, expected 1", cleaned)
	}

	// Verify expired URL is now inactive
	retrieved, err := repo.GetURLByShortCode(ctx, "testexpired")
	if err != nil {
		t.Errorf("Failed to retrieve expired URL: %v", err)
		return
	}

	if retrieved.IsActive {
		t.Errorf("Expired URL should be inactive, but IsActive = %v", retrieved.IsActive)
	}

	// Verify active URL is still active
	retrieved, err = repo.GetURLByShortCode(ctx, "testactive")
	if err != nil {
		t.Errorf("Failed to retrieve active URL: %v", err)
		return
	}

	if !retrieved.IsActive {
		t.Errorf("Active URL should remain active, but IsActive = %v", retrieved.IsActive)
	}
}

func TestRepository_Health(t *testing.T) {
	repo, cleanup := setupTestRepository(t)
	defer cleanup()

	ctx := context.Background()

	err := repo.Health(ctx)
	if err != nil {
		t.Errorf("Health() unexpected error: %v", err)
	}
}

// Benchmark tests
func BenchmarkRepository_CreateURL(b *testing.B) {
	repo, cleanup := setupTestRepository(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := &models.URL{
			ShortCode: fmt.Sprintf("bench%d", i),
			TargetURL: "https://example.com",
			IsActive:  true,
		}
		repo.CreateURL(ctx, url)
	}
}

func BenchmarkRepository_GetURLByShortCode(b *testing.B) {
	repo, cleanup := setupTestRepository(&testing.T{})
	defer cleanup()

	ctx := context.Background()

	// Create test URL
	testURL := &models.URL{
		ShortCode: "benchget",
		TargetURL: "https://example.com",
		IsActive:  true,
	}
	repo.CreateURL(ctx, testURL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetURLByShortCode(ctx, "benchget")
	}
}
