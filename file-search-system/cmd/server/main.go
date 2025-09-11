package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/file-search/file-search-system/internal/api"
	"github.com/file-search/file-search-system/internal/config"
	"github.com/file-search/file-search-system/internal/database"
	"github.com/file-search/file-search-system/internal/service"
	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "", "Path to configuration file (auto-detected if not specified)")
	initDB     = flag.Bool("init-db", false, "Initialize database schema")
	daemon     = flag.Bool("daemon", false, "Run as background daemon")
)

func main() {
	flag.Parse()

	// Initialize logger
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Set log level
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Initialize database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Initialize database schema if requested
	if *initDB {
		if err := db.InitSchema(); err != nil {
			log.WithError(err).Fatal("Failed to initialize database schema")
		}
		log.Info("Database schema initialized successfully")
		return
	}

	// Context is managed by the service

	// Initialize background service
	svc, err := service.NewService(cfg, db, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize service")
	}

	// Start background service
	if err := svc.Start(); err != nil {
		log.WithError(err).Fatal("Failed to start service")
	}

	// Initialize and start API server
	apiServer := api.NewServer(cfg, db, svc, log)
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.APIHost, cfg.APIPort)
		log.WithField("address", addr).Info("Starting API server")
		if err := apiServer.Start(addr); err != nil {
			log.WithError(err).Error("API server stopped")
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")

	// Stop services
	apiServer.Stop()
	svc.Stop()

	log.Info("Shutdown complete")
}
