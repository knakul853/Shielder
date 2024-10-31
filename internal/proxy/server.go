package proxy

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/knakul853/shielder/internal/limiter"
	"github.com/knakul853/shielder/internal/monitor"
	"github.com/sirupsen/logrus"
)

type Server struct {
	server      *http.Server
	target      *url.URL
	rateLimiter *limiter.RateLimiter
	metrics     *monitor.MetricsCollector
	logger      *logrus.Logger
}

type Config struct {
	ListenAddr  string
	TargetURL   string
	ReadTimeout time.Duration
}

// NewServer initializes a new reverse proxy server that forwards requests to the target URL.
// The server uses the given rate limiter to block requests that exceed the configured rate
// limit, and the given metrics collector to collect metrics about the request traffic.
//
// The server is configured with the given listen address and read/write timeout.
//
// The target URL is parsed and validated at construction time, and the server is ready to
// be started with the Start method.
func NewServer(cfg Config, limiter *limiter.RateLimiter, metrics *monitor.MetricsCollector) *Server {
	target, err := url.Parse(cfg.TargetURL)
	if err != nil {
		log.Fatalf("Failed to parse target URL: %v", err) // Use logrus later
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.DebugLevel) // Adjust log level as needed

	proxy := &Server{
		target:      target,
		rateLimiter: limiter,
		metrics:     metrics,
		logger:      logger,
	}

	proxy.server = &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      proxy.handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.ReadTimeout,
	}

	return proxy
}

// handler returns an http.Handler that forwards requests to the target URL after
// checking that the request is allowed according to the configured rate limit.
//
// The handler logs the request and response, and records metrics about the request
// traffic, including the number of requests and the number of blocked requests.
//
// If the request is blocked due to rate limiting, the handler returns a 429 status
// code with a "Too Many Requests" message. If there is an error checking the rate
// limit, the handler returns a 500 status code with an "Internal Server Error"
// message.
func (s *Server) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr

		// Start timing the request
		start := time.Now()
		defer func() {
			s.metrics.ObserveRequestDuration(r.URL.Path, time.Since(start))
		}()

		s.logger.WithFields(logrus.Fields{
			"client_ip": clientIP,
			"method":    r.Method,
			"url":       r.URL,
		}).Info("Request received")

		// Check if IP is blocked
		blocked, err := s.rateLimiter.IsBlocked(r.Context(), clientIP)
		if err != nil {
			s.logger.WithError(err).Error("Error checking if IP is blocked")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if blocked {
			s.logger.WithField("client_ip", clientIP).Info("IP blocked")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			s.metrics.IncBlockedRequests(clientIP)
			return
		}

		// Check rate limit
		allowed, err := s.rateLimiter.IsAllowed(r.Context(), clientIP)
		if err != nil {
			s.logger.WithError(err).Error("Error checking rate limit")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if !allowed {
			s.logger.WithField("client_ip", clientIP).Info("Rate limit exceeded")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			s.metrics.IncBlockedRequests(clientIP)
			return
		}

		// Forward the request to the target
		proxy := httputil.NewSingleHostReverseProxy(s.target)
		proxy.ServeHTTP(w, r)

		s.logger.WithFields(logrus.Fields{
			"client_ip": clientIP,
			"status":    http.StatusOK,
		}).Info("Request successful")

		s.metrics.IncSuccessfulRequests(clientIP)
	})
}

func (s *Server) Start() error {
	s.logger.WithField("address", s.server.Addr).Info("Starting server")
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server")
	return s.server.Shutdown(ctx)
}
