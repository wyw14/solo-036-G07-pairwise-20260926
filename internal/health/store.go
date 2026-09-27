package health

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wyw14/solo-036/internal/model"
)

// deviceKey 以结构体作键，避免设备 ID 含 "/"（如 station-a/main）带来的拼接转义问题。
type deviceKey struct {
	stationID string
	deviceID  string
}

// Store 是站点遥测健康档案与原始报文的内存存储，自带互斥锁。
// 与 journal.Store / pressure.history 不同，原始报文按设备有界环形留存。
type Store struct {
	mu       sync.Mutex
	profiles map[deviceKey]*DeviceProfile
	messages map[deviceKey][]RawMessage // 每设备环形，旧 -> 新
}

func NewStore() *Store {
	return &Store{
		profiles: make(map[deviceKey]*DeviceProfile),
		messages: make(map[deviceKey][]RawMessage),
	}
}

// Report 记录一次站点上报：首次见到的 (站点, 设备) 自动建档，并把逐字报文
// 追加进该设备的有界环形留存。返回值均为拷贝。
func (s *Store) Report(in Report) ReportResult {
	receivedAt := in.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	key := deviceKey{stationID: in.StationID, deviceID: in.DeviceID}

	s.mu.Lock()
	defer s.mu.Unlock()

	profile, ok := s.profiles[key]
	if !ok {
		profile = &DeviceProfile{
			StationID:       in.StationID,
			DeviceID:        in.DeviceID,
			FirstSeenAt:     receivedAt,
			LastHeartbeatAt: receivedAt,
		}
		s.profiles[key] = profile
	}
	oldCode := profile.FaultCode

	// 任何上报（含纯心跳）都代表一次连接心跳。
	profile.LastHeartbeatAt = receivedAt
	profile.UpdatedAt = receivedAt

	// FaultCode 指针三态：nil 保持原状；指向 "" 清除；指向非空置位。
	if in.FaultCode != nil {
		profile.FaultCode = strings.TrimSpace(*in.FaultCode)
	}

	heartbeat := in.SampleAt.IsZero()
	if !heartbeat {
		// 采样时间取历史最大值，乱序/重试的旧采样不得把档案回拨。
		if profile.LastSampleAt == nil || in.SampleAt.After(*profile.LastSampleAt) {
			sampleAt := in.SampleAt
			profile.LastSampleAt = &sampleAt
		}
	}

	message := RawMessage{
		ID:         model.NewRevision(),
		StationID:  in.StationID,
		DeviceID:   in.DeviceID,
		Raw:        in.Raw,
		ReceivedAt: receivedAt,
		FaultCode:  profile.FaultCode,
		Heartbeat:  heartbeat,
	}
	// 单条消息的 SampleAt 只反映本次上报：纯心跳消息保持 nil。
	if !heartbeat {
		at := in.SampleAt
		message.SampleAt = &at
	}
	s.messages[key] = append(s.messages[key], message)
	if excess := len(s.messages[key]) - MaxMessages; excess > 0 {
		s.messages[key] = append([]RawMessage(nil), s.messages[key][excess:]...)
	}

	newCode := profile.FaultCode
	result := ReportResult{
		Profile:     cloneProfile(profile),
		Message:     message,
		FaultRaised: newCode != "" && newCode != oldCode,
		FaultCode:   newCode,
	}
	state, reason := Evaluate(result.Profile, receivedAt, DefaultLateThreshold, DefaultOfflineThreshold)
	result.Profile.State = state
	result.Profile.Reason = reason
	return result
}

// Profiles 返回全部档案的深拷贝，State/Reason 按 now 实时计算，
// 按 (station_id, device_id) 排序；无档案时返回非 nil 空切片。
func (s *Store) Profiles(now time.Time) []DeviceProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]DeviceProfile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		clone := cloneProfile(profile)
		clone.State, clone.Reason = Evaluate(clone, now, DefaultLateThreshold, DefaultOfflineThreshold)
		out = append(out, clone)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StationID != out[j].StationID {
			return out[i].StationID < out[j].StationID
		}
		return out[i].DeviceID < out[j].DeviceID
	})
	return out
}

// Messages 返回某站点（必填）下某设备（deviceID 为空表示该站点全部设备）的原始报文，
// 最新优先，最多 limit 条；limit 被夹取到 [1, MaxMessages]（<=0 取 MaxMessages）。
// 结果始终是非 nil 切片。
func (s *Store) Messages(stationID, deviceID string, limit int) []RawMessage {
	if limit <= 0 || limit > MaxMessages {
		limit = MaxMessages
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RawMessage, 0)
	for key, ring := range s.messages {
		if key.stationID != stationID {
			continue
		}
		if deviceID != "" && key.deviceID != deviceID {
			continue
		}
		// 各设备环形为旧 -> 新，逆序追加得到该设备的最新优先。
		for i := len(ring) - 1; i >= 0; i-- {
			out = append(out, ring[i])
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ReceivedAt.Equal(out[j].ReceivedAt) {
			return out[i].ReceivedAt.After(out[j].ReceivedAt)
		}
		return out[i].ID > out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func cloneProfile(p *DeviceProfile) DeviceProfile {
	clone := *p
	if p.LastSampleAt != nil {
		sampleAt := *p.LastSampleAt
		clone.LastSampleAt = &sampleAt
	}
	return clone
}
