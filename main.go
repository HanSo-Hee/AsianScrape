/*
 * Asian Drama Scraper Bot
 * Author: Abhishek (MrAbhi2k3)
 * GitHub: https://github.com/mrabhi2k3
 */

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"asianscraper/config"
	"asianscraper/db"
	"asianscraper/tgbot"
)

func startWebServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // default fallback
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("AsianScraper Bot is running!"))
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	log.Printf("Web server started on port %s", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, nil); err != nil {
		log.Printf("Web server failed: %v", err)
	}
}

func main() {
	log.Println("Starting AsianScraper Migration to Go...")

	// 1. Load config
	config.Load()

	// 2. Initialize database
	db.Init(config.Global.MongoSRV)
	defer db.Global.Close()

	// 3. Start web server (Render health check)
	go startWebServer()

	// 4. Start Telegram bot
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("Shutdown signal received, shutting down gracefully...")
		cancel()
	}()

	tgbot.StartBot(ctx)
}
