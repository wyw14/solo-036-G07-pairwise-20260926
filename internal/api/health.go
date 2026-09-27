package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wyw14/solo-036/internal/health"
	"github.com/wyw14/solo-036/internal/service"
)

type telemetryRequest struct {
	StationID string  `json:"station_id"`
	DeviceID  string  `json:"device_id"`
	SampleAt  string  `json:"sample_at"`
	FaultCode *string `json:"fault_code"`
}

// Telemetry 接收站点设备的数据上报或纯心跳（同一端点，省略 sample_at 即心跳）。
func (h *Handler) Telemetry(w http.ResponseWriter, r *http.Request) {
	var request telemetryRequest
	raw, err := decodeBodyRaw(w, r, &request)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	request.StationID = strings.TrimSpace(request.StationID)
	request.DeviceID = strings.TrimSpace(request.DeviceID)
	if request.StationID == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("station_id is required"))
		return
	}
	if request.DeviceID == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("device_id is required"))
		return
	}
	report := health.Report{
		StationID: request.StationID,
		DeviceID:  request.DeviceID,
		FaultCode: request.FaultCode,
		Raw:       raw,
	}
	// 与 parseObservedAt 不同：空白/省略 sample_at 表示纯心跳，保留零值而非补当前时间。
	if strings.TrimSpace(request.SampleAt) != "" {
		sampleAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(request.SampleAt))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, errors.New("sample_at must use RFC3339 format"))
			return
		}
		report.SampleAt = sampleAt.UTC()
	}
	profile, err := h.runtime.ReportTelemetry(report)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnknownStation):
			writeError(w, r, http.StatusNotFound, err)
		case errors.Is(err, service.ErrStationDisabled):
			writeError(w, r, http.StatusConflict, err)
		default:
			writeError(w, r, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// HealthProfiles 返回全部站点设备的健康档案（结论实时计算）。
func (h *Handler) HealthProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.runtime.HealthProfiles())
}

// HealthMessages 返回某站点设备的原始上报报文，最新优先。
func (h *Handler) HealthMessages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	stationID := strings.TrimSpace(query.Get("station_id"))
	if stationID == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("station_id query parameter is required"))
		return
	}
	deviceID := strings.TrimSpace(query.Get("device_id"))
	limit := health.MaxMessages
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, r, http.StatusBadRequest, errors.New("limit must be a positive integer"))
			return
		}
		limit = parsed
	}
	messages, err := h.runtime.HealthMessages(stationID, deviceID, limit)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnknownStation):
			writeError(w, r, http.StatusNotFound, err)
		default:
			writeError(w, r, http.StatusBadRequest, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, messages)
}
