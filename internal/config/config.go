// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the application
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Redis     RedisConfig     `yaml:"redis"`
	RateLimit RateLimitConfig `yaml:"rateLimit"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Proxy     ProxyConfig     `yaml:"proxy"`
}

type ServerConfig struct {
	ListenAddr     string        `yaml:"listenAddr"`
	ReadTimeout    time.Duration `yaml:"readTimeout"`
	WriteTimeout   time.Duration `yaml:"writeTimeout"`
	MaxHeaderBytes int           `yaml:"maxHeaderBytes"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	// Redis sentinel support
	UseSentinel   bool     `yaml:"useSentinel"`
	MasterName    string   `yaml:"masterName"`
	SentinelAddrs []string `yaml:"sentinelAddrs"`
}

type RateLimitConfig struct {
	RequestsPerMinute int           `yaml:"requestsPerMinute"`
	BurstSize         int           `yaml:"burstSize"`
	BlockDuration     time.Duration `yaml:"blockDuration"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type ProxyConfig struct {
	TargetURL         string   `yaml:"targetURL"`
	TrustedProxies    []string `yaml:"trustedProxies"`
	AllowedDomains    []string `yaml:"allowedDomains"`
	BlockedCountries  []string `yaml:"blockedCountries"`
	EnableGeoBlocking bool     `yaml:"enableGeoBlocking"`
}

// Load reads the configuration from a YAML file and environment variables
func Load(configPath string) (*Config, error) {
	config := &Config{}

	// Read the config file
	if err := readConfigFile(configPath, config); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Override with environment variables
	if err := loadEnvOverrides(config); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Validate the configuration
	if err := validate(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// readConfigFile reads and parses the YAML configuration file
func readConfigFile(configPath string, config *Config) error {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	err = yaml.Unmarshal(file, config)
	if err != nil {
		return fmt.Errorf("error parsing config file: %w", err)
	}

	return nil
}

// loadEnvOverrides checks for environment variables that override config values
func loadEnvOverrides(config *Config) error {
	// Server configuration
	if addr := os.Getenv("SHIELDER_LISTEN_ADDR"); addr != "" {
		config.Server.ListenAddr = addr
	}

	// Redis configuration
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		config.Redis.Addr = addr
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.Redis.Password = password
	}

	// Rate limit configuration
	if rpm := os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE"); rpm != "" {
		var requestsPerMinute int
		if _, err := fmt.Sscanf(rpm, "%d", &requestsPerMinute); err == nil {
			config.RateLimit.RequestsPerMinute = requestsPerMinute
		}
	}

	// Proxy configuration
	if targetURL := os.Getenv("PROXY_TARGET_URL"); targetURL != "" {
		config.Proxy.TargetURL = targetURL
	}

	return nil
}

// validate checks if the configuration is valid
func validate(config *Config) error {
	if config.Server.ListenAddr == "" {
		return fmt.Errorf("server listen address is required")
	}

	if config.Proxy.TargetURL == "" {
		return fmt.Errorf("proxy target URL is required")
	}

	if config.RateLimit.RequestsPerMinute <= 0 {
		return fmt.Errorf("rate limit requests per minute must be positive")
	}

	if config.RateLimit.BlockDuration <= 0 {
		return fmt.Errorf("rate limit block duration must be positive")
	}

	return nil
}

// ToRedisOptions converts RedisConfig to redis.Options
func (rc *RedisConfig) ToRedisOptions() *redis.Options {
	return &redis.Options{
		Addr:     rc.Addr,
		Password: rc.Password,
		DB:       rc.DB,
	}
}

// ToRedisSentinelOptions converts RedisConfig to redis.FailoverOptions if using sentinel
func (rc *RedisConfig) ToRedisSentinelOptions() *redis.FailoverOptions {
	if !rc.UseSentinel {
		return nil
	}

	return &redis.FailoverOptions{
		MasterName:    rc.MasterName,
		SentinelAddrs: rc.SentinelAddrs,
		Password:      rc.Password,
		DB:            rc.DB,
	}
}
