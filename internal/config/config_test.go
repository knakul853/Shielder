// internal/config/config_test.go
package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
server:
  listenAddr: ":8080"
  readTimeout: 5s
  writeTimeout: 5s
redis:
  addr: "localhost:6379"
rateLimit:
  requestsPerMinute: 100
  blockDuration: 1h
proxy:
  targetURL: "http://localhost:3000"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading configuration
	config, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if config.Server.ListenAddr != ":8080" {
		t.Errorf("Expected listen address :8080, got %s", config.Server.ListenAddr)
	}

	if config.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("Expected 100 requests per minute, got %d", config.RateLimit.RequestsPerMinute)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Create a basic config file
	configContent := `
server:
  listenAddr: ":8080"
redis:
  addr: "localhost:6379"
rateLimit:
  requestsPerMinute: 100
  blockDuration: 1h
proxy:
  targetURL: "http://localhost:3000"
`
	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Set environment variables
	os.Setenv("SHIELDER_LISTEN_ADDR", ":9090")
	os.Setenv("REDIS_ADDR", "redis:6379")
	defer func() {
		os.Unsetenv("SHIELDER_LISTEN_ADDR")
		os.Unsetenv("REDIS_ADDR")
	}()

	// Load config
	config, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variables took precedence
	if config.Server.ListenAddr != ":9090" {
		t.Errorf("Expected listen address :9090, got %s", config.Server.ListenAddr)
	}

	if config.Redis.Addr != "redis:6379" {
		t.Errorf("Expected redis address redis:6379, got %s", config.Redis.Addr)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "Valid config",
			config: Config{
				Server: ServerConfig{
					ListenAddr: ":8080",
				},
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 100,
					BlockDuration:     time.Hour,
				},
				Proxy: ProxyConfig{
					TargetURL: "http://localhost:3000",
				},
			},
			expectError: false,
		},
		{
			name: "Missing listen address",
			config: Config{
				Server: ServerConfig{},
				RateLimit: RateLimitConfig{
					RequestsPerMinute: 100,
					BlockDuration:     time.Hour,
				},
				Proxy: ProxyConfig{
					TargetURL: "http://localhost:3000",
				},
			},
			expectError: true,
		},
		{
			name: "Invalid rate limit",
			config: Config{
				Server: ServerConfig{
					ListenAddr: ":8080",
				},
				RateLimit: RateLimitConfig{
					RequestsPerMinute: -1,
					BlockDuration:     time.Hour,
				},
				Proxy: ProxyConfig{
					TargetURL: "http://localhost:3000",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error, got %v", err)
			}
		})
	}
}
