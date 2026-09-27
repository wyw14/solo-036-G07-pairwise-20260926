package health

import (
	"sync"
	"testing"
	"time"
)

func strptr(s string) *string { return &s }

func TestReportAutoCreatesProfile(t *testing.T) {
	store := NewStore()
	received := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	sample := received.Add(-2 * time.Second)
	result := store.Report(Report{
		StationID:  "station-a",
		DeviceID:   "station-a/main",
		SampleAt:   sample,
		Raw:        `{"station_id":"station-a"}`,
		ReceivedAt: received,
	})
	p := result.Profile
	if p.StationID != "station-a" || p.DeviceID != "station-a/main" {
		t.Fatalf("unexpected profile identity: %+v", p)
	}
	if !p.FirstSeenAt.Equal(received) || !p.LastHeartbeatAt.Equal(received) || !p.UpdatedAt.Equal(received) {
		t.Fatalf("first report must stamp all timestamps at received time: %+v", p)
	}
	if p.LastSampleAt == nil || !p.LastSampleAt.Equal(sample) {
		t.Fatalf("last sample = %v, want %v", p.LastSampleAt, sample)
	}
	if p.State != StateHealthy {
		t.Fatalf("fresh report state = %q, want healthy", p.State)
	}
	if result.Message.Heartbeat {
		t.Fatalf("data report must not be flagged heartbeat")
	}
	if result.Message.Raw == "" || result.Message.ID == "" {
		t.Fatalf("raw message must be stored with an id: %+v", result.Message)
	}
	if msgs := store.Messages("station-a", "station-a/main", 10); len(msgs) != 1 {
		t.Fatalf("messages len = %d, want 1", len(msgs))
	}
}

func TestHeartbeatOnlyKeepsSampleAndFault(t *testing.T) {
	store := NewStore()
	t0 := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	sample := t0.Add(-time.Second)
	store.Report(Report{StationID: "station-a", DeviceID: "d", SampleAt: sample, FaultCode: strptr("E1"), ReceivedAt: t0})

	t1 := t0.Add(30 * time.Second)
	result := store.Report(Report{StationID: "station-a", DeviceID: "d", ReceivedAt: t1}) // 纯心跳、无 fault_code
	p := result.Profile
	if !p.LastHeartbeatAt.Equal(t1) || !p.UpdatedAt.Equal(t1) {
		t.Fatalf("heartbeat must advance heartbeat timestamps: %+v", p)
	}
	if p.LastSampleAt == nil || !p.LastSampleAt.Equal(sample) {
		t.Fatalf("heartbeat must not change last sample: %v", p.LastSampleAt)
	}
	if p.FaultCode != "E1" {
		t.Fatalf("bare heartbeat must not clear fault, got %q", p.FaultCode)
	}
	if !result.Message.Heartbeat || result.Message.SampleAt != nil {
		t.Fatalf("ping message must be heartbeat-only with nil sample: %+v", result.Message)
	}
}

func TestOutOfOrderSampleDoesNotMoveBackward(t *testing.T) {
	store := NewStore()
	t0 := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	newer := t0
	store.Report(Report{StationID: "s", DeviceID: "d", SampleAt: newer, ReceivedAt: t0})
	store.Report(Report{StationID: "s", DeviceID: "d", SampleAt: t0.Add(-time.Minute), ReceivedAt: t0.Add(time.Second)})
	profiles := store.Profiles(t0.Add(2 * time.Second))
	if len(profiles) != 1 {
		t.Fatalf("profiles len = %d, want 1", len(profiles))
	}
	if profiles[0].LastSampleAt == nil || !profiles[0].LastSampleAt.Equal(newer) {
		t.Fatalf("old out-of-order sample must not move LastSampleAt backward: %v", profiles[0].LastSampleAt)
	}
}

func TestFaultPointerSemanticsAndEdges(t *testing.T) {
	store := NewStore()
	t0 := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)

	r1 := store.Report(Report{StationID: "s", DeviceID: "d", FaultCode: strptr("E1"), ReceivedAt: t0})
	if !r1.FaultRaised || r1.FaultCode != "E1" {
		t.Fatalf("empty -> E1 must raise, got raised=%v code=%q", r1.FaultRaised, r1.FaultCode)
	}
	r2 := store.Report(Report{StationID: "s", DeviceID: "d", FaultCode: strptr("E1"), ReceivedAt: t0.Add(time.Second)})
	if r2.FaultRaised {
		t.Fatalf("same code E1 -> E1 must not raise again")
	}
	r3 := store.Report(Report{StationID: "s", DeviceID: "d", FaultCode: nil, ReceivedAt: t0.Add(2 * time.Second)})
	if r3.FaultRaised || r3.Profile.FaultCode != "E1" {
		t.Fatalf("nil pointer must leave fault unchanged (raised=%v code=%q)", r3.FaultRaised, r3.Profile.FaultCode)
	}
	r4 := store.Report(Report{StationID: "s", DeviceID: "d", FaultCode: strptr("E2"), ReceivedAt: t0.Add(3 * time.Second)})
	if !r4.FaultRaised || r4.FaultCode != "E2" {
		t.Fatalf("E1 -> E2 must raise, got raised=%v code=%q", r4.FaultRaised, r4.FaultCode)
	}
	r5 := store.Report(Report{StationID: "s", DeviceID: "d", FaultCode: strptr(""), ReceivedAt: t0.Add(4 * time.Second)})
	if r5.FaultRaised || r5.Profile.FaultCode != "" {
		t.Fatalf("explicit empty code must clear without raising (raised=%v code=%q)", r5.FaultRaised, r5.Profile.FaultCode)
	}
}

func TestMessageRingBounded(t *testing.T) {
	store := NewStore()
	t0 := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	var firstID string
	for i := 0; i < MaxMessages+5; i++ {
		result := store.Report(Report{StationID: "s", DeviceID: "d", ReceivedAt: t0.Add(time.Duration(i) * time.Second)})
		if i == 0 {
			firstID = result.Message.ID
		}
	}
	msgs := store.Messages("s", "d", MaxMessages+50) // 超限应被夹取到 MaxMessages
	if len(msgs) != MaxMessages {
		t.Fatalf("ring len = %d, want %d", len(msgs), MaxMessages)
	}
	for _, m := range msgs {
		if m.ID == firstID {
			t.Fatalf("oldest message %s should have been evicted", firstID)
		}
	}
	if msgs[0].ReceivedAt != t0.Add(time.Duration(MaxMessages+4)*time.Second) {
		t.Fatalf("messages must be newest-first, got newest received_at %v", msgs[0].ReceivedAt)
	}
}

func TestMessagesFilteringAndLimit(t *testing.T) {
	store := NewStore()
	t0 := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		store.Report(Report{StationID: "station-a", DeviceID: "d1", ReceivedAt: t0.Add(time.Duration(i) * time.Second)})
		store.Report(Report{StationID: "station-a", DeviceID: "d2", ReceivedAt: t0.Add(time.Duration(i) * time.Second)})
		store.Report(Report{StationID: "station-b", DeviceID: "d3", ReceivedAt: t0.Add(time.Duration(i) * time.Second)})
	}
	if msgs := store.Messages("station-a", "", 0); len(msgs) != 6 { // limit 0 夹取为 MaxMessages
		t.Fatalf("station-wide messages = %d, want 6", len(msgs))
	}
	if msgs := store.Messages("station-a", "d1", 2); len(msgs) != 2 {
		t.Fatalf("device-filtered limited messages = %d, want 2", len(msgs))
	} else if msgs[0].DeviceID != "d1" {
		t.Fatalf("filter leaked device %q", msgs[0].DeviceID)
	}
	if msgs := store.Messages("station-a", "unknown", 10); len(msgs) != 0 {
		t.Fatalf("unknown device messages = %d, want 0", len(msgs))
	}
	if msgs := store.Messages("station-x", "", 10); msgs == nil || len(msgs) != 0 {
		t.Fatalf("unknown station must return non-nil empty slice, got %#v", msgs)
	}
}

func TestProfilesSortedAndIsolated(t *testing.T) {
	store := NewStore()
	t0 := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	store.Report(Report{StationID: "station-b", DeviceID: "z", ReceivedAt: t0})
	store.Report(Report{StationID: "station-a", DeviceID: "m", ReceivedAt: t0})
	store.Report(Report{StationID: "station-a", DeviceID: "a", ReceivedAt: t0})

	profiles := store.Profiles(t0)
	want := []struct{ station, device string }{
		{"station-a", "a"}, {"station-a", "m"}, {"station-b", "z"},
	}
	if len(profiles) != len(want) {
		t.Fatalf("profiles len = %d, want %d", len(profiles), len(want))
	}
	for i, w := range want {
		if profiles[i].StationID != w.station || profiles[i].DeviceID != w.device {
			t.Fatalf("profiles[%d] = %s/%s, want %s/%s", i, profiles[i].StationID, profiles[i].DeviceID, w.station, w.device)
		}
	}
	// 改返回值不得影响存储。
	profiles[0].FaultCode = "MUTATED"
	again := store.Profiles(t0)
	if again[0].FaultCode != "" {
		t.Fatalf("returned profile must be an isolated copy, store has %q", again[0].FaultCode)
	}
}

func TestReportConcurrent(t *testing.T) {
	store := NewStore()
	t0 := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			device := "d"
			if g%2 == 0 {
				device = "d0"
			}
			for i := 0; i < 10; i++ {
				store.Report(Report{
					StationID:  "station-a",
					DeviceID:   device,
					ReceivedAt: t0.Add(time.Duration(g*10+i) * time.Second),
				})
			}
		}(g)
	}
	wg.Wait()
	for _, device := range []string{"d", "d0"} {
		if len(store.Messages("station-a", device, MaxMessages)) > MaxMessages {
			t.Fatalf("ring for %s exceeded bound", device)
		}
	}
}
