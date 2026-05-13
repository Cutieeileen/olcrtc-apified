// Package channel defines the channel model and lifecycle management for multi-tenant tunnels.
package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Status represents the current state of a channel tunnel.
type Status string

const (
	// StatusStarting indicates the tunnel is being established.
	StatusStarting Status = "starting"
	// StatusRunning indicates the tunnel is active and connected.
	StatusRunning Status = "running"
	// StatusStopped indicates the tunnel has been intentionally stopped.
	StatusStopped Status = "stopped"
	// StatusError indicates the tunnel has failed.
	StatusError Status = "error"
)

const (
	// DefaultTTLDays is the default channel time-to-live in days.
	DefaultTTLDays = 30
	// DefaultLink is the default link type.
	DefaultLink = "direct"
	// DefaultDNSServer is the default DNS server.
	DefaultDNSServer = "1.1.1.1:53"

	keyBytes = 32
)

// TransportConfig holds transport-specific parameters.
type TransportConfig struct {
	VideoWidth      int    `json:"video_width,omitempty"`
	VideoHeight     int    `json:"video_height,omitempty"`
	VideoFPS        int    `json:"video_fps,omitempty"`
	VideoBitrate    string `json:"video_bitrate,omitempty"`
	VideoHW         string `json:"video_hw,omitempty"`
	VideoCodec      string `json:"video_codec,omitempty"`
	VideoQRSize     int    `json:"video_qr_size,omitempty"`
	VideoQRRecovery string `json:"video_qr_recovery,omitempty"`
	VideoTileModule int    `json:"video_tile_module,omitempty"`
	VideoTileRS     int    `json:"video_tile_rs,omitempty"`
	VP8FPS          int    `json:"vp8_fps,omitempty"`
	VP8BatchSize    int    `json:"vp8_batch_size,omitempty"`
	SEIFPS          int    `json:"sei_fps,omitempty"`
	SEIBatchSize    int    `json:"sei_batch_size,omitempty"`
	SEIFragmentSize int    `json:"sei_fragment_size,omitempty"`
	SEIAckTimeoutMS int    `json:"sei_ack_timeout_ms,omitempty"`
}

// Channel represents a managed tunnel with its configuration and lifecycle metadata.
type Channel struct {
	ID              string          `json:"id"`
	Carrier         string          `json:"carrier"`
	Transport       string          `json:"transport"`
	Link            string          `json:"link"`
	RoomID          string          `json:"room_id"`
	ClientID        string          `json:"client_id"`
	KeyHex          string          `json:"key_hex"`
	DNSServer       string          `json:"dns_server"`
	SOCKSProxyAddr  string          `json:"socks_proxy_addr,omitempty"`
	SOCKSProxyPort  int             `json:"socks_proxy_port,omitempty"`
	TransportConfig TransportConfig `json:"transport_config"`
	Status          Status          `json:"status"`
	StatusMessage   string          `json:"status_message,omitempty"`
	ExpiresAt       time.Time       `json:"expires_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Store provides persistence for channel data.
type Store interface {
	// Create persists a new channel.
	Create(ctx context.Context, ch *Channel) error
	// Get retrieves a channel by ID.
	Get(ctx context.Context, id string) (*Channel, error)
	// List returns all channels.
	List(ctx context.Context) ([]*Channel, error)
	// Update replaces a channel's data.
	Update(ctx context.Context, ch *Channel) error
	// Delete removes a channel by ID.
	Delete(ctx context.Context, id string) error
	// DeleteExpired removes channels whose expires_at is in the past and returns their IDs.
	DeleteExpired(ctx context.Context) ([]string, error)
	// Count returns the total number of channels.
	Count(ctx context.Context) (int, error)
	// Close releases any resources held by the store.
	Close() error
}

// CreateRequest holds the parameters for creating a new channel.
type CreateRequest struct {
	Carrier         string          `json:"carrier"`
	Transport       string          `json:"transport"`
	Link            string          `json:"link,omitempty"`
	RoomID          string          `json:"room_id,omitempty"`
	ClientID        string          `json:"client_id"`
	DNSServer       string          `json:"dns_server,omitempty"`
	SOCKSProxyAddr  string          `json:"socks_proxy_addr,omitempty"`
	SOCKSProxyPort  int             `json:"socks_proxy_port,omitempty"`
	TTLDays         int             `json:"ttl_days,omitempty"`
	TransportConfig TransportConfig `json:"transport_config"`
}

// UpdateRequest holds the parameters for updating a channel.
type UpdateRequest struct {
	Carrier         *string          `json:"carrier,omitempty"`
	Transport       *string          `json:"transport,omitempty"`
	Link            *string          `json:"link,omitempty"`
	RoomID          *string          `json:"room_id,omitempty"`
	ClientID        *string          `json:"client_id,omitempty"`
	KeyHex          *string          `json:"key_hex,omitempty"`
	DNSServer       *string          `json:"dns_server,omitempty"`
	SOCKSProxyAddr  *string          `json:"socks_proxy_addr,omitempty"`
	SOCKSProxyPort  *int             `json:"socks_proxy_port,omitempty"`
	TTLDays         *int             `json:"ttl_days,omitempty"`
	TransportConfig *TransportConfig `json:"transport_config,omitempty"`
}

// RenewRequest holds the parameters for renewing a channel.
type RenewRequest struct {
	TTLDays int `json:"ttl_days"`
}

// GenerateKeyHex generates a random 32-byte encryption key and returns it as a 64-char hex string.
func GenerateKeyHex() (string, error) {
	key := make([]byte, keyBytes)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return hex.EncodeToString(key), nil
}
