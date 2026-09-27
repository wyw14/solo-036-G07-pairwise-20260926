package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/wyw14/solo-036/internal/service"
)

type telemetryReportRequest struct {
	StationID string `json:"station_id"`
	DeviceID  string `json:"device_id"`
	SampledAt string `json:"sampled_at"`
	Condition string `json:"condition"`
	Message   string `json:"message"`
}

func decodeBodyRaw(w http.ResponseWriter, r *http.Request, destination any) (string, error) {
	reader := http.MaxBytesReader(w, r.Body, requestLimit)
	defer reader.Close()
	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", errors.New("request body is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return "", err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", errors.New("request body must contain one JSON value")
	}
	return string(raw), nil
}

func (h *Handler) ReportTelemetry(w http.ResponseWriter, r *http.Request) {
	var request telemetryReportRequest
	raw, err := decodeBodyRaw(w, r, &request)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	sampledAt, err := parseObservedAt(request.SampledAt)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	message := strings.TrimSpace(request.Message)
	if message == "" {
		message = raw
	}
	profile, err := h.runtime.ReportTelemetry(service.TelemetryReportRequest{
		StationID: request.StationID,
		DeviceID:  request.DeviceID,
		Condition: request.Condition,
		Message:   message,
		SampledAt: sampledAt,
	})
	if err != nil {
		writeError(w, r, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

func (h *Handler) TelemetryHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.runtime.TelemetryHealth())
}

func (h *Handler) TelemetryMessages(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceID == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("device_id is required"))
		return
	}
	messages, err := h.runtime.TelemetryMessages(deviceID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}
