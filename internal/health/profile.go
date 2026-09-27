package health

import "time"

// DeviceProfile 是单台站点设备的遥测健康档案。State/Reason 不持久化在档案内，
// 而是在读出时由 Evaluate 依据时间戳实时计算。
type DeviceProfile struct {
	StationID       string     `json:"station_id"`
	DeviceID        string     `json:"device_id"`
	State           State      `json:"state"`
	Reason          string     `json:"reason,omitempty"`
	FaultCode       string     `json:"fault_code,omitempty"`
	LastSampleAt    *time.Time `json:"last_sample_at,omitempty"` // nil 表示仅收到过心跳
	LastHeartbeatAt time.Time  `json:"last_heartbeat_at"`
	FirstSeenAt     time.Time  `json:"first_seen_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// RawMessage 是站点上报的逐字原始报文及其到达元数据。
type RawMessage struct {
	ID         string     `json:"id"`
	StationID  string     `json:"station_id"`
	DeviceID   string     `json:"device_id"`
	Raw        string     `json:"raw"`
	SampleAt   *time.Time `json:"sample_at,omitempty"` // nil 表示纯心跳
	ReceivedAt time.Time  `json:"received_at"`
	FaultCode  string     `json:"fault_code,omitempty"`
	Heartbeat  bool       `json:"heartbeat"`
}

// Report 是一次站点上报。SampleAt 为零值时表示纯心跳，不更新 LastSampleAt；
// FaultCode 为 nil 时保持原故障状态，指向 "" 表示清除，指向非空串表示置位。
type Report struct {
	StationID  string
	DeviceID   string
	SampleAt   time.Time
	FaultCode  *string
	Raw        string
	ReceivedAt time.Time // UTC；零值时由 Store 填充
}

// ReportResult 是 Store.Report 的结果：档案副本、落库的原始报文，
// 以及本次是否产生了新的（边沿触发）故障码。
type ReportResult struct {
	Profile     DeviceProfile
	Message     RawMessage
	FaultRaised bool
	FaultCode   string
}
