package health

import (
	"testing"
	"time"
)

func TestEvaluatePriority(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	fresh := now.Add(-10 * time.Second)
	staleSample := now.Add(-(DefaultLateThreshold + time.Second))
	staleHeartbeat := now.Add(-(DefaultOfflineThreshold + time.Second))
	code := "E1"

	cases := []struct {
		name    string
		profile DeviceProfile
		want    State
	}{
		{
			name:    "fault beats offline and late",
			profile: DeviceProfile{FaultCode: code, LastHeartbeatAt: staleHeartbeat, LastSampleAt: &staleSample},
			want:    StateFault,
		},
		{
			name:    "fault beats late with fresh heartbeat",
			profile: DeviceProfile{FaultCode: code, LastHeartbeatAt: fresh, LastSampleAt: &staleSample},
			want:    StateFault,
		},
		{
			name:    "offline beats late",
			profile: DeviceProfile{LastHeartbeatAt: staleHeartbeat, LastSampleAt: &staleSample},
			want:    StateOffline,
		},
		{
			name:    "offline with zero heartbeat",
			profile: DeviceProfile{},
			want:    StateOffline,
		},
		{
			name:    "late with fresh heartbeat",
			profile: DeviceProfile{LastHeartbeatAt: fresh, LastSampleAt: &staleSample},
			want:    StateLate,
		},
		{
			name:    "healthy with fresh heartbeat and sample",
			profile: DeviceProfile{LastHeartbeatAt: fresh, LastSampleAt: &fresh},
			want:    StateHealthy,
		},
		{
			name:    "heartbeat-only with fresh heartbeat is never late",
			profile: DeviceProfile{LastHeartbeatAt: fresh, LastSampleAt: nil},
			want:    StateHealthy,
		},
		{
			name:    "future sample with fresh heartbeat is healthy",
			profile: DeviceProfile{LastHeartbeatAt: fresh, LastSampleAt: ptrTime(now.Add(time.Hour))},
			want:    StateHealthy,
		},
		{
			name:    "fault wins even with zero heartbeat",
			profile: DeviceProfile{FaultCode: code},
			want:    StateFault,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := Evaluate(tc.profile, now, DefaultLateThreshold, DefaultOfflineThreshold)
			if got != tc.want {
				t.Fatalf("Evaluate state = %q, want %q (reason %q)", got, tc.want, reason)
			}
			if got == StateHealthy && reason != "" {
				t.Fatalf("healthy profile must have empty reason, got %q", reason)
			}
			if got != StateHealthy && reason == "" {
				t.Fatalf("non-healthy profile must carry a reason")
			}
		})
	}
}

func TestEvaluateStrictThresholdBoundary(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)

	// 心跳年龄恰好等于失联阈值、采样新鲜：仍健康（严格 >）。
	atBoundary := now.Add(-DefaultOfflineThreshold)
	freshSample := now.Add(-time.Second)
	if got, _ := Evaluate(DeviceProfile{LastHeartbeatAt: atBoundary, LastSampleAt: &freshSample}, now, DefaultLateThreshold, DefaultOfflineThreshold); got != StateHealthy {
		t.Fatalf("heartbeat age == offline threshold: got %q, want healthy", got)
	}
	// 多 1ns 即失联。
	overBoundary := now.Add(-DefaultOfflineThreshold - time.Nanosecond)
	if got, _ := Evaluate(DeviceProfile{LastHeartbeatAt: overBoundary}, now, DefaultLateThreshold, DefaultOfflineThreshold); got != StateOffline {
		t.Fatalf("heartbeat age == threshold+1ns: got %q, want offline", got)
	}

	// 采样年龄恰好等于迟到阈值、心跳新鲜：仍健康。
	freshHeartbeat := now.Add(-time.Second)
	sampleAtBoundary := now.Add(-DefaultLateThreshold)
	if got, _ := Evaluate(DeviceProfile{LastHeartbeatAt: freshHeartbeat, LastSampleAt: &sampleAtBoundary}, now, DefaultLateThreshold, DefaultOfflineThreshold); got != StateHealthy {
		t.Fatalf("sample age == late threshold: got %q, want healthy", got)
	}
	// 多 1ns 即迟到。
	sampleOver := now.Add(-DefaultLateThreshold - time.Nanosecond)
	if got, _ := Evaluate(DeviceProfile{LastHeartbeatAt: freshHeartbeat, LastSampleAt: &sampleOver}, now, DefaultLateThreshold, DefaultOfflineThreshold); got != StateLate {
		t.Fatalf("sample age == threshold+1ns: got %q, want late", got)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
