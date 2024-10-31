package limiter

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type Config struct {
	RequestsPerMinute int
	BurstSize         int
	BlockDuration     time.Duration
}

type RateLimiter struct {
	client *redis.Client
	config Config
	logger *logrus.Logger
}

// NewRedisClient initializes a new Redis client using the provided configuration options.
// It returns the Redis client if successful or an error if the connection cannot be established.
func NewRedisClient(cfg redis.Options) (*redis.Client, error) {
	client := redis.NewClient(&cfg)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

// NewRateLimiter initializes a new rate limiter using the provided Redis client and configuration.
// The returned rate limiter can be used to block or allow requests based on the configured rate limit.
func NewRateLimiter(client *redis.Client, config Config, logger *logrus.Logger) *RateLimiter {
	return &RateLimiter{
		client: client,
		config: config,
		logger: logger,
	}
}

// IsAllowed checks if the given IP is allowed to make a request based on the
// configured rate limit. If the IP exceeds the rate limit, it is blocked for the
// duration configured in the BlockDuration field of the Config struct.
// Returns true if the request is allowed, false if it is blocked, and an error if
// there is an issue with the Redis connection.
func (r *RateLimiter) IsAllowed(ctx context.Context, ip string) (bool, error) {
	r.logger.WithFields(logrus.Fields{
		"ip": ip,
	}).Info("Checking if IP is allowed")

	pipe := r.client.Pipeline()

	// Key for storing request count
	key := "rate:" + ip

	// Increment the counter
	incr := pipe.Incr(ctx, key)

	// Set expiration if the key is new
	pipe.Expire(ctx, key, time.Minute)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		r.logger.WithError(err).Error("Error executing Redis pipeline")
		return false, err
	}

	// Check if request count exceeds limit
	count := incr.Val()
	r.logger.WithFields(logrus.Fields{
		"ip":     ip,
		"count":  count,
		"limit":  r.config.RequestsPerMinute,
	}).Info("Request count checked")

	if count > int64(r.config.RequestsPerMinute) {
		// Block the IP
		err = r.BlockIP(ctx, ip)
		if err != nil {
			r.logger.WithError(err).Error("Error blocking IP")
		}
		return false, err
	}

	return true, nil
}

// BlockIP sets a Redis key to block the given IP address for the duration
// configured in the BlockDuration field of the Config struct. It returns an
// error if there is an issue with the Redis connection.

func (r *RateLimiter) BlockIP(ctx context.Context, ip string) error {
	r.logger.WithFields(logrus.Fields{
		"ip": ip,
	}).Info("Blocking IP")
	key := "blocked:" + ip
	err := r.client.Set(ctx, key, true, r.config.BlockDuration).Err()
	if err != nil {
		r.logger.WithError(err).Error("Error setting blocked key")
	}
	return err
}

func (r *RateLimiter) IsBlocked(ctx context.Context, ip string) (bool, error) {
	r.logger.WithFields(logrus.Fields{
		"ip": ip,
	}).Info("Checking if IP is blocked")
	key := "blocked:" + ip
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		r.logger.WithError(err).Error("Error checking blocked key")
		return false, err
	}
	return exists == 1, nil
}
