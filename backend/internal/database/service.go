package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"backend/internal/models"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"
)

// DBConfig holds database connection pool configuration
type DBConfig struct {
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time"`
}

// DefaultDBConfig returns default database configuration
func DefaultDBConfig() *DBConfig {
	return &DBConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 30 * time.Minute,
	}
}

// LoadDBConfigFromEnv loads database configuration from environment variables
func LoadDBConfigFromEnv() *DBConfig {
	config := DefaultDBConfig()
	
	// Load max open connections
	if maxOpenStr := os.Getenv("DB_MAX_OPEN_CONNS"); maxOpenStr != "" {
		if maxOpen, err := strconv.Atoi(maxOpenStr); err == nil && maxOpen > 0 {
			config.MaxOpenConns = maxOpen
			log.Printf("[DATABASE] Using custom MaxOpenConns: %d", maxOpen)
		}
	}
	
	// Load max idle connections
	if maxIdleStr := os.Getenv("DB_MAX_IDLE_CONNS"); maxIdleStr != "" {
		if maxIdle, err := strconv.Atoi(maxIdleStr); err == nil && maxIdle > 0 {
			config.MaxIdleConns = maxIdle
			log.Printf("[DATABASE] Using custom MaxIdleConns: %d", maxIdle)
		}
	}
	
	// Load connection max lifetime
	if lifetimeStr := os.Getenv("DB_CONN_MAX_LIFETIME"); lifetimeStr != "" {
		if lifetime, err := time.ParseDuration(lifetimeStr); err == nil {
			config.ConnMaxLifetime = lifetime
			log.Printf("[DATABASE] Using custom ConnMaxLifetime: %s", lifetime)
		}
	}
	
	// Load connection max idle time
	if idleTimeStr := os.Getenv("DB_CONN_MAX_IDLE_TIME"); idleTimeStr != "" {
		if idleTime, err := time.ParseDuration(idleTimeStr); err == nil {
			config.ConnMaxIdleTime = idleTime
			log.Printf("[DATABASE] Using custom ConnMaxIdleTime: %s", idleTime)
		}
	}
	
	return config
}

// Service represents the main database service that combines connection management and repository access
type Service interface {
	// Connection management
	Health() map[string]string
	TestConnection() map[string]interface{}
	Close() error
	
	// Repository access - exposes all URL repository methods
	URLRepository
}

// service implements the Service interface
type service struct {
	db         *sql.DB
	repository *Repository
}

// Global instance for singleton pattern
var (
	database   = os.Getenv("BLUEPRINT_DB_DATABASE")
	password   = os.Getenv("BLUEPRINT_DB_PASSWORD")
	username   = os.Getenv("BLUEPRINT_DB_USERNAME")
	port       = os.Getenv("BLUEPRINT_DB_PORT")
	host       = os.Getenv("BLUEPRINT_DB_HOST")
	schema     = os.Getenv("BLUEPRINT_DB_SCHEMA")
	dbInstance *service
)

// New creates a new database service with repository access using default configuration
func New() Service {
	return NewWithConfig(LoadDBConfigFromEnv())
}

// NewWithConfig creates a new database service with custom configuration
func NewWithConfig(config *DBConfig) Service {
	log.Printf("[DATABASE] Initializing database service with config: MaxOpen=%d, MaxIdle=%d", 
		config.MaxOpenConns, config.MaxIdleConns)
	
	// Reuse existing connection if available
	if dbInstance != nil {
		log.Printf("[DATABASE] Reusing existing database connection")
		return dbInstance
	}
	
	// Validate port environment variable
	if port == "" {
		log.Fatalf("[DATABASE] FATAL: BLUEPRINT_DB_PORT environment variable is required")
	}
	
	// Build connection string
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=%s", 
		username, password, host, port, database, schema)
	
	log.Printf("[DATABASE] Connecting to database: %s@%s:%s/%s", username, host, port, database)
	
	// Open database connection
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		log.Fatalf("[DATABASE] FATAL: Failed to open database connection: %v", err)
	}
	
	// Configure connection pool with provided configuration
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	
	log.Printf("[DATABASE] Connection pool configured - MaxOpen: %d, MaxIdle: %d, MaxLifetime: %s, MaxIdleTime: %s",
		config.MaxOpenConns, config.MaxIdleConns, config.ConnMaxLifetime, config.ConnMaxIdleTime)
	
	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("[DATABASE] FATAL: Failed to ping database: %v", err)
	}
	
	// Create repository
	repository := NewRepository(db)
	
	// Create service instance
	dbInstance = &service{
		db:         db,
		repository: repository,
	}
	
	log.Printf("[DATABASE] Successfully initialized database service")
	return dbInstance
}

// Health checks the health of the database connection
func (s *service) Health() map[string]string {
	log.Printf("[DATABASE] Performing health check")
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	stats := make(map[string]string)
	
	// Ping the database
	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Printf("[DATABASE] ERROR: Health check failed: %v", err)
		return stats
	}
	
	// Database is up, add more statistics
	stats["status"] = "up"
	stats["message"] = "Database is healthy"
	
	// Get database stats
	dbStats := s.db.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)
	
	// Evaluate stats to provide health warnings
	if dbStats.OpenConnections > 20 {
		stats["warning"] = "High number of open connections"
		log.Printf("[DATABASE] WARNING: High connection count: %d", dbStats.OpenConnections)
	}
	
	if dbStats.WaitCount > 1000 {
		stats["warning"] = "High number of connection waits"
		log.Printf("[DATABASE] WARNING: High wait count: %d", dbStats.WaitCount)
	}
	
	log.Printf("[DATABASE] Health check passed - Status: %s, Connections: %d", 
		stats["status"], dbStats.OpenConnections)
	
	return stats
}

// TestConnection tests database connectivity by running operations on actual tables
func (s *service) TestConnection() map[string]interface{} {
	log.Printf("[DATABASE] Running comprehensive database test")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result := make(map[string]interface{})
	result["test_started"] = time.Now().Format(time.RFC3339)
	
	// Test 1: Basic connectivity
	err := s.db.PingContext(ctx)
	if err != nil {
		result["ping_error"] = err.Error()
		result["overall_status"] = "failed"
		log.Printf("[DATABASE] ERROR: Ping test failed: %v", err)
		return result
	}
	result["ping_success"] = true
	
	// Test 2: Check if main tables exist and are accessible
	tables := []string{"urls", "reserved_codes", "click_events", "url_counters_live"}
	tablesAccessible := 0
	
	for _, table := range tables {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		err := s.db.QueryRowContext(ctx, query).Scan(&count)
		
		if err != nil {
			result[table+"_error"] = err.Error()
			log.Printf("[DATABASE] ERROR: Table %s not accessible: %v", table, err)
		} else {
			result[table+"_count"] = count
			result[table+"_accessible"] = true
			tablesAccessible++
			log.Printf("[DATABASE] SUCCESS: Table %s accessible with %d records", table, count)
		}
	}
	
	result["tables_accessible"] = tablesAccessible
	result["total_tables"] = len(tables)
	
	// Test 3: Test write operations with transaction
	testShortCode := fmt.Sprintf("test_%d", time.Now().Unix())
	
	// Begin transaction for atomic test
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		result["transaction_error"] = err.Error()
		log.Printf("[DATABASE] ERROR: Failed to begin transaction: %v", err)
	} else {
		// Try to insert a test record
		_, err = tx.ExecContext(ctx,
			"INSERT INTO urls (short_code, target_url, is_active) VALUES ($1, $2, $3)",
			testShortCode, "https://example.com/test", true)
		
		if err != nil {
			result["write_test_error"] = err.Error()
			result["write_test_success"] = false
			log.Printf("[DATABASE] ERROR: Write test failed: %v", err)
			tx.Rollback()
		} else {
			result["write_test_success"] = true
			log.Printf("[DATABASE] SUCCESS: Write test passed")
			
			// Rollback to clean up test data
			err = tx.Rollback()
			if err != nil {
				result["rollback_warning"] = fmt.Sprintf("Failed to rollback test transaction: %v", err)
				log.Printf("[DATABASE] WARNING: Rollback failed: %v", err)
			} else {
				result["rollback_success"] = true
			}
		}
	}
	
	// Test 4: Repository health check
	err = s.repository.Health(ctx)
	if err != nil {
		result["repository_error"] = err.Error()
		log.Printf("[DATABASE] ERROR: Repository health check failed: %v", err)
	} else {
		result["repository_healthy"] = true
		log.Printf("[DATABASE] SUCCESS: Repository health check passed")
	}
	
	// Overall status
	if tablesAccessible == len(tables) && result["write_test_success"] == true && result["repository_healthy"] == true {
		result["overall_status"] = "healthy"
		log.Printf("[DATABASE] SUCCESS: All database tests passed")
	} else {
		result["overall_status"] = "degraded"
		log.Printf("[DATABASE] WARNING: Some database tests failed")
	}
	
	result["test_completed"] = time.Now().Format(time.RFC3339)
	return result
}

// Close closes the database connection
func (s *service) Close() error {
	log.Printf("[DATABASE] Closing database connection to: %s", database)
	
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			log.Printf("[DATABASE] ERROR: Failed to close database connection: %v", err)
			return err
		}
	}
	
	// Reset singleton instance
	dbInstance = nil
	log.Printf("[DATABASE] Successfully closed database connection")
	return nil
}

// Repository method delegation - makes Service implement URLRepository interface
// This allows Service to be used anywhere URLRepository is expected

func (s *service) CreateURL(ctx context.Context, url *models.URL) error {
	return s.repository.CreateURL(ctx, url)
}

func (s *service) GetURLByShortCode(ctx context.Context, shortCode string) (*models.URL, error) {
	return s.repository.GetURLByShortCode(ctx, shortCode)
}

func (s *service) GetURLByID(ctx context.Context, id int64) (*models.URL, error) {
	return s.repository.GetURLByID(ctx, id)
}

func (s *service) UpdateURL(ctx context.Context, url *models.URL) error {
	return s.repository.UpdateURL(ctx, url)
}

func (s *service) DeactivateURL(ctx context.Context, shortCode string) error {
	return s.repository.DeactivateURL(ctx, shortCode)
}

func (s *service) IsReservedCode(ctx context.Context, code string) (bool, error) {
	return s.repository.IsReservedCode(ctx, code)
}

func (s *service) AddReservedCode(ctx context.Context, code, reason, description string) error {
	return s.repository.AddReservedCode(ctx, code, reason, description)
}

func (s *service) RecordClick(ctx context.Context, click *models.ClickEvent) error {
	return s.repository.RecordClick(ctx, click)
}

func (s *service) GetClickCount(ctx context.Context, urlID int64) (int64, error) {
	return s.repository.GetClickCount(ctx, urlID)
}

func (s *service) GetLastClicked(ctx context.Context, urlID int64) (*time.Time, error) {
	return s.repository.GetLastClicked(ctx, urlID)
}

func (s *service) UpdateCounterShards(ctx context.Context, urlID int64) error {
	return s.repository.UpdateCounterShards(ctx, urlID)
}

func (s *service) CleanupExpiredURLs(ctx context.Context) (int64, error) {
	return s.repository.CleanupExpiredURLs(ctx)
}

func (s *service) GetURLsCreatedSince(ctx context.Context, since time.Time, limit int) ([]*models.URL, error) {
	return s.repository.GetURLsCreatedSince(ctx, since, limit)
}

// New analytics method delegations
func (s *service) GetClicksByDay(ctx context.Context, urlID int64, days int) ([]models.DayStat, error) {
	return s.repository.GetClicksByDay(ctx, urlID, days)
}

func (s *service) GetTopReferrers(ctx context.Context, urlID int64, days int, limit int) ([]models.ReferrerStat, error) {
	return s.repository.GetTopReferrers(ctx, urlID, days, limit)
}

func (s *service) GetBrowserStats(ctx context.Context, urlID int64, days int, limit int) ([]models.BrowserStat, error) {
	return s.repository.GetBrowserStats(ctx, urlID, days, limit)
}

// GetDB returns the underlying database connection (for advanced use cases)
func (s *service) GetDB() *sql.DB {
	return s.db
}

// GetRepository returns the repository instance (for direct access if needed)
func (s *service) GetRepository() URLRepository {
	return s.repository
}