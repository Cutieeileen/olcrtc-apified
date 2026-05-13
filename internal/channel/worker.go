package channel

import (
	"context"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/logger"
)

// DefaultExpiryCheckInterval is the default interval for checking expired channels.
const DefaultExpiryCheckInterval = 1 * time.Minute

// StartExpirationWorker periodically removes expired channels.
func (m *Manager) StartExpirationWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.cleanExpired()
		}
	}
}

func (m *Manager) cleanExpired() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	expired, err := m.store.DeleteExpired(ctx)
	if err != nil {
		logger.Warnf("expiration worker: %v", err)
		return
	}

	for _, id := range expired {
		m.stopTunnel(id)
		logger.Infof("expired channel %s removed", id)
	}
}
