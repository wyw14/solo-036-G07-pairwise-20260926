package model

import (
	"errors"
	"strings"
	"time"
)

type TelemetryHealth string

const (
	TelemetryHealthy TelemetryHealth = "healthy"
	TelemetryLate    TelemetryHealth = "late"
	TelemetryLost    TelemetryHealth = "lost"
	TelemetryFault   TelemetryHealth = "fault"
)

type TelemetryReport struct {
	ID         string    `json:"id"`
	StationID  string    `json:"station_id"`
	DeviceID   string    `json:"device_id"`
	Condition  string    `json:"condition"`
	Message    string    `json:"message"`
	SampledAt  time.Time `json:"sampled_at"`
	ReceivedAt time.Time `json:"received_at"`
}

type DeviceHealthProfile struct {
	DeviceID        string          `json:"device_id"`
	StationID       string          `json:"station_id"`
	Health          TelemetryHealth `json:"health"`
	Reason          string          `json:"reason"`
	Reports         int             `json:"reports"`
	LastCondition   string          `json:"last_condition,omitempty"`
	LastSampledAt   *time.Time      `json:"last_sampled_at,omitempty"`
	LastHeartbeatAt *time.Time      `json:"last_heartbeat_at,omitempty"`
	EvaluatedAt     time.Time       `json:"evaluated_at"`
}

func ValidateTelemetryReport(report TelemetryReport) error {
	if strings.TrimSpace(report.StationID) == "" {
		return errors.New("telemetry station is required")
	}
	if strings.TrimSpace(report.DeviceID) == "" {
		return errors.New("telemetry device id is required")
	}
	if report.SampledAt.IsZero() {
		return errors.New("telemetry sample time is required")
	}
	if report.ReceivedAt.IsZero() {
		return errors.New("telemetry receive time is required")
	}
	switch report.Condition {
	case "", "ok", "fault":
		return nil
	default:
		return errors.New("telemetry condition must be ok or fault")
	}
}
