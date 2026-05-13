package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/openlibrecommunity/olcrtc/internal/channel"
)

type statusResponse struct {
	ActiveChannels int              `json:"active_channels"`
	MaxChannels    int              `json:"max_channels"`
	AvailableSlots int              `json:"available_slots"`
	CanCreate      bool             `json:"can_create"`
	Channels       []channelSummary `json:"channels"`
}

type channelSummary struct {
	ID            string         `json:"id"`
	Carrier       string         `json:"carrier"`
	Transport     string         `json:"transport"`
	Status        channel.Status `json:"status"`
	StatusMessage string         `json:"status_message,omitempty"`
}

type listResponse struct {
	Channels []*channel.Channel `json:"channels"`
	Total    int                `json:"total"`
}

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	var req channel.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ch, err := s.manager.Create(r.Context(), req)
	if err != nil {
		status := errorStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, ch)
}

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.manager.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if channels == nil {
		channels = []*channel.Channel{}
	}

	writeJSON(w, http.StatusOK, listResponse{
		Channels: channels,
		Total:    len(channels),
	})
}

func (s *Server) handleGetChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ch, err := s.manager.Get(r.Context(), id)
	if err != nil {
		status := errorStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req channel.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ch, err := s.manager.Update(r.Context(), id, req)
	if err != nil {
		status := errorStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.manager.Delete(r.Context(), id); err != nil {
		status := errorStatus(err)
		writeError(w, status, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRenewChannel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req channel.RenewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ch, err := s.manager.Renew(r.Context(), id, req)
	if err != nil {
		status := errorStatus(err)
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ch)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	count, max, err := s.manager.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	channels, err := s.manager.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	summaries := make([]channelSummary, 0, len(channels))
	for _, ch := range channels {
		summaries = append(summaries, channelSummary{
			ID:            ch.ID,
			Carrier:       ch.Carrier,
			Transport:     ch.Transport,
			Status:        ch.Status,
			StatusMessage: ch.StatusMessage,
		})
	}

	writeJSON(w, http.StatusOK, statusResponse{
		ActiveChannels: count,
		MaxChannels:    max,
		AvailableSlots: max - count,
		CanCreate:      count < max,
		Channels:       summaries,
	})
}

func errorStatus(err error) int {
	switch {
	case errors.Is(err, channel.ErrChannelNotFound):
		return http.StatusNotFound
	case errors.Is(err, channel.ErrMaxChannelsReached):
		return http.StatusConflict
	case errors.Is(err, channel.ErrCarrierRequired),
		errors.Is(err, channel.ErrTransportRequired),
		errors.Is(err, channel.ErrClientIDRequired),
		errors.Is(err, channel.ErrRoomIDRequired),
		errors.Is(err, channel.ErrTTLDaysRequired),
		errors.Is(err, channel.ErrUnsupportedCarrier),
		errors.Is(err, channel.ErrUnsupportedTransport),
		errors.Is(err, channel.ErrVideoWidthRequired),
		errors.Is(err, channel.ErrVideoHeightRequired),
		errors.Is(err, channel.ErrVideoFPSRequired),
		errors.Is(err, channel.ErrVideoBitrateRequired),
		errors.Is(err, channel.ErrVideoHWRequired):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
