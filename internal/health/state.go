package health

import "time"

// State 是站点设备遥测健康结论。
type State string

const (
	StateHealthy State = "healthy" // 健康
	StateLate    State = "late"    // 迟到：心跳新鲜但采样陈旧
	StateOffline State = "offline" // 失联：心跳超时
	StateFault   State = "fault"   // 故障：设备自报故障码
)

// 判定阈值与留存上限，是整个功能唯一的调参点。
const (
	// DefaultLateThreshold 呼应 GrantPassage 中 90s 的跟踪连续性窗口。
	DefaultLateThreshold = 90 * time.Second
	// DefaultOfflineThreshold 是心跳被判定为失联的超时时间。
	DefaultOfflineThreshold = 5 * time.Minute
	// MaxMessages 是每台设备保留的原始报文上限（有界环形）。
	MaxMessages = 100
)

// 状态原因文案，随结论一并返回给值守人员。
const (
	reasonFault   = "device self-reported a fault"
	reasonOffline = "heartbeat exceeded offline threshold"
	reasonLate    = "latest sample exceeded late threshold"
)
