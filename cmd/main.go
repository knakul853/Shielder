package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/knakul853/shielder/internal/config"
	"github.com/knakul853/shielder/internal/limiter"
	"github.com/knakul853/shielder/internal/monitor"
	"github.com/knakul853/shielder/internal/proxy"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel)

	// Get the absolute path of the config file
	configPath, err := filepath.Abs("configs/config.yaml")
	if err != nil {
		logger.WithError(err).Fatalf("Failed to get absolute path for config file")
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.WithError(err).Fatalf("Failed to load config")
	}

	// Create context that listens for the interrupt signal from the OS
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize Redis client
	redisClient, err := limiter.NewRedisClient(*cfg.Redis.ToRedisOptions())
	if err != nil {
		logger.WithError(err).Fatalf("Failed to connect to Redis")
	}
	defer redisClient.Close()

	// Initialize rate limiter
	limiterConfig := limiter.Config{
		RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
		BurstSize:         cfg.RateLimit.BurstSize,
		BlockDuration:     cfg.RateLimit.BlockDuration,
	}
	rateLimiter := limiter.NewRateLimiter(redisClient, limiterConfig, logger)

	// Initialize metrics collector
	metrics := monitor.NewMetricsCollector()

	// Create and start the proxy server
	proxyCfg := proxy.Config{
		ListenAddr:  cfg.Server.ListenAddr,
		TargetURL:   cfg.Proxy.TargetURL,
		ReadTimeout: cfg.Server.ReadTimeout,
	}
	server := proxy.NewServer(proxyCfg, rateLimiter, metrics)

	go func() {
		if err := server.Start(); err != nil {
			logger.WithError(err).Error("Server error")
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	logger.Info("Shutting down gracefully...")

	// Shutdown the server
	if err := server.Shutdown(context.Background()); err != nil {
		logger.WithError(err).Error("Error during shutdown")
	}
}
