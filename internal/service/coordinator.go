package service

import (
	"github.com/wyw14/solo-036/internal/compressor"
	"github.com/wyw14/solo-036/internal/journal"
	"github.com/wyw14/solo-036/internal/launcher"
	"github.com/wyw14/solo-036/internal/permit"
	"github.com/wyw14/solo-036/internal/pressure"
	"github.com/wyw14/solo-036/internal/receiver"
	"github.com/wyw14/solo-036/internal/route"
	"github.com/wyw14/solo-036/internal/run"
	"github.com/wyw14/solo-036/internal/telemetry"
	"github.com/wyw14/solo-036/internal/tracker"
	"github.com/wyw14/solo-036/internal/valve"
	"sync"
	"time"
)

type Runtime struct {
	Runs       *run.Store
	Routes     *route.Coordinator
	Tracker    *tracker.Coordinator
	Lineage    *tracker.SourceLineage
	Journal    *journal.Store
	Pressure   *pressure.Controller
	Permits    *permit.Evaluator
	Launcher   *launcher.Controller
	Valves     *valve.Controller
	Receiver   *receiver.Controller
	Compressor *compressor.Worker
	Telemetry  *telemetry.Recorder
	mu         sync.Mutex
	control    *ControlState
}

func NewRuntime() *Runtime {
	runs := run.NewStore()
	routes := route.NewCoordinator()
	tracks := tracker.NewCoordinator()
	lineage := tracker.NewSourceLineage()
	journalStore := journal.NewStore()
	p := pressure.NewController()
	permits := permit.NewEvaluator()
	launch := launcher.NewController(p, permits)
	valves := valve.NewController(p)
	recv := receiver.NewController()
	worker := compressor.NewWorker(p, runs)
	telemetryRecorder := telemetry.NewRecorder(telemetry.DefaultLateAfter, telemetry.DefaultLostAfter)
	return &Runtime{Runs: runs, Routes: routes, Tracker: tracks, Lineage: lineage, Journal: journalStore, Pressure: p, Permits: permits, Launcher: launch, Valves: valves, Receiver: recv, Compressor: worker, Telemetry: telemetryRecorder, control: defaultControlState()}
}

func (r *Runtime) ConfigureTelemetry(lateAfter, lostAfter time.Duration) {
	if lateAfter <= 0 || lostAfter <= lateAfter {
		return
	}
	r.Telemetry = telemetry.NewRecorder(lateAfter, lostAfter)
}
