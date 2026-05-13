package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/openlibrecommunity/olcrtc/internal/channel"
	"github.com/openlibrecommunity/olcrtc/internal/server"
	"github.com/openlibrecommunity/olcrtc/internal/store"
)

const testMasterKey = "test-secret-key-12345"

func init() {
	// Replace server.Run with a no-op for tests.
	channel.SetRunServerFunc(func(ctx context.Context,
		_, _, _, _, _, _ string,
		_ string, _ string, _ int,
		_ int, _ int, _ int,
		_ string, _ string,
		_ int, _ string, _ string,
		_ int, _ int,
		_ int, _ int,
		_ int, _ int, _ int, _ int,
	) error {
		<-ctx.Done()
		return nil
	})
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	mgr := channel.NewManager(st, 5, "1.1.1.1:53")
	srv := NewServer(mgr, testMasterKey, ":0")
	ts := httptest.NewServer(srv.httpServer.Handler)
	t.Cleanup(func() {
		mgr.StopAll()
		ts.Close()
	})
	return ts
}

func doReq(t *testing.T, ts *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}

	req, err := http.NewRequest(method, ts.URL+path, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testMasterKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

func TestAuthMiddleware(t *testing.T) {
	ts := testServer(t)

	// No auth header.
	req, _ := http.NewRequest("GET", ts.URL+"/api/v1/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}

	// Wrong key.
	req, _ = http.NewRequest("GET", ts.URL+"/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp2.StatusCode)
	}

	// Correct key.
	resp3 := doReq(t, ts, "GET", "/api/v1/status", nil)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp3.StatusCode)
	}
}

func TestStatus(t *testing.T) {
	ts := testServer(t)

	resp := doReq(t, ts, "GET", "/api/v1/status", nil)
	defer resp.Body.Close()

	var sr statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if sr.ActiveChannels != 0 {
		t.Errorf("ActiveChannels = %d, want 0", sr.ActiveChannels)
	}
	if sr.MaxChannels != 5 {
		t.Errorf("MaxChannels = %d, want 5", sr.MaxChannels)
	}
	if !sr.CanCreate {
		t.Error("CanCreate should be true")
	}
}

func TestCreateAndGetChannel(t *testing.T) {
	ts := testServer(t)

	// Create.
	createReq := channel.CreateRequest{
		Carrier:   "wbstream",
		Transport: "datachannel",
		ClientID:  "test-client",
		RoomID:    "test-room-123",
		TTLDays:   30,
	}

	resp := doReq(t, ts, "POST", "/api/v1/channels", createReq)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var errResp errorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("status = %d, want 201, error: %s", resp.StatusCode, errResp.Error)
	}

	var ch channel.Channel
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if ch.Carrier != "wbstream" {
		t.Errorf("Carrier = %q, want wbstream", ch.Carrier)
	}
	if ch.KeyHex == "" {
		t.Error("KeyHex should be generated")
	}
	if len(ch.KeyHex) != 64 {
		t.Errorf("KeyHex len = %d, want 64", len(ch.KeyHex))
	}

	// Get.
	resp2 := doReq(t, ts, "GET", "/api/v1/channels/"+ch.ID, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Get status = %d, want 200", resp2.StatusCode)
	}

	var ch2 channel.Channel
	if err := json.NewDecoder(resp2.Body).Decode(&ch2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if ch2.ID != ch.ID {
		t.Errorf("ID = %q, want %q", ch2.ID, ch.ID)
	}
}

func TestListChannels(t *testing.T) {
	ts := testServer(t)

	// Create two channels.
	for _, cid := range []string{"client-a", "client-b"} {
		resp := doReq(t, ts, "POST", "/api/v1/channels", channel.CreateRequest{
			Carrier:   "wbstream",
			Transport: "datachannel",
			ClientID:  cid,
			RoomID:    "room-" + cid,
		})
		resp.Body.Close()
	}

	resp := doReq(t, ts, "GET", "/api/v1/channels", nil)
	defer resp.Body.Close()

	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if lr.Total != 2 {
		t.Errorf("Total = %d, want 2", lr.Total)
	}
}

func TestDeleteChannel(t *testing.T) {
	ts := testServer(t)

	resp := doReq(t, ts, "POST", "/api/v1/channels", channel.CreateRequest{
		Carrier:   "wbstream",
		Transport: "datachannel",
		ClientID:  "del-client",
		RoomID:    "del-room",
	})
	var ch channel.Channel
	_ = json.NewDecoder(resp.Body).Decode(&ch)
	resp.Body.Close()

	resp2 := doReq(t, ts, "DELETE", "/api/v1/channels/"+ch.ID, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("Delete status = %d, want 204", resp2.StatusCode)
	}

	resp3 := doReq(t, ts, "GET", "/api/v1/channels/"+ch.ID, nil)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("Get after delete status = %d, want 404", resp3.StatusCode)
	}
}

func TestRenewChannel(t *testing.T) {
	ts := testServer(t)

	resp := doReq(t, ts, "POST", "/api/v1/channels", channel.CreateRequest{
		Carrier:   "wbstream",
		Transport: "datachannel",
		ClientID:  "renew-client",
		RoomID:    "renew-room",
		TTLDays:   1,
	})
	var ch channel.Channel
	_ = json.NewDecoder(resp.Body).Decode(&ch)
	resp.Body.Close()

	resp2 := doReq(t, ts, "POST", "/api/v1/channels/"+ch.ID+"/renew", channel.RenewRequest{TTLDays: 60})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Renew status = %d, want 200", resp2.StatusCode)
	}

	var renewed channel.Channel
	_ = json.NewDecoder(resp2.Body).Decode(&renewed)
	if !renewed.ExpiresAt.After(ch.ExpiresAt) {
		t.Errorf("ExpiresAt should be extended: got %v, original %v", renewed.ExpiresAt, ch.ExpiresAt)
	}
}

func TestMaxChannelsLimit(t *testing.T) {
	ts := testServer(t) // max 5 channels

	// Create 5 channels.
	for i := range 5 {
		resp := doReq(t, ts, "POST", "/api/v1/channels", channel.CreateRequest{
			Carrier:   "wbstream",
			Transport: "datachannel",
			ClientID:  "limit-client",
			RoomID:    "limit-room-" + string(rune('0'+i)),
		})
		resp.Body.Close()
	}

	// 6th should fail.
	resp := doReq(t, ts, "POST", "/api/v1/channels", channel.CreateRequest{
		Carrier:   "wbstream",
		Transport: "datachannel",
		ClientID:  "limit-client",
		RoomID:    "limit-room-extra",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}
}

func TestCreateValidation(t *testing.T) {
	ts := testServer(t)

	tests := []struct {
		name string
		req  channel.CreateRequest
	}{
		{"no carrier", channel.CreateRequest{Transport: "datachannel", ClientID: "c", RoomID: "r"}},
		{"unsupported carrier", channel.CreateRequest{Carrier: "unknown", Transport: "datachannel", ClientID: "c"}},
		{"no transport", channel.CreateRequest{Carrier: "wbstream", ClientID: "c", RoomID: "r"}},
		{"unsupported transport", channel.CreateRequest{
			Carrier: "wbstream", Transport: "unknown", ClientID: "c", RoomID: "r",
		}},
		{"no client_id", channel.CreateRequest{Carrier: "wbstream", Transport: "datachannel", RoomID: "r"}},
		{"telemost no room", channel.CreateRequest{Carrier: "telemost", Transport: "datachannel", ClientID: "c"}},
		{"wbstream no room", channel.CreateRequest{Carrier: "wbstream", Transport: "datachannel", ClientID: "c"}},
		{"videochannel missing config", channel.CreateRequest{
			Carrier: "wbstream", Transport: "videochannel", ClientID: "c", RoomID: "r",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := doReq(t, ts, "POST", "/api/v1/channels", tt.req)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

// Suppress unused import warnings — server package is only used via init().
var _ = server.Run
