package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wyw14/solo-036/internal/model"
)

const messageKeep = 20

const (
	DefaultLateAfter = 30 * time.Second
	DefaultLostAfter = 120 * time.Second
)

type Recorder struct {
	mu        sync.Mutex
	lateAfter time.Duration
	lostAfter time.Duration
	latest    map[string]model.TelemetryReport
	reports   map[string]int
	messages  map[string][]model.TelemetryReport
}

func NewRecorder(lateAfter, lostAfter time.Duration) *Recorder {
	return &Recorder{
		lateAfter: lateAfter,
		lostAfter: lostAfter,
		latest:    make(map[string]model.TelemetryReport),
		reports:   make(map[string]int),
		messages:  make(map[string][]model.TelemetryReport),
	}
}

func Ready(r *Recorder) bool { return r != nil && r.latest != nil && r.messages != nil }

func (r *Recorder) Record(report model.TelemetryReport) error {
	if err := model.ValidateTelemetryReport(report); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latest[report.DeviceID] = report
	r.reports[report.DeviceID]++
	kept := append(r.messages[report.DeviceID], report)
	if len(kept) > messageKeep {
		kept = kept[len(kept)-messageKeep:]
	}
	r.messages[report.DeviceID] = kept
	return nil
}

func (r *Recorder) Profile(deviceID string, now time.Time) model.DeviceHealthProfile {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.profileLocked(deviceID, now)
}

func (r *Recorder) Profiles(known []string, now time.Time) []model.DeviceHealthProfile {
	r.mu.Lock()
	defer r.mu.Unlock()
	devices := make(map[string]bool, len(known)+len(r.latest))
	for _, deviceID := range known {
		if strings.TrimSpace(deviceID) != "" {
			devices[deviceID] = true
		}
	}
	for deviceID := range r.latest {
		devices[deviceID] = true
	}
	profiles := make([]model.DeviceHealthProfile, 0, len(devices))
	for deviceID := range devices {
		profiles = append(profiles, r.profileLocked(deviceID, now))
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].DeviceID < profiles[j].DeviceID })
	return profiles
}

func (r *Recorder) Messages(deviceID string) []model.TelemetryReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.TelemetryReport(nil), r.messages[deviceID]...)
}

func (r *Recorder) Known(deviceID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.latest[deviceID]
	return ok
}

func (r *Recorder) profileLocked(deviceID string, now time.Time) model.DeviceHealthProfile {
	profile := model.DeviceHealthProfile{
		DeviceID:    deviceID,
		StationID:   stationOf(deviceID),
		Reports:     r.reports[deviceID],
		EvaluatedAt: now,
	}
	latest, ok := r.latest[deviceID]
	if ok {
		sampledAt, heartbeatAt := latest.SampledAt, latest.ReceivedAt
		profile.StationID = latest.StationID
		profile.LastCondition = latest.Condition
		profile.LastSampledAt = &sampledAt
		profile.LastHeartbeatAt = &heartbeatAt
	}
	profile.Health, profile.Reason = classify(latest, ok, r.lateAfter, r.lostAfter, now)
	return profile
}

func classify(latest model.TelemetryReport, ok bool, lateAfter, lostAfter time.Duration, now time.Time) (model.TelemetryHealth, string) {
	if !ok {
		return model.TelemetryLost, "no telemetry has ever been received from this device"
	}
	if silence := now.Sub(latest.ReceivedAt); silence > lostAfter {
		return model.TelemetryLost, fmt.Sprintf("heartbeat silent for %s (limit %s)", silence.Round(time.Second), lostAfter)
	}
	if latest.Condition == "fault" {
		return model.TelemetryFault, "device reported a fault in its latest heartbeat"
	}
	if age := now.Sub(latest.SampledAt); age > lateAfter {
		return model.TelemetryLate, fmt.Sprintf("latest sample is %s old (limit %s)", age.Round(time.Second), lateAfter)
	}
	return model.TelemetryHealthy, "heartbeat and samples are within limits"
}

func stationOf(deviceID string) string {
	if station, _, found := strings.Cut(deviceID, "/"); found {
		return station
	}
	return ""
}
