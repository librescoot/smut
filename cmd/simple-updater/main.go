package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/librescoot/simple-updater/pkg/config"
	"github.com/librescoot/simple-updater/pkg/download"
	"github.com/librescoot/simple-updater/pkg/mender"
	"github.com/librescoot/simple-updater/pkg/redis"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		log.Fatalf("Error parsing configuration: %v", err)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Printf("Simple Updater starting with config: %+v", cfg)

	if err := checkMenderAvailable(); err != nil {
		log.Fatalf("Error checking mender-update: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	redisClient, err := redis.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		log.Fatalf("Error creating Redis client: %v", err)
	}
	defer redisClient.Close()

	downloadManager := download.NewManager(cfg.DownloadDir)

	menderClient := mender.NewClient()

	if err := checkAndCommitUpdate(menderClient); err != nil {
		log.Printf("Error checking/committing update: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Context canceled, exiting...")
			return
		default:
			url, _, err := redisClient.WaitForUpdate(ctx, cfg.UpdateKey, cfg.ChecksumKey)
			if err != nil {
				if err == context.Canceled {
					log.Println("Context canceled, exiting...")
					return
				}
				log.Printf("Error waiting for update: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("Received update URL: %s", url)

			if err := handleUpdate(ctx, url, downloadManager, menderClient, redisClient, cfg); err != nil {
				log.Printf("Error handling update: %v", err)
				if err := redisClient.SetFailure(ctx, cfg.FailureKey, err.Error()); err != nil {
					log.Printf("Error setting failure in Redis: %v", err)
				}
			}
		}
	}
}

func checkMenderAvailable() error {
	_, err := exec.LookPath("mender-update")
	if err != nil {
		return fmt.Errorf("mender-update not found in PATH: %w", err)
	}
	return nil
}

func checkAndCommitUpdate(menderClient *mender.Client) error {
	needsCommit, err := menderClient.NeedsCommit()
	if err != nil {
		return fmt.Errorf("error checking if update needs commit: %w", err)
	}

	if needsCommit {
		log.Println("Update needs to be committed, committing...")
		if err := menderClient.Commit(); err != nil {
			return fmt.Errorf("error committing update: %w", err)
		}
		log.Println("Update committed successfully")
	} else {
		log.Println("No update needs to be committed")
	}

	return nil
}

func handleUpdate(
	ctx context.Context,
	url string,
	downloadManager *download.Manager,
	menderClient *mender.Client,
	redisClient *redis.Client,
	cfg *config.Config,
) error {
	downloadPath, err := downloadManager.Download(ctx, url)
	if err != nil {
		return fmt.Errorf("error downloading update: %w", err)
	}
	log.Printf("Downloaded update to: %s", downloadPath)

	checksum, err := redisClient.GetChecksum(ctx, cfg.ChecksumKey)
	if err != nil {
		log.Printf("Warning: Could not retrieve checksum from Redis: %v", err)
	}

	if checksum != "" {
		log.Printf("Verifying checksum: %s", checksum)
		if err := downloadManager.VerifyChecksum(downloadPath, checksum); err != nil {
			os.Remove(downloadPath)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		log.Println("Checksum verification successful")
	} else {
		log.Println("No checksum provided, skipping verification")
	}

	log.Println("Installing update...")
	if err := menderClient.Install(downloadPath); err != nil {
		os.Remove(downloadPath)
		return fmt.Errorf("error installing update: %w", err)
	}
	log.Println("Update installed successfully")

	if err := os.Remove(downloadPath); err != nil {
		log.Printf("Warning: Failed to remove downloaded file %s: %v", downloadPath, err)
	}

	return nil
}
