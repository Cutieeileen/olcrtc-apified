package channel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/openlibrecommunity/olcrtc/internal/logger"
	"github.com/openlibrecommunity/olcrtc/internal/names"
	"github.com/openlibrecommunity/olcrtc/internal/provider/jazz"
	"github.com/openlibrecommunity/olcrtc/internal/provider/wbstream"
	"github.com/openlibrecommunity/olcrtc/internal/server"
)

const (
	carrierJazz     = "jazz"
	carrierWBStream = "wbstream"
	carrierTelemost = "telemost"

	telemostRoomURLPrefix = "https://telemost.yandex.ru/j/"

	statusCheckDelay = 3 * time.Second
)

var (
	// ErrMaxChannelsReached is returned when the channel limit is reached.
	ErrMaxChannelsReached = errors.New("maximum channel limit reached")
	// ErrChannelNotFound is returned when a channel is not found.
	ErrChannelNotFound = errors.New("channel not found")
	// ErrCarrierRequired is returned when no carrier is specified.
	ErrCarrierRequired = errors.New("carrier required")
	// ErrTransportRequired is returned when no transport is specified.
	ErrTransportRequired = errors.New("transport required")
	// ErrClientIDRequired is returned when no client ID is specified.
	ErrClientIDRequired = errors.New("client_id required")
	// ErrRoomIDRequired is returned when room ID is required but missing.
	ErrRoomIDRequired = errors.New("room_id required for telemost")
	// ErrTTLDaysRequired is returned when ttl_days is missing or zero in renew request.
	ErrTTLDaysRequired = errors.New("ttl_days required and must be positive")
)

// RunServerFunc is the function signature for starting a tunnel.
type RunServerFunc = func(
	ctx context.Context,
	linkName, transportName, carrierName, roomURL, keyHex, clientID string,
	dnsServer, socksProxyAddr string, socksProxyPort int,
	videoWidth, videoHeight, videoFPS int,
	videoBitrate, videoHW string,
	videoQRSize int, videoQRRecovery, videoCodec string,
	videoTileModule, videoTileRS int,
	vp8FPS, vp8BatchSize int,
	seiFPS, seiBatchSize, seiFragmentSize, seiAckTimeoutMS int,
) error

// runServerFunc is the function used to start a tunnel. Replaceable in tests.
//
//nolint:gochecknoglobals // Tests replace this with a stub.
var runServerFunc RunServerFunc = server.Run

// SetRunServerFunc replaces the tunnel runner. Used in tests to inject a no-op stub.
func SetRunServerFunc(fn RunServerFunc) { runServerFunc = fn }

// runningChannel holds runtime state for an active tunnel goroutine.
type runningChannel struct {
	ch     *Channel
	cancel context.CancelFunc
}

// Manager manages the lifecycle of multiple tunnel channels.
type Manager struct {
	store       Store
	mu          sync.RWMutex
	channels    map[string]*runningChannel
	maxChannels int
	dnsServer   string
}

// NewManager creates a new channel manager.
func NewManager(st Store, maxChannels int, dnsServer string) *Manager {
	return &Manager{
		store:       st,
		channels:    make(map[string]*runningChannel),
		maxChannels: maxChannels,
		dnsServer:   dnsServer,
	}
}

// Create validates the request, generates a key and room (if needed), persists the channel, and starts the tunnel.
//
//nolint:cyclop
func (m *Manager) Create(ctx context.Context, req CreateRequest) (*Channel, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}

	count, err := m.store.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count channels: %w", err)
	}
	if count >= m.maxChannels {
		return nil, fmt.Errorf("%w (%d/%d)", ErrMaxChannelsReached, count, m.maxChannels)
	}

	keyHex, err := GenerateKeyHex()
	if err != nil {
		return nil, err
	}

	roomID := req.RoomID
	if roomID == "" {
		roomID, err = m.generateRoomID(ctx, req.Carrier)
		if err != nil {
			return nil, fmt.Errorf("generate room: %w", err)
		}
	}

	lnk := req.Link
	if lnk == "" {
		lnk = DefaultLink
	}
	dns := req.DNSServer
	if dns == "" {
		dns = m.dnsServer
	}
	ttl := req.TTLDays
	if ttl <= 0 {
		ttl = DefaultTTLDays
	}

	now := time.Now().UTC().Truncate(time.Second)
	ch := &Channel{
		ID:              uuid.New().String(),
		Carrier:         req.Carrier,
		Transport:       req.Transport,
		Link:            lnk,
		RoomID:          roomID,
		ClientID:        req.ClientID,
		KeyHex:          keyHex,
		DNSServer:       dns,
		SOCKSProxyAddr:  req.SOCKSProxyAddr,
		SOCKSProxyPort:  req.SOCKSProxyPort,
		TransportConfig: req.TransportConfig,
		Status:          StatusStopped,
		ExpiresAt:       now.Add(time.Duration(ttl) * 24 * time.Hour),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := m.store.Create(ctx, ch); err != nil {
		return nil, fmt.Errorf("persist channel: %w", err)
	}

	m.startTunnel(ch) //nolint:contextcheck
	return ch, nil
}

// Get returns a channel by ID with live status.
func (m *Manager) Get(ctx context.Context, id string) (*Channel, error) {
	ch, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	if ch == nil {
		return nil, ErrChannelNotFound
	}

	m.mu.RLock()
	if rc, ok := m.channels[id]; ok {
		ch.Status = rc.ch.Status
		ch.StatusMessage = rc.ch.StatusMessage
	}
	m.mu.RUnlock()

	return ch, nil
}

// List returns all channels with live status.
func (m *Manager) List(ctx context.Context) ([]*Channel, error) {
	channels, err := m.store.List(ctx)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	m.mu.RLock()
	for _, ch := range channels {
		if rc, ok := m.channels[ch.ID]; ok {
			ch.Status = rc.ch.Status
			ch.StatusMessage = rc.ch.StatusMessage
		}
	}
	m.mu.RUnlock()

	return channels, nil
}

// Update stops the old tunnel, applies changes, persists, and restarts.
func (m *Manager) Update(ctx context.Context, id string, req UpdateRequest) (*Channel, error) {
	ch, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	if ch == nil {
		return nil, ErrChannelNotFound
	}

	m.stopTunnel(id)

	applyUpdate(ch, req)
	ch.UpdatedAt = time.Now().UTC().Truncate(time.Second)

	// If carrier changed and no explicit room_id, regenerate room.
	if req.Carrier != nil && req.RoomID == nil {
		newRoom, err := m.generateRoomID(ctx, ch.Carrier)
		if err != nil {
			return nil, fmt.Errorf("regenerate room: %w", err)
		}
		ch.RoomID = newRoom
	}

	if err := m.store.Update(ctx, ch); err != nil {
		return nil, fmt.Errorf("persist update: %w", err)
	}

	m.startTunnel(ch) //nolint:contextcheck
	return ch, nil
}

// Delete stops the tunnel and removes the channel from the store.
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.stopTunnel(id)
	if err := m.store.Delete(ctx, id); err != nil {
		return err //nolint:wrapcheck
	}
	return nil
}

// Renew extends the channel's expiration.
func (m *Manager) Renew(ctx context.Context, id string, req RenewRequest) (*Channel, error) {
	if req.TTLDays <= 0 {
		return nil, ErrTTLDaysRequired
	}

	ch, err := m.store.Get(ctx, id)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	if ch == nil {
		return nil, ErrChannelNotFound
	}

	now := time.Now().UTC().Truncate(time.Second)
	base := ch.ExpiresAt
	if base.Before(now) {
		base = now
	}
	ch.ExpiresAt = base.Add(time.Duration(req.TTLDays) * 24 * time.Hour)
	ch.UpdatedAt = now

	if err := m.store.Update(ctx, ch); err != nil {
		return nil, fmt.Errorf("persist renew: %w", err)
	}

	// Refresh live status.
	m.mu.RLock()
	if rc, ok := m.channels[id]; ok {
		ch.Status = rc.ch.Status
		ch.StatusMessage = rc.ch.StatusMessage
	}
	m.mu.RUnlock()

	return ch, nil
}

// Count returns the number of channels and the max limit.
func (m *Manager) Count(ctx context.Context) (int, int, error) {
	count, err := m.store.Count(ctx)
	return count, m.maxChannels, err //nolint:wrapcheck
}

// RestoreAll loads all non-expired channels from the store and starts their tunnels.
func (m *Manager) RestoreAll(ctx context.Context) error {
	// First clean up expired channels.
	expired, err := m.store.DeleteExpired(ctx)
	if err != nil {
		logger.Warnf("cleanup expired on restore: %v", err)
	}
	if len(expired) > 0 {
		logger.Infof("cleaned %d expired channels on startup", len(expired))
	}

	channels, err := m.store.List(ctx)
	if err != nil {
		return fmt.Errorf("list channels for restore: %w", err)
	}

	for _, ch := range channels {
		logger.Infof("restoring channel %s (%s/%s)", ch.ID, ch.Carrier, ch.Transport)
		m.startTunnel(ch) //nolint:contextcheck
	}

	if len(channels) > 0 {
		logger.Infof("restored %d channels", len(channels))
	}
	return nil
}

// StopAll cancels all running tunnels.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rc := range m.channels {
		logger.Infof("stopping channel %s", id)
		rc.cancel()
	}
	m.channels = make(map[string]*runningChannel)
}

func (m *Manager) startTunnel(ch *Channel) {
	ctx, cancel := context.WithCancel(context.Background())
	rc := &runningChannel{ch: ch, cancel: cancel}

	m.mu.Lock()
	m.channels[ch.ID] = rc
	m.mu.Unlock()

	m.updateStatus(ch.ID, StatusStarting, "")

	chID := ch.ID

	go func() {
		roomURL := buildRoomURL(ch.Carrier, ch.RoomID)
		tc := ch.TransportConfig

		err := runServerFunc(ctx,
			ch.Link, ch.Transport, ch.Carrier, roomURL,
			ch.KeyHex, ch.ClientID, ch.DNSServer,
			ch.SOCKSProxyAddr, ch.SOCKSProxyPort,
			tc.VideoWidth, tc.VideoHeight, tc.VideoFPS,
			tc.VideoBitrate, tc.VideoHW,
			tc.VideoQRSize, tc.VideoQRRecovery, tc.VideoCodec,
			tc.VideoTileModule, tc.VideoTileRS,
			tc.VP8FPS, tc.VP8BatchSize,
			tc.SEIFPS, tc.SEIBatchSize, tc.SEIFragmentSize, tc.SEIAckTimeoutMS,
		)
		if err != nil && ctx.Err() == nil {
			logger.Warnf("channel %s tunnel error: %v", chID, err)
			m.updateStatus(chID, StatusError, err.Error()) //nolint:contextcheck
		} else {
			m.updateStatus(chID, StatusStopped, "") //nolint:contextcheck
		}
	}()

	// Monitor startup: if still starting after delay, mark as running.
	go func() {
		time.Sleep(statusCheckDelay)
		m.mu.RLock()
		rc2, ok := m.channels[chID]
		m.mu.RUnlock()
		if ok && rc2 == rc && rc2.ch.Status == StatusStarting {
			m.updateStatus(chID, StatusRunning, "")
		}
	}()
}

func (m *Manager) stopTunnel(id string) {
	m.mu.Lock()
	rc, ok := m.channels[id]
	if ok {
		rc.cancel()
		delete(m.channels, id)
	}
	m.mu.Unlock()
}

func (m *Manager) updateStatus(id string, status Status, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rc, ok := m.channels[id]; ok {
		rc.ch.Status = status
		rc.ch.StatusMessage = msg
	}
	// Also persist status to store.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := m.store.Get(ctx, id)
	if err != nil || ch == nil {
		return
	}
	ch.Status = status
	ch.StatusMessage = msg
	ch.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	_ = m.store.Update(ctx, ch)
}

func (m *Manager) generateRoomID(ctx context.Context, carrier string) (string, error) {
	switch carrier {
	case carrierJazz:
		info, err := jazz.CreateRoom(ctx)
		if err != nil {
			return "", fmt.Errorf("jazz.CreateRoom: %w", err)
		}
		return info.RoomID, nil
	case carrierWBStream:
		roomID, err := wbstream.CreateRoom(ctx, names.Generate())
		if err != nil {
			return "", fmt.Errorf("wbstream.CreateRoom: %w", err)
		}
		return roomID, nil
	default:
		return "", fmt.Errorf("carrier %s does not support room generation", carrier) //nolint:err113
	}
}

func validateCreateRequest(req CreateRequest) error {
	if req.Carrier == "" {
		return ErrCarrierRequired
	}
	if req.Transport == "" {
		return ErrTransportRequired
	}
	if req.ClientID == "" {
		return ErrClientIDRequired
	}
	if req.Carrier == carrierTelemost && req.RoomID == "" {
		return ErrRoomIDRequired
	}
	return nil
}

//nolint:cyclop
func applyUpdate(ch *Channel, req UpdateRequest) {
	if req.Carrier != nil {
		ch.Carrier = *req.Carrier
	}
	if req.Transport != nil {
		ch.Transport = *req.Transport
	}
	if req.Link != nil {
		ch.Link = *req.Link
	}
	if req.RoomID != nil {
		ch.RoomID = *req.RoomID
	}
	if req.ClientID != nil {
		ch.ClientID = *req.ClientID
	}
	if req.KeyHex != nil {
		ch.KeyHex = *req.KeyHex
	}
	if req.DNSServer != nil {
		ch.DNSServer = *req.DNSServer
	}
	if req.SOCKSProxyAddr != nil {
		ch.SOCKSProxyAddr = *req.SOCKSProxyAddr
	}
	if req.SOCKSProxyPort != nil {
		ch.SOCKSProxyPort = *req.SOCKSProxyPort
	}
	if req.TTLDays != nil && *req.TTLDays > 0 {
		ch.ExpiresAt = time.Now().UTC().Add(time.Duration(*req.TTLDays) * 24 * time.Hour)
	}
	if req.TransportConfig != nil {
		ch.TransportConfig = *req.TransportConfig
	}
}

func buildRoomURL(carrier, roomID string) string {
	switch carrier {
	case carrierTelemost:
		return telemostRoomURLPrefix + roomID
	case carrierJazz:
		if roomID == "" {
			return "any"
		}
		return roomID
	default:
		return roomID
	}
}
