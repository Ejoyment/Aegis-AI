package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ejoyment/Aegis-AI/gateway/internal/audit"
	"github.com/Ejoyment/Aegis-AI/gateway/internal/proxy"
	"github.com/Ejoyment/Aegis-AI/gateway/internal/ratelimit"
	"github.com/Ejoyment/Aegis-AI/gateway/internal/redact"
)

func main() {
	log.Println("AegisAI Gateway starting...")

	// Load configuration from environment variables
	cfg, err := proxy.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded:")
	log.Printf("  Port: %d", cfg.Port)
	log.Printf("  Upstream: %s", cfg.UpstreamURL)
	log.Printf("  Max Tokens/Min: %d", cfg.MaxTokensPerMinute)
	log.Printf("  Max Cost/Day: $%.2f", float64(cfg.MaxCostPerDay)/100.0)

	// Initialize Redis rate limiter
	rateLimiter, err := ratelimit.NewRedisRateLimiter(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to initialize rate limiter: %v", err)
	}
	defer rateLimiter.Close()

	// Initialize PostgreSQL audit logger
	auditLogger, err := audit.NewPostgresLogger(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize audit logger: %v", err)
	}
	defer auditLogger.Close()

	// Initialize SLM PII redactor client
	slmRedactor := redact.NewSLMRedactorClient(cfg.SLMRedactorURL)
	if cfg.SLMRedactorURL != "" {
		if err := slmRedactor.HealthCheck(); err != nil {
			log.Printf("Warning: SLM redactor not ready: %v", err)
		} else {
			log.Printf("Connected to SLM redactor at %s", cfg.SLMRedactorURL)
		}
	}

	// Create the proxy with all integrated components
	p, err := proxy.NewProxy(cfg, rateLimiter, auditLogger, slmRedactor)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	// Register the proxy handler
	http.Handle("/", p)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down gracefully...")
		rateLimiter.Close()
		auditLogger.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("AegisAI Gateway listening on %s", addr)
	log.Printf("Forwarding LLM requests to: %s", cfg.UpstreamURL)
	log.Printf("Rate limiting: %s", cfg.RedisURL)
	log.Printf("Audit logging: %s", cfg.DatabaseURL)
	log.Printf("Ready to intercept agents via X-Aegis-Agent-ID header")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
