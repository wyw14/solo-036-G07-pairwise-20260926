package health

import "time"

// Evaluate 依据档案在 now 时刻的实时健康结论，是无 I/O、无时钟、无锁的纯函数，
// 阈值由调用方注入以便穷举测试。
//
// 判定优先级：故障 > 失联 > 迟到 > 健康。年龄与阈值做严格比较：恰好等于阈值
// 仍视为新鲜（"未超过"），超过 1ns 才翻转。
func Evaluate(p DeviceProfile, now time.Time, lateThreshold, offlineThreshold time.Duration) (State, string) {
	// 故障：设备自报故障码，无论数据是否陈旧，优先级最高。
	if p.FaultCode != "" {
		return StateFault, reasonFault
	}
	// 失联：从未收到心跳，或最近一次心跳已严格超过失联阈值。
	if p.LastHeartbeatAt.IsZero() || now.Sub(p.LastHeartbeatAt) > offlineThreshold {
		return StateOffline, reasonOffline
	}
	// 迟到：心跳新鲜，但最新采样严格早于迟到阈值。仅发过心跳的设备不算迟到。
	if p.LastSampleAt != nil && now.Sub(*p.LastSampleAt) > lateThreshold {
		return StateLate, reasonLate
	}
	return StateHealthy, ""
}
