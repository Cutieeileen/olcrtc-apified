package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver (no CGO required)

	"github.com/openlibrecommunity/olcrtc/internal/channel"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS channels (
    id               TEXT PRIMARY KEY,
    carrier          TEXT NOT NULL,
    transport        TEXT NOT NULL,
    link             TEXT NOT NULL DEFAULT 'direct',
    room_id          TEXT NOT NULL DEFAULT '',
    client_id        TEXT NOT NULL,
    key_hex          TEXT NOT NULL,
    dns_server       TEXT NOT NULL DEFAULT '1.1.1.1:53',
    socks_proxy_addr TEXT NOT NULL DEFAULT '',
    socks_proxy_port INTEGER NOT NULL DEFAULT 0,
    transport_config TEXT NOT NULL DEFAULT '{}',
    status           TEXT NOT NULL DEFAULT 'stopped',
    status_message   TEXT NOT NULL DEFAULT '',
    expires_at       DATETIME NOT NULL,
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_channels_expires_at ON channels(expires_at);
`

// SQLiteStore implements channel.Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite database at the given path.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Create(ctx context.Context, c *channel.Channel) error {
	tcJSON, err := json.Marshal(c.TransportConfig)
	if err != nil {
		return fmt.Errorf("marshal transport config: %w", err)
	}

	const q = `INSERT INTO channels
		(id, carrier, transport, link, room_id, client_id, key_hex,
		 dns_server, socks_proxy_addr, socks_proxy_port, transport_config,
		 status, status_message, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.ExecContext(ctx, q,
		c.ID, c.Carrier, c.Transport, c.Link, c.RoomID, c.ClientID, c.KeyHex,
		c.DNSServer, c.SOCKSProxyAddr, c.SOCKSProxyPort, string(tcJSON),
		string(c.Status), c.StatusMessage,
		c.ExpiresAt.UTC().Format(time.RFC3339),
		c.CreatedAt.UTC().Format(time.RFC3339),
		c.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert channel: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (*channel.Channel, error) {
	const q = `SELECT id, carrier, transport, link, room_id, client_id, key_hex,
		dns_server, socks_proxy_addr, socks_proxy_port, transport_config,
		status, status_message, expires_at, created_at, updated_at
		FROM channels WHERE id = ?`

	c := &channel.Channel{}
	var tcJSON string
	var expiresAt, createdAt, updatedAt string

	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&c.ID, &c.Carrier, &c.Transport, &c.Link, &c.RoomID, &c.ClientID, &c.KeyHex,
		&c.DNSServer, &c.SOCKSProxyAddr, &c.SOCKSProxyPort, &tcJSON,
		&c.Status, &c.StatusMessage, &expiresAt, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get channel: %w", err)
	}

	if err := json.Unmarshal([]byte(tcJSON), &c.TransportConfig); err != nil {
		return nil, fmt.Errorf("unmarshal transport config: %w", err)
	}
	if err := parseTimes(c, expiresAt, createdAt, updatedAt); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]*channel.Channel, error) {
	const q = `SELECT id, carrier, transport, link, room_id, client_id, key_hex,
		dns_server, socks_proxy_addr, socks_proxy_port, transport_config,
		status, status_message, expires_at, created_at, updated_at
		FROM channels ORDER BY created_at`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	var channels []*channel.Channel
	for rows.Next() {
		c := &channel.Channel{}
		var tcJSON string
		var expiresAt, createdAt, updatedAt string

		if err := rows.Scan(
			&c.ID, &c.Carrier, &c.Transport, &c.Link, &c.RoomID, &c.ClientID, &c.KeyHex,
			&c.DNSServer, &c.SOCKSProxyAddr, &c.SOCKSProxyPort, &tcJSON,
			&c.Status, &c.StatusMessage, &expiresAt, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}

		if err := json.Unmarshal([]byte(tcJSON), &c.TransportConfig); err != nil {
			return nil, fmt.Errorf("unmarshal transport config: %w", err)
		}
		if err := parseTimes(c, expiresAt, createdAt, updatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

func (s *SQLiteStore) Update(ctx context.Context, c *channel.Channel) error {
	tcJSON, err := json.Marshal(c.TransportConfig)
	if err != nil {
		return fmt.Errorf("marshal transport config: %w", err)
	}

	const q = `UPDATE channels SET
		carrier = ?, transport = ?, link = ?, room_id = ?, client_id = ?, key_hex = ?,
		dns_server = ?, socks_proxy_addr = ?, socks_proxy_port = ?, transport_config = ?,
		status = ?, status_message = ?, expires_at = ?, updated_at = ?
		WHERE id = ?`

	res, err := s.db.ExecContext(ctx, q,
		c.Carrier, c.Transport, c.Link, c.RoomID, c.ClientID, c.KeyHex,
		c.DNSServer, c.SOCKSProxyAddr, c.SOCKSProxyPort, string(tcJSON),
		string(c.Status), c.StatusMessage,
		c.ExpiresAt.UTC().Format(time.RFC3339),
		c.UpdatedAt.UTC().Format(time.RFC3339),
		c.ID,
	)
	if err != nil {
		return fmt.Errorf("update channel: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("channel %s not found", c.ID)
	}
	return nil
}

func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM channels WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("channel %s not found", id)
	}
	return nil
}

func (s *SQLiteStore) DeleteExpired(ctx context.Context) ([]string, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := s.db.QueryContext(ctx, "SELECT id FROM channels WHERE expires_at <= ?", now)
	if err != nil {
		return nil, fmt.Errorf("select expired: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan expired id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(ids) > 0 {
		_, err = s.db.ExecContext(ctx, "DELETE FROM channels WHERE expires_at <= ?", now)
		if err != nil {
			return nil, fmt.Errorf("delete expired: %w", err)
		}
	}
	return ids, nil
}

func (s *SQLiteStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM channels").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count channels: %w", err)
	}
	return count, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func parseTimes(c *channel.Channel, expiresAt, createdAt, updatedAt string) error {
	var err error
	c.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return fmt.Errorf("parse expires_at: %w", err)
	}
	c.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return fmt.Errorf("parse created_at: %w", err)
	}
	c.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return fmt.Errorf("parse updated_at: %w", err)
	}
	return nil
}
