package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"backend/internal/database"
	"backend/internal/shortener"
)

type Server struct {
	port int

	db               database.Service
	shortenerSvc     shortener.Service
	shortenerHandler *shortener.Handler
}

// App wraps the HTTP server and provides lifecycle management
type App struct {
	HTTPServer   *http.Server
	shortenerSvc shortener.Service
	db           database.Service
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown(ctx context.Context) error {
	log.Println("[APP] Starting graceful shutdown...")

	// Shutdown HTTP server first
	if err := a.HTTPServer.Shutdown(ctx); err != nil {
		log.Printf("[APP] HTTP server shutdown error: %v", err)
	}

	// Shutdown shortener service (drains pending clicks)
	if err := a.shortenerSvc.Shutdown(ctx); err != nil {
		log.Printf("[APP] Shortener service shutdown error: %v", err)
	}

	// Close database connection
	if err := a.db.Close(); err != nil {
		log.Printf("[APP] Database close error: %v", err)
	}

	log.Println("[APP] Graceful shutdown complete")
	return nil
}

// ListenAndServe starts the HTTP server
func (a *App) ListenAndServe() error {
	return a.HTTPServer.ListenAndServe()
}

func NewServer() *App {
	// Parse port with proper error handling
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080" // Default port
	}
	
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("[SERVER] WARNING: Invalid PORT value '%s', using default 8080: %v", portStr, err)
		port = 8080
	}

	// Initialize database service
	db := database.New()

	// Initialize shortener service with configuration
	config := &shortener.Config{
		MaxRetries:          5,
		BaseURL:             fmt.Sprintf("http://localhost:%d", port),
		DefaultCodeLength:   7,
		MaxCustomCodeLength: 50,
		CollisionThreshold:  3,
		ClickTimeout:        5 * time.Second,
		EnableAnalytics:     true,
		AnonymizeIPs:        true,
		RespectDNT:          false,
	}

	shortenerSvc := shortener.NewService(db, config)
	shortenerHandler := shortener.NewHandler(shortenerSvc)

	NewServer := &Server{
		port:             port,
		db:               db,
		shortenerSvc:     shortenerSvc,
		shortenerHandler: shortenerHandler,
	}

	// Declare Server config
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return &App{
		HTTPServer:   httpServer,
		shortenerSvc: shortenerSvc,
		db:           db,
	}
}
