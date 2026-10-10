package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-models-pulse/internal/aggregator"
	"ai-models-pulse/internal/handlers"
	"ai-models-pulse/internal/models"
)

//go:embed web/static/*
var embeddedStatic embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("========================================================\n")
	log.Printf(" 🚀 AI Models Pulse - Real-time AI Observatory\n")
	log.Printf("========================================================\n")

	// 1. Initialize aggregator and live models manager
	agg := aggregator.NewAggregator()
	liveModelManager := models.NewLiveModelManager()
	
	// Initial live internet model sync & drop watcher
	go liveModelManager.SyncFromInternet()
	liveModelManager.StartLiveDropWatcher()

	// 2. Start background feed sync
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initial sync and schedule refresh every 15 minutes
	agg.StartAutoRefresh(ctx, 15*time.Minute)

	// Periodic model sync every 30 minutes
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				liveModelManager.SyncFromInternet()
			}
		}
	}()

	// 3. Initialize API handlers
	api := handlers.NewAPIHandler(agg, liveModelManager)

	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/api/feed", api.GetFeed)
	mux.HandleFunc("/api/models", api.GetModels)
	mux.HandleFunc("/api/models/live-stream", api.StreamLiveDrops)
	mux.HandleFunc("/api/models/latest-drops", api.GetLatestDrops)
	mux.HandleFunc("/api/stats", api.GetStats)
	mux.HandleFunc("/api/sources", api.GetSources)
	mux.HandleFunc("/api/refresh", api.RefreshFeed)

	// Static UI file server
	staticContent, err := fs.Sub(embeddedStatic, "web/static")
	if err != nil {
		log.Fatalf("Failed to sub embedded static FS: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticContent))
	mux.Handle("/", fileServer)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      loggingMiddleware(corsMiddleware(mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 4. Start HTTP Server
	go func() {
		log.Printf("🌐 Server listening on http://localhost:%s\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// 5. Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\nShutting down server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v\n", err)
	}
	log.Println("Server stopped.")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/favicon.ico" {
			log.Printf("%s %s in %v\n", r.Method, r.URL.Path, time.Since(start))
		}
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
