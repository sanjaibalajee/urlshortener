package shortener

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler wraps the shortener service for HTTP handling
type Handler struct {
	service Service
}

// NewHandler creates a new HTTP handler
func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

// HTTPError represents an API error response
type HTTPError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// HTTPResponse represents a successful API response
type HTTPResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[HANDLER] ERROR: Failed to encode JSON response: %v", err)
	}
}

// writeError writes JSON error response
func writeError(w http.ResponseWriter, statusCode int, err error, message string) {
	log.Printf("[HANDLER] ERROR: %s - %v", message, err)
	
	response := HTTPError{
		Error:   err.Error(),
		Message: message,
		Code:    statusCode,
	}
	
	writeJSON(w, statusCode, response)
}

// writeSuccess writes JSON success response
func writeSuccess(w http.ResponseWriter, data interface{}, message string) {
	response := HTTPResponse{
		Success: true,
		Data:    data,
		Message: message,
	}
	
	writeJSON(w, http.StatusOK, response)
}

// CreateShortURL handles POST /api/shorten
func (h *Handler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HANDLER] CreateShortURL request from %s", r.RemoteAddr)
	
	var req CreateURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err, "Invalid JSON payload")
		return
	}
	
	// Basic validation
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, "URL is required")
		return
	}
	
	url, err := h.service.CreateShortURL(r.Context(), &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		
		// Map specific errors to appropriate HTTP status codes
		switch {
		case strings.Contains(err.Error(), "invalid URL"):
			statusCode = http.StatusBadRequest
		case strings.Contains(err.Error(), "custom code"):
			statusCode = http.StatusConflict
		case strings.Contains(err.Error(), "reserved"):
			statusCode = http.StatusConflict
		}
		
		writeError(w, statusCode, err, "Failed to create short URL")
		return
	}
	
	log.Printf("[HANDLER] SUCCESS: Created short URL %s -> %s", url.ShortCode, url.TargetURL)
	writeSuccess(w, url, "Short URL created successfully")
}

// RedirectURL handles GET /{shortCode}
func (h *Handler) RedirectURL(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	log.Printf("[HANDLER] Redirect request for: %s from %s", shortCode, r.RemoteAddr)
	
	if shortCode == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, "Short code is required")
		return
	}
	
	// Parse click context from request
	clickCtx := ParseClickContextFromRequest(r)
	
	url, err := h.service.GetURLForRedirect(r.Context(), shortCode, clickCtx)
	if err != nil {
		statusCode := http.StatusNotFound
		
		switch err {
		case ErrURLExpired:
			statusCode = http.StatusGone
		case ErrURLInactive:
			statusCode = http.StatusForbidden
		}
		
		writeError(w, statusCode, err, "URL not available")
		return
	}
	
	log.Printf("[HANDLER] SUCCESS: Redirecting %s -> %s", shortCode, url.TargetURL)
	
	// Perform redirect
	http.Redirect(w, r, url.TargetURL, http.StatusFound)
}

// GetURLInfo handles GET /api/urls/{shortCode}
func (h *Handler) GetURLInfo(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	log.Printf("[HANDLER] GetURLInfo request for: %s", shortCode)
	
	if shortCode == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, "Short code is required")
		return
	}
	
	info, err := h.service.GetURLInfo(r.Context(), shortCode)
	if err != nil {
		statusCode := http.StatusNotFound
		if err == ErrURLNotFound {
			statusCode = http.StatusNotFound
		}
		
		writeError(w, statusCode, err, "URL not found")
		return
	}
	
	log.Printf("[HANDLER] SUCCESS: Retrieved info for %s", shortCode)
	writeSuccess(w, info, "URL information retrieved")
}

// UpdateURL handles PUT /api/urls/{shortCode}
func (h *Handler) UpdateURL(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	log.Printf("[HANDLER] UpdateURL request for: %s", shortCode)
	
	if shortCode == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, "Short code is required")
		return
	}
	
	var req UpdateURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err, "Invalid JSON payload")
		return
	}
	
	url, err := h.service.UpdateURL(r.Context(), shortCode, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		
		if err == ErrURLNotFound {
			statusCode = http.StatusNotFound
		} else if strings.Contains(err.Error(), "invalid") {
			statusCode = http.StatusBadRequest
		}
		
		writeError(w, statusCode, err, "Failed to update URL")
		return
	}
	
	log.Printf("[HANDLER] SUCCESS: Updated URL %s", shortCode)
	writeSuccess(w, url, "URL updated successfully")
}

// DeleteURL handles DELETE /api/urls/{shortCode}
func (h *Handler) DeleteURL(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	log.Printf("[HANDLER] DeleteURL request for: %s", shortCode)
	
	if shortCode == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, "Short code is required")
		return
	}
	
	err := h.service.DeactivateURL(r.Context(), shortCode)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == ErrURLNotFound {
			statusCode = http.StatusNotFound
		}
		
		writeError(w, statusCode, err, "Failed to delete URL")
		return
	}
	
	log.Printf("[HANDLER] SUCCESS: Deleted URL %s", shortCode)
	writeSuccess(w, nil, "URL deleted successfully")
}

// GetAnalytics handles GET /api/urls/{shortCode}/analytics
func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	log.Printf("[HANDLER] GetAnalytics request for: %s", shortCode)
	
	if shortCode == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, "Short code is required")
		return
	}
	
	// Parse days parameter (default to 30)
	days := 30
	if daysParam := r.URL.Query().Get("days"); daysParam != "" {
		if parsedDays, err := strconv.Atoi(daysParam); err == nil && parsedDays > 0 {
			days = parsedDays
		}
	}
	
	analytics, err := h.service.GetAnalytics(r.Context(), shortCode, days)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == ErrURLNotFound {
			statusCode = http.StatusNotFound
		}
		
		writeError(w, statusCode, err, "Failed to retrieve analytics")
		return
	}
	
	log.Printf("[HANDLER] SUCCESS: Retrieved analytics for %s (%d days)", shortCode, days)
	writeSuccess(w, analytics, "Analytics retrieved successfully")
}

// ValidateCustomCode handles GET /api/validate/{code}
func (h *Handler) ValidateCustomCode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	log.Printf("[HANDLER] ValidateCustomCode request for: %s", code)
	
	if code == "" {
		writeError(w, http.StatusBadRequest, ErrInvalidRequest, "Custom code is required")
		return
	}
	
	err := h.service.ValidateCustomCode(r.Context(), code)
	
	response := map[string]interface{}{
		"code":      code,
		"available": err == nil,
	}
	
	if err != nil {
		response["reason"] = err.Error()
		log.Printf("[HANDLER] Custom code %s not available: %v", code, err)
	} else {
		log.Printf("[HANDLER] Custom code %s is available", code)
	}
	
	writeSuccess(w, response, "Custom code validation completed")
}

// GetRecentURLs handles GET /api/urls
func (h *Handler) GetRecentURLs(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HANDLER] GetRecentURLs request")
	
	// Parse limit parameter (default to 10, max 100)
	limit := 10
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if parsedLimit, err := strconv.Atoi(limitParam); err == nil && parsedLimit > 0 {
			if parsedLimit > 100 {
				parsedLimit = 100
			}
			limit = parsedLimit
		}
	}
	
	urls, err := h.service.GetRecentURLs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err, "Failed to retrieve recent URLs")
		return
	}
	
	log.Printf("[HANDLER] SUCCESS: Retrieved %d recent URLs", len(urls))
	writeSuccess(w, urls, "Recent URLs retrieved successfully")
}

// HealthCheck handles GET /api/health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HANDLER] Health check request")
	
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "url-shortener",
		"version":   "1.0.0",
	}
	
	writeSuccess(w, health, "Service is healthy")
}

// RegisterRoutes registers all shortener routes with the given router
func (h *Handler) RegisterRoutes(r chi.Router) {
	log.Printf("[HANDLER] Registering shortener routes")
	
	// API routes
	r.Route("/api", func(r chi.Router) {
		// Core functionality
		r.Post("/shorten", h.CreateShortURL)
		r.Get("/health", h.HealthCheck)
		
		// URL management
		r.Route("/urls", func(r chi.Router) {
			r.Get("/", h.GetRecentURLs)
			r.Get("/{shortCode}", h.GetURLInfo)
			r.Put("/{shortCode}", h.UpdateURL)
			r.Delete("/{shortCode}", h.DeleteURL)
			r.Get("/{shortCode}/analytics", h.GetAnalytics)
		})
		
		// Validation
		r.Get("/validate/{code}", h.ValidateCustomCode)
	})
	
	// Redirect route (must be last to avoid conflicts)
	r.Get("/{shortCode}", h.RedirectURL)
	
	log.Printf("[HANDLER] Shortener routes registered successfully")
}