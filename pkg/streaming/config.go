package streaming

import "time"

type StreamCleanupConfig struct {
	Enable        bool
	CleanInterval time.Duration
}
