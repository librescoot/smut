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

	// Publish the status update
	publishErr := c.client.Publish(ctx, OTAHashKey, OTAStatusField).Err()
	if publishErr != nil {
		log.Printf("Failed to publish status update for field %s: %v", OTAStatusField, publishErr)
	} else {
		log.Printf("Published status update for field %s", OTAStatusField)
	}

	return nil
}

// SetUpdateType sets the update-type field in the ota hash in Redis
func (c *Client) SetUpdateType(ctx context.Context, updateType string) error {
	err := c.client.HSet(ctx, OTAHashKey, OTAUpdateTypeField, updateType).Err()
	if err != nil {
		return fmt.Errorf("failed to set %s field in %s hash in Redis: %w", OTAUpdateTypeField, OTAHashKey, err)
	}
	log.Printf("Set %s field in %s hash to '%s'", OTAUpdateTypeField, OTAHashKey, updateType)

	// Publish the update type update
	publishErr := c.client.Publish(ctx, OTAHashKey, OTAUpdateTypeField).Err()
	if publishErr != nil {
		log.Printf("Failed to publish update type update for field %s: %v", OTAUpdateTypeField, publishErr)
	} else {
		log.Printf("Published update type update for field %s", OTAUpdateTypeField)
	}

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

// WaitForUpdate waits for an update URL using BLPOP and keeps popping until the list is empty
func (c *Client) WaitForUpdate(ctx context.Context, updateKey string, checksumKey string) (string, string, error) {
	log.Printf("Waiting for update on key: %s", updateKey)
	
	// First BLPOP to wait for at least one entry
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

	// Get the first URL
	lastUrl := result[1]
	if lastUrl == "" {
		return "", "", fmt.Errorf("received empty URL")
	}
	
	// Keep popping until the list is empty
	for {
		// Use LPOP (non-blocking) to check if there are more entries
		result, err := c.client.LPop(ctx, updateKey).Result()
		if err != nil {
			if err == redis.Nil {
				// List is empty, we're done
				break
			}
			// Log other errors but continue with the last URL we got
			log.Printf("Warning: Error during LPOP from key %s: %v", updateKey, err)
			break
		}
		
		// If we got a non-empty URL, update our lastUrl
		if result != "" {
			log.Printf("Found additional URL in list, using: %s", result)
			lastUrl = result
		}
	}
	
	log.Printf("Using final URL from list: %s", lastUrl)

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

	return lastUrl, checksum, nil
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
