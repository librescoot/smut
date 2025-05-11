package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/librescoot/smut/pkg/config"
	"github.com/librescoot/smut/pkg/download"
	"github.com/librescoot/smut/pkg/mender"
	"github.com/librescoot/smut/pkg/redis"
)

var Version string

func main() {
	cfg, err := config.Parse()
	if err != nil {
		log.Fatalf("Error parsing configuration: %v", err)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	// Version is set at build time using ldflags
	if Version == "" {
		Version = "dev"
	}
	log.Printf("Simple Mender Update Tool %s starting with config: %+v", Version, cfg)

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

	// Set the update key and component in the Redis client
	redisClient.SetUpdateKey(cfg.UpdateKey)
	redisClient.SetComponent(cfg.Component)

	// Set initial status and update type
	if err := redisClient.SetStatus(ctx, "initializing"); err != nil {
		log.Printf("Error setting initial status in Redis: %v", err)
	}
	if err := redisClient.SetUpdateType(ctx, cfg.UpdateType); err != nil {
		log.Printf("Error setting initial update type in Redis: %v", err)
	}

	downloadManager := download.NewManager(cfg.DownloadDir)

	menderClient := mender.NewClient()

	if err := checkAndCommitUpdate(menderClient); err != nil {
		log.Printf("Error checking/committing update: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Context canceled, exiting...")
			// Set status to unknown on exit
			if err := redisClient.SetStatus(context.Background(), "unknown"); err != nil {
				log.Printf("Error setting final status in Redis: %v", err)
			}
			if err := redisClient.SetUpdateType(context.Background(), "none"); err != nil {
				log.Printf("Error setting final update type in Redis: %v", err)
			}
			return
		default:
			// Set status to checking-updates before waiting
			if err := redisClient.SetStatus(ctx, "checking-updates"); err != nil {
				log.Printf("Error setting status to checking-updates in Redis: %v", err)
			}

			url, _, err := redisClient.WaitForUpdate(ctx, cfg.UpdateKey, cfg.ChecksumKey)
			if err != nil {
				if err == context.Canceled {
					log.Println("Context canceled, exiting...")
					// Set status to unknown on exit
					if err := redisClient.SetStatus(context.Background(), "unknown"); err != nil {
						log.Printf("Error setting final status in Redis: %v", err)
					}
					if err := redisClient.SetUpdateType(context.Background(), "none"); err != nil {
						log.Printf("Error setting final update type in Redis: %v", err)
					}
					return
				}
				log.Printf("Error waiting for update: %v", err)
				// Set status to checking-update-error on error
				if err := redisClient.SetStatus(ctx, "checking-update-error"); err != nil {
					log.Printf("Error setting status to checking-update-error in Redis: %v", err)
				}
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("Received update URL: %s", url)

			if err := handleUpdate(ctx, url, downloadManager, menderClient, redisClient, cfg); err != nil {
				log.Printf("Error handling update: %v", err)
				// Set status to appropriate error state based on handleUpdate error
				status := "unknown" // Default to unknown
				if strings.Contains(err.Error(), "download") {
					status = "downloading-update-error"
				} else if strings.Contains(err.Error(), "install") {
					status = "installing-update-error"
				}
				if err := redisClient.SetStatus(ctx, status); err != nil {
					log.Printf("Error setting error status in Redis: %v", err)
				}

				if err := redisClient.SetFailure(ctx, cfg.FailureKey, err.Error()); err != nil {
					log.Printf("Error setting failure in Redis: %v", err)
				}
			} else {
				// Set status to installation-complete-waiting-reboot on success
				if err := redisClient.SetStatus(ctx, "installation-complete-waiting-reboot"); err != nil {
					log.Printf("Error setting status to installation-complete-waiting-reboot in Redis: %v", err)
				}
				// Set update type to none on success
				if err := redisClient.SetUpdateType(ctx, "none"); err != nil {
					log.Printf("Error setting update type to none in Redis: %v", err)
				}
				
				// Wait for reboot instead of continuing to check for updates
				log.Println("Update installed successfully. Waiting for reboot...")
				select {
				case <-ctx.Done():
					log.Println("Context canceled, exiting...")
					return
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
	var downloadPath string
	var err error
	
	// Check if this is a file:// URL
	if strings.HasPrefix(url, "file://") {
		// For file:// URLs, extract the path and skip downloading
		filePath := strings.TrimPrefix(url, "file://")
		log.Printf("Using local file: %s", filePath)
		downloadPath = filePath
	} else {
		// Set status to downloading-updates for non-file URLs
		if err := redisClient.SetStatus(ctx, "downloading-updates"); err != nil {
			log.Printf("Error setting status to downloading-updates in Redis: %v", err)
		}

		downloadPath, err = downloadManager.Download(ctx, url)
		if err != nil {
			// Set status to downloading-update-error on download error
			if err := redisClient.SetStatus(ctx, "downloading-update-error"); err != nil {
				log.Printf("Error setting status to downloading-update-error in Redis: %v", err)
			}
			return fmt.Errorf("error downloading update: %w", err)
		}
		log.Printf("Downloaded update to: %s", downloadPath)
	}

	checksum, err := redisClient.GetChecksum(ctx, cfg.ChecksumKey)
	if err != nil {
		log.Printf("Warning: Could not retrieve checksum from Redis: %v", err)
	}

	if checksum != "" {
		log.Printf("Verifying checksum: %s", checksum)
		if err := downloadManager.VerifyChecksum(downloadPath, checksum); err != nil {
			os.Remove(downloadPath)
			// Set status to downloading-update-error on checksum mismatch
			if err := redisClient.SetStatus(ctx, "downloading-update-error"); err != nil {
				log.Printf("Error setting status to downloading-update-error in Redis: %v", err)
			}
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		log.Println("Checksum verification successful")
	} else {
		log.Println("No checksum provided, skipping verification")
	}

	log.Println("Installing update...")
	// Set status to installing-updates
	if err := redisClient.SetStatus(ctx, "installing-updates"); err != nil {
		log.Printf("Error setting status to installing-updates in Redis: %v", err)
	}

	if err := menderClient.Install(downloadPath); err != nil {
		os.Remove(downloadPath)
		// Set status to installing-update-error on install error
		if err := redisClient.SetStatus(ctx, "installing-update-error"); err != nil {
			log.Printf("Error setting status to installing-update-error in Redis: %v", err)
		}
		return fmt.Errorf("error installing update: %w", err)
	}
	log.Println("Update installed successfully")

	// Only remove the file if it was downloaded (not a file:// URL)
	if !strings.HasPrefix(url, "file://") {
		if err := os.Remove(downloadPath); err != nil {
			log.Printf("Warning: Failed to remove downloaded file %s: %v", downloadPath, err)
		}
	}

	// Set final success status based on update type
	successStatus := "installation-complete-waiting-reboot" // Default for non-blocking
	if cfg.UpdateType == "blocking" {
		successStatus = "installation-complete-waiting-dashboard-reboot"
	}
	if err := redisClient.SetStatus(ctx, successStatus); err != nil {
		log.Printf("Error setting final success status in Redis: %v", err)
	}

	return nil
}
