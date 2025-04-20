package redis

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis/v8"
)

const (
	// OTAHashKey is the Redis hash key for OTA status and type
	OTAHashKey = "ota"
	// OTAStatusField is the field within the OTA hash for the overall OTA status
	OTAStatusField = "status"
	// OTAUpdateTypeField is the field within the OTA hash for the update type (blocking/non-blocking)
	OTAUpdateTypeField = "update-type"
)

// Client is a Redis client wrapper
type Client struct {
	client *redis.Client
}

// SetStatus sets the status field in the ota hash in Redis
func (c *Client) SetStatus(ctx context.Context, status string) error {
	err := c.client.HSet(ctx, OTAHashKey, OTAStatusField, status).Err()
	if err != nil {
		return fmt.Errorf("failed to set %s field in %s hash in Redis: %w", OTAStatusField, OTAHashKey, err)
	}
	log.Printf("Set %s field in %s hash to '%s'", OTAStatusField, OTAHashKey, status)
	return nil
}

// SetUpdateType sets the update-type field in the ota hash in Redis
func (c *Client) SetUpdateType(ctx context.Context, updateType string) error {
	err := c.client.HSet(ctx, OTAHashKey, OTAUpdateTypeField, updateType).Err()
	if err != nil {
		return fmt.Errorf("failed to set %s field in %s hash in Redis: %w", OTAUpdateTypeField, OTAHashKey, err)
	}
	log.Printf("Set %s field in %s hash to '%s'", OTAUpdateTypeField, OTAHashKey, updateType)
	return nil
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
