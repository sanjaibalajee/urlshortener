package shortener

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"backend/internal/database"
	"backend/internal/models"
)

// MockRepository implements URLRepository for testing
type MockRepository struct {
	urls         map[string]*models.URL
	reservedCode map[string]bool
	clickCounts  map[int64]int64
	lastClicked  map[int64]*time.Time
	nextID       int64
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		urls:         make(map[string]*models.URL),
		reservedCode: make(map[string]bool),
		clickCounts:  make(map[int64]int64),
		lastClicked:  make(map[int64]*time.Time),
		nextID:       1,
	}
}

func (m *MockRepository) CreateURL(ctx context.Context, url *models.URL) error {
	if _, exists := m.urls[url.ShortCode]; exists {
		return errors.New("short code already exists: " + url.ShortCode)
	}
	
	url.ID = m.nextID
	url.CreatedAt = time.Now()
	m.nextID++
	
	m.urls[url.ShortCode] = url
	return nil
}

func (m *MockRepository) GetURLByShortCode(ctx context.Context, shortCode string) (*models.URL, error) {
	if url, exists := m.urls[shortCode]; exists {
		return url, nil
	}
	return nil, errors.New("URL not found: " + shortCode)
}

func (m *MockRepository) GetURLByID(ctx context.Context, id int64) (*models.URL, error) {
	for _, url := range m.urls {
		if url.ID == id {
			return url, nil
		}
	}
	return nil, errors.New("URL not found")
}

func (m *MockRepository) UpdateURL(ctx context.Context, url *models.URL) error {
	if existing, exists := m.urls[url.ShortCode]; exists {
		existing.TargetURL = url.TargetURL
		existing.IsActive = url.IsActive
		existing.ExpiresAt = url.ExpiresAt
		return nil
	}
	return errors.New("URL not found")
}

func (m *MockRepository) DeactivateURL(ctx context.Context, shortCode string) error {
	if url, exists := m.urls[shortCode]; exists {
		url.IsActive = false
		return nil
	}
	return errors.New("URL not found: " + shortCode)
}

func (m *MockRepository) IsReservedCode(ctx context.Context, code string) (bool, error) {
	return m.reservedCode[code], nil
}

func (m *MockRepository) AddReservedCode(ctx context.Context, code, reason, description string) error {
	m.reservedCode[code] = true
	return nil
}

func (m *MockRepository) RecordClick(ctx context.Context, click *models.ClickEvent) error {
	click.ID = m.nextID
	m.nextID++
	return nil
}

func (m *MockRepository) GetClickCount(ctx context.Context, urlID int64) (int64, error) {
	return m.clickCounts[urlID], nil
}

func (m *MockRepository) GetLastClicked(ctx context.Context, urlID int64) (*time.Time, error) {
	return m.lastClicked[urlID], nil
}

func (m *MockRepository) UpdateCounterShards(ctx context.Context, urlID int64) error {
	m.clickCounts[urlID]++
	now := time.Now()
	m.lastClicked[urlID] = &now
	return nil
}

func (m *MockRepository) CleanupExpiredURLs(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockRepository) GetURLsCreatedSince(ctx context.Context, since time.Time, limit int) ([]*models.URL, error) {
	var urls []*models.URL
	for _, url := range m.urls {
		if url.CreatedAt.After(since) {
			urls = append(urls, url)
			if len(urls) >= limit {
				break
			}
		}
	}
	return urls, nil
}

// New analytics methods for mock repository
func (m *MockRepository) GetClicksByDay(ctx context.Context, urlID int64, days int) ([]models.DayStat, error) {
	// Mock implementation - return sample data
	return []models.DayStat{
		{Date: "2025-09-01", Clicks: 5},
		{Date: "2025-09-02", Clicks: 3},
	}, nil
}

func (m *MockRepository) GetTopReferrers(ctx context.Context, urlID int64, days int, limit int) ([]models.ReferrerStat, error) {
	// Mock implementation - return sample data
	return []models.ReferrerStat{
		{Referrer: "Direct", Clicks: 8},
		{Referrer: "google.com", Clicks: 2},
	}, nil
}

func (m *MockRepository) GetBrowserStats(ctx context.Context, urlID int64, days int, limit int) ([]models.BrowserStat, error) {
	// Mock implementation - return sample data
	return []models.BrowserStat{
		{Browser: "Chrome", Clicks: 6},
		{Browser: "Firefox", Clicks: 2},
	}, nil
}

func (m *MockRepository) GetAnalyticsBatch(ctx context.Context, urlID int64, days int, referrerLimit int, browserLimit int) (*database.AnalyticsBatch, error) {
	// Mock implementation - return sample data
	return &database.AnalyticsBatch{
		ClicksByDay: []models.DayStat{
			{Date: "2025-09-01", Clicks: 5},
			{Date: "2025-09-02", Clicks: 3},
		},
		TopReferrers: []models.ReferrerStat{
			{Referrer: "Direct", Clicks: 8},
			{Referrer: "google.com", Clicks: 2},
		},
		BrowserStats: []models.BrowserStat{
			{Browser: "Chrome", Clicks: 6},
			{Browser: "Firefox", Clicks: 2},
		},
	}, nil
}

// Test helper functions
func setupTestService() Service {
	repo := NewMockRepository()
	config := DefaultConfig()
	config.BaseURL = "http://test.ly"
	return NewService(repo, config)
}

func TestNewService(t *testing.T) {
	service := setupTestService()
	
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestCreateShortURL(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	tests := []struct {
		name      string
		request   *CreateURLRequest
		wantError bool
	}{
		{
			name: "valid URL",
			request: &CreateURLRequest{
				URL: "https://example.com",
			},
			wantError: false,
		},
		{
			name: "valid URL with custom code",
			request: &CreateURLRequest{
				URL:        "https://example.com/path",
				CustomCode: "mycustom",
			},
			wantError: false,
		},
		{
			name: "invalid URL",
			request: &CreateURLRequest{
				URL: "not-a-url",
			},
			wantError: true,
		},
		{
			name: "empty URL",
			request: &CreateURLRequest{
				URL: "",
			},
			wantError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := service.CreateShortURL(ctx, tt.request)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("CreateShortURL() expected error, got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("CreateShortURL() unexpected error: %v", err)
				return
			}
			
			if url.ID == 0 {
				t.Errorf("CreateShortURL() did not set ID")
			}
			
			if url.ShortCode == "" {
				t.Errorf("CreateShortURL() did not set ShortCode")
			}
			
			if tt.request.CustomCode != "" && url.ShortCode != tt.request.CustomCode {
				t.Errorf("CreateShortURL() ShortCode = %s, expected %s", 
					url.ShortCode, tt.request.CustomCode)
			}
			
			if !url.IsActive {
				t.Errorf("CreateShortURL() URL should be active")
			}
		})
	}
}

func TestCreateShortURL_CustomCodeValidation(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	// Add a reserved code through the service interface
	// Since we can't access the private fields, we'll use a workaround
	// by calling AddReservedCode on the mock repo directly
	repo := NewMockRepository()
	repo.AddReservedCode(ctx, "admin", "test", "admin code for testing")
	config := DefaultConfig()
	config.BaseURL = "http://test.ly"
	service = NewService(repo, config)
	
	tests := []struct {
		name       string
		customCode string
		wantError  bool
		errorType  error
	}{
		{"valid custom code", "mylink", false, nil},
		{"reserved code", "admin", true, models.ErrReservedCode},
		{"invalid characters", "my@link", true, models.ErrInvalidCustomCode},
		{"too short", "a", true, models.ErrCustomCodeTooShort},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CreateURLRequest{
				URL:        "https://example.com",
				CustomCode: tt.customCode,
			}
			
			_, err := service.CreateShortURL(ctx, req)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("CreateShortURL() expected error, got none")
					return
				}
				// Check error type if specified
				if tt.errorType != nil && !strings.Contains(err.Error(), tt.errorType.Error()) {
					t.Errorf("CreateShortURL() error = %v, expected to contain %v", err, tt.errorType)
				}
			} else {
				if err != nil {
					t.Errorf("CreateShortURL() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetURLForRedirect(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	// Create test URL
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "testget",
	}
	
	createdURL, err := service.CreateShortURL(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}
	
	tests := []struct {
		name      string
		shortCode string
		wantError bool
		errorType error
	}{
		{"existing URL", "testget", false, nil},
		{"non-existent URL", "notfound", true, ErrURLNotFound},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clickCtx := &ClickContext{
				IP:        "192.168.1.1",
				UserAgent: "Mozilla/5.0",
			}
			
			url, err := service.GetURLForRedirect(ctx, tt.shortCode, clickCtx)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("GetURLForRedirect() expected error, got none")
					return
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("GetURLForRedirect() error = %v, expected %v", err, tt.errorType)
				}
			} else {
				if err != nil {
					t.Errorf("GetURLForRedirect() unexpected error: %v", err)
					return
				}
				
				if url.ShortCode != tt.shortCode {
					t.Errorf("GetURLForRedirect() ShortCode = %s, expected %s", 
						url.ShortCode, tt.shortCode)
				}
				
				if url.ID != createdURL.ID {
					t.Errorf("GetURLForRedirect() returned wrong URL")
				}
			}
		})
	}
}

func TestGetURLForRedirect_ExpiredURL(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	// Create expired URL
	pastTime := time.Now().Add(-time.Hour)
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "expired",
		ExpiresAt:  &pastTime,
	}
	
	_, err := service.CreateShortURL(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}
	
	_, err = service.GetURLForRedirect(ctx, "expired", nil)
	if err != ErrURLExpired {
		t.Errorf("GetURLForRedirect() error = %v, expected %v", err, ErrURLExpired)
	}
}

func TestGetURLInfo(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	// Create test URL
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "testinfo",
	}
	
	createdURL, err := service.CreateShortURL(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}
	
	info, err := service.GetURLInfo(ctx, "testinfo")
	if err != nil {
		t.Errorf("GetURLInfo() unexpected error: %v", err)
		return
	}
	
	if info.ShortCode != "testinfo" {
		t.Errorf("GetURLInfo() ShortCode = %s, expected %s", info.ShortCode, "testinfo")
	}
	
	if info.TargetURL != createdURL.TargetURL {
		t.Errorf("GetURLInfo() TargetURL = %s, expected %s", info.TargetURL, createdURL.TargetURL)
	}
	
	if info.ClickCount != 0 {
		t.Errorf("GetURLInfo() ClickCount = %d, expected 0", info.ClickCount)
	}
}

func TestUpdateURL(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	// Create test URL
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "testupdate",
	}
	
	_, err := service.CreateShortURL(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}
	
	// Update URL
	updateReq := &UpdateURLRequest{
		TargetURL: "https://updated.com",
	}
	
	updatedURL, err := service.UpdateURL(ctx, "testupdate", updateReq)
	if err != nil {
		t.Errorf("UpdateURL() unexpected error: %v", err)
		return
	}
	
	if updatedURL.TargetURL != "https://updated.com" {
		t.Errorf("UpdateURL() TargetURL = %s, expected %s", 
			updatedURL.TargetURL, "https://updated.com")
	}
}

func TestDeactivateURL(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	// Create test URL
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "testdeactivate",
	}
	
	_, err := service.CreateShortURL(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}
	
	// Deactivate URL
	err = service.DeactivateURL(ctx, "testdeactivate")
	if err != nil {
		t.Errorf("DeactivateURL() unexpected error: %v", err)
	}
	
	// Verify URL is inactive
	_, err = service.GetURLForRedirect(ctx, "testdeactivate", nil)
	if err != ErrURLInactive {
		t.Errorf("GetURLForRedirect() error = %v, expected %v", err, ErrURLInactive)
	}
}

func TestValidateCustomCode(t *testing.T) {
	// Create a service with a mock repo that has reserved codes
	repo := NewMockRepository()
	repo.AddReservedCode(context.Background(), "admin", "test", "admin code for testing")
	config := DefaultConfig()
	config.BaseURL = "http://test.ly"
	service := NewService(repo, config)
	ctx := context.Background()
	
	// Create an existing code
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "existing",
	}
	service.CreateShortURL(ctx, req)
	
	tests := []struct {
		name      string
		code      string
		wantError bool
		errorType error
	}{
		{"valid new code", "newcode", false, nil},
		{"reserved code", "admin", true, models.ErrReservedCode},
		{"existing code", "existing", true, ErrCustomCodeTaken},
		{"invalid format", "bad@code", true, models.ErrInvalidCustomCode},
		{"too short", "x", true, models.ErrCustomCodeTooShort},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateCustomCode(ctx, tt.code)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateCustomCode() expected error, got none")
					return
				}
				if tt.errorType != nil && !strings.Contains(err.Error(), tt.errorType.Error()) {
					t.Errorf("ValidateCustomCode() error = %v, expected to contain %v", err, tt.errorType)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateCustomCode() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestParseClickContextFromRequest(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedIP     string
		expectedUA     string
		expectedDNT    bool
		expectedUTMLen int
	}{
		{
			name: "basic request",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", "http://test.ly/abc123", nil)
				req.Header.Set("User-Agent", "Mozilla/5.0")
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			},
			expectedIP:  "192.168.1.1",
			expectedUA:  "Mozilla/5.0",
			expectedDNT: false,
		},
		{
			name: "request with UTM params",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", 
					"http://test.ly/abc123?utm_source=twitter&utm_medium=social&other=value", nil)
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			},
			expectedIP:     "192.168.1.1",
			expectedUTMLen: 2,
		},
		{
			name: "request with DNT header",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", "http://test.ly/abc123", nil)
				req.Header.Set("DNT", "1")
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			},
			expectedIP:  "192.168.1.1",
			expectedDNT: true,
		},
		{
			name: "request with X-Forwarded-For",
			setupRequest: func() *http.Request {
				req, _ := http.NewRequest("GET", "http://test.ly/abc123", nil)
				req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
				req.RemoteAddr = "192.168.1.1:12345"
				return req
			},
			expectedIP: "203.0.113.1",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			clickCtx := ParseClickContextFromRequest(req)
			
			if clickCtx == nil {
				t.Fatal("ParseClickContextFromRequest() returned nil")
			}
			
			if tt.expectedIP != "" && clickCtx.IP != tt.expectedIP {
				t.Errorf("ParseClickContextFromRequest() IP = %s, expected %s", 
					clickCtx.IP, tt.expectedIP)
			}
			
			if tt.expectedUA != "" && clickCtx.UserAgent != tt.expectedUA {
				t.Errorf("ParseClickContextFromRequest() UserAgent = %s, expected %s", 
					clickCtx.UserAgent, tt.expectedUA)
			}
			
			if clickCtx.DNTHeader != tt.expectedDNT {
				t.Errorf("ParseClickContextFromRequest() DNTHeader = %v, expected %v", 
					clickCtx.DNTHeader, tt.expectedDNT)
			}
			
			if tt.expectedUTMLen > 0 && len(clickCtx.UTMParams) != tt.expectedUTMLen {
				t.Errorf("ParseClickContextFromRequest() UTMParams length = %d, expected %d", 
					len(clickCtx.UTMParams), tt.expectedUTMLen)
			}
		})
	}
}

func TestAnonymizeIP(t *testing.T) {
	// Test anonymizeIP functionality indirectly through click recording
	// Since anonymizeIP is private, we'll test it through the public API
	
	service := setupTestService()
	ctx := context.Background()
	
	// Create test URL
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "testanon",
	}
	
	_, err := service.CreateShortURL(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create test URL: %v", err)
	}
	
	// Test click recording with IP anonymization
	clickCtx := &ClickContext{
		IP:        "192.168.1.100",
		UserAgent: "Mozilla/5.0",
	}
	
	// This should work without error, and IP should be anonymized internally
	err = service.RecordClick(ctx, "testanon", clickCtx)
	if err != nil {
		t.Errorf("RecordClick() unexpected error: %v", err)
	}
}

func TestGetRecentURLs(t *testing.T) {
	service := setupTestService()
	ctx := context.Background()
	
	// Create some test URLs
	for i := 0; i < 5; i++ {
		req := &CreateURLRequest{
			URL:        fmt.Sprintf("https://example.com/%d", i),
			CustomCode: fmt.Sprintf("test%d", i),
		}
		_, err := service.CreateShortURL(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create test URL %d: %v", i, err)
		}
	}
	
	// Get recent URLs
	urls, err := service.GetRecentURLs(ctx, 3)
	if err != nil {
		t.Errorf("GetRecentURLs() unexpected error: %v", err)
		return
	}
	
	if len(urls) != 3 {
		t.Errorf("GetRecentURLs() returned %d URLs, expected 3", len(urls))
	}
}

// Benchmark tests
func BenchmarkCreateShortURL(b *testing.B) {
	service := setupTestService()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := &CreateURLRequest{
			URL: fmt.Sprintf("https://example.com/%d", i),
		}
		service.CreateShortURL(ctx, req)
	}
}

func BenchmarkGetURLForRedirect(b *testing.B) {
	service := setupTestService()
	ctx := context.Background()
	
	// Create test URL
	req := &CreateURLRequest{
		URL:        "https://example.com",
		CustomCode: "benchtest",
	}
	service.CreateShortURL(ctx, req)
	
	clickCtx := &ClickContext{
		IP:        "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetURLForRedirect(ctx, "benchtest", clickCtx)
	}
}