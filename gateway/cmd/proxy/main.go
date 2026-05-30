package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Ejoyment/Aegis-AI/gateway/internal/proxy"
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

	// TODO: Week 3 — Initialize Redis rate limiter
	// TODO: Week 4 — Initialize PostgreSQL audit logger
	// TODO: Week 6 — Initialize SLM redactor client

	// Create the proxy with nil placeholders for optional components
	p, err := proxy.NewProxy(cfg, nil, nil, nil)
	if err != nil {
		log.Fatalf("Failed to create proxy: %v", err)
	}

	// Register the proxy handler
	http.Handle("/", p)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("AegisAI Gateway listening on %s", addr)
	log.Printf("Forwarding LLM requests to: %s", cfg.UpstreamURL)
	log.Printf("Ready to intercept agents via X-Aegis-Agent-ID header")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
