package redis

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

// Client is a Redis client wrapper
type Client struct {
	client *redis.Client
}

// NewClient creates a new Redis client
func NewClient(ctx context.Context, addr string) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		client: client,
	}, nil
}

// Close closes the Redis client
func (c *Client) Close() error {
	return c.client.Close()
}

// WaitForUpdate waits for an update URL using BLPOP
func (c *Client) WaitForUpdate(ctx context.Context, updateKey string, checksumKey string) (string, string, error) {
	log.Printf("Waiting for update on key: %s", updateKey)
	result, err := c.client.BLPop(ctx, 0, updateKey).Result()
	if err != nil {
		if err == context.Canceled {
			return "", "", err
		}
		return "", "", fmt.Errorf("failed to BLPOP from key %s: %w", updateKey, err)
	}

	if len(result) != 2 {
		return "", "", fmt.Errorf("unexpected result from BLPOP: %v", result)
	}

	url := result[1]
	if url == "" {
		return "", "", fmt.Errorf("received empty URL")
	}

	checksum := ""
	if checksumKey != "" {
		checksum, err := c.client.Get(ctx, checksumKey).Result()
		if err != nil && err != redis.Nil {
			return "", "", fmt.Errorf("failed to get checksum from key %s: %w", checksumKey, err)
		}
		if err != redis.Nil && checksum != "" {
			log.Printf("Found checksum: %s", checksum)
		}
	}

	return url, checksum, nil
}

// GetChecksum gets the checksum from Redis
func (c *Client) GetChecksum(ctx context.Context, key string) (string, error) {
	checksum, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil // Checksum key not set
		}
		return "", fmt.Errorf("failed to get checksum from Redis: %w", err)
	}
	return checksum, nil
}

// SetFailure sets the failure key in Redis
func (c *Client) SetFailure(ctx context.Context, key, message string) error {
	err := c.client.Set(ctx, key, message, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set failure key in Redis: %w", err)
	}

	return nil
}
