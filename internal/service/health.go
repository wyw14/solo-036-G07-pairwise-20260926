package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wyw14/solo-036/internal/health"
	"github.com/wyw14/solo-036/internal/model"
)

// 站点维度校验错误，由 API 层映射为对应的 HTTP 状态码。
var (
	ErrUnknownStation  = errors.New("unknown station")
	ErrStationDisabled = errors.New("station is disabled")
)

// ReportTelemetry 接收一次站点设备遥测上报（数据或纯心跳），自动建档并留存原始报文。
// 调用方需保证传入的 ReceivedAt 为零值（由本方法盖服务端时间戳）。
func (r *Runtime) ReportTelemetry(in health.Report) (health.DeviceProfile, error) {
	in.StationID = strings.TrimSpace(in.StationID)
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	if in.StationID == "" {
		return health.DeviceProfile{}, errors.New("station id is required")
	}
	if in.DeviceID == "" {
		return health.DeviceProfile{}, errors.New("device id is required")
	}
	control := r.ensureControl()
	station, ok := control.Stations[in.StationID]
	if !ok {
		return health.DeviceProfile{}, fmt.Errorf("%w %q", ErrUnknownStation, in.StationID)
	}
	if !station.Enabled {
		return health.DeviceProfile{}, fmt.Errorf("%w %q", ErrStationDisabled, in.StationID)
	}

	in.ReceivedAt = time.Now().UTC()
	// Health.Store 自带互斥锁；此处不持有 r.mu，Store 也不回调 Runtime，无锁序环。
	result := r.Health.Report(in)
	if result.FaultRaised {
		// 边沿触发的低频设备故障审计事件：非 run 维度，不写入 model.Incident。
		if err := r.appendEvent(model.Event{
			Kind:    "device_fault",
			Payload: in.StationID + "|" + in.DeviceID + "|" + result.FaultCode,
			At:      result.Message.ReceivedAt,
		}); err != nil {
			return health.DeviceProfile{}, err
		}
	}
	return result.Profile, nil
}

// HealthProfiles 返回全部设备档案，健康结论按当前时间实时计算。
func (r *Runtime) HealthProfiles() []health.DeviceProfile {
	return r.Health.Profiles(time.Now().UTC())
}

// HealthMessages 返回某站点（必填）设备的原始上报报文，最新优先。
func (r *Runtime) HealthMessages(stationID, deviceID string, limit int) ([]health.RawMessage, error) {
	stationID = strings.TrimSpace(stationID)
	if stationID == "" {
		return nil, errors.New("station_id query parameter is required")
	}
	control := r.ensureControl()
	if _, ok := control.Stations[stationID]; !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownStation, stationID)
	}
	return r.Health.Messages(stationID, strings.TrimSpace(deviceID), limit), nil
}
