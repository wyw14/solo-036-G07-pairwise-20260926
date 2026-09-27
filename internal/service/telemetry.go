package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wyw14/solo-036/internal/model"
)

type TelemetryReportRequest struct {
	StationID string
	DeviceID  string
	Condition string
	Message   string
	SampledAt time.Time
}

func (r *Runtime) ReportTelemetry(request TelemetryReportRequest) (model.DeviceHealthProfile, error) {
	control := r.ensureControl()
	if _, ok := control.Stations[request.StationID]; !ok {
		return model.DeviceHealthProfile{}, fmt.Errorf("unknown telemetry station %s", request.StationID)
	}
	if strings.TrimSpace(request.DeviceID) == "" {
		return model.DeviceHealthProfile{}, errors.New("telemetry device id is required")
	}
	if request.Condition == "" {
		request.Condition = "ok"
	}
	if request.SampledAt.IsZero() {
		request.SampledAt = time.Now().UTC()
	}
	report := model.TelemetryReport{
		ID:         model.NewRevision(),
		StationID:  request.StationID,
		DeviceID:   request.DeviceID,
		Condition:  request.Condition,
		Message:    request.Message,
		SampledAt:  request.SampledAt,
		ReceivedAt: time.Now().UTC(),
	}
	if err := r.Telemetry.Record(report); err != nil {
		return model.DeviceHealthProfile{}, err
	}
	return r.Telemetry.Profile(report.DeviceID, time.Now().UTC()), nil
}

func (r *Runtime) TelemetryHealth() []model.DeviceHealthProfile {
	equipment := r.EquipmentStatus()
	known := make([]string, 0, len(equipment))
	for _, item := range equipment {
		known = append(known, item.Name)
	}
	return r.Telemetry.Profiles(known, time.Now().UTC())
}

func (r *Runtime) TelemetryMessages(deviceID string) ([]model.TelemetryReport, error) {
	if strings.TrimSpace(deviceID) == "" {
		return nil, errors.New("device_id is required")
	}
	if !r.Telemetry.Known(deviceID) {
		return nil, fmt.Errorf("unknown telemetry device %s", deviceID)
	}
	return r.Telemetry.Messages(deviceID), nil
}
