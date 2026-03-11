package logger

import (
	"fmt"
	"os"
	"sync"

	cfg "github.com/jsol/tenex-platform/internal/platform/config"
	apperrors "github.com/jsol/tenex-platform/internal/platform/errors"
	logcore "github.com/jsol/tenex-platform/internal/platform/logger/core"
	"github.com/jsol/tenex-platform/internal/platform/logger/exporters/loki"
	"github.com/jsol/tenex-platform/internal/platform/logger/facade"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type loggerSnapshot struct {
	logger  *logcore.Logger
	cleanup func() error
	stop    func()
	atom    *zap.AtomicLevel
}

type loggerState struct {
	mu         sync.Mutex
	snap       *loggerSnapshot
	shutdownWg sync.WaitGroup
}

func (s *loggerState) update(next *loggerSnapshot) *loggerSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.snap
	s.snap = next
	facade.Init(next.logger)
	return old
}

func (s *loggerState) shutdown() error {
	s.mu.Lock()
	snap := s.snap
	s.mu.Unlock()
	s.shutdownWg.Wait()

	return shutdownSnapshot(snap)
}

func (s *loggerState) setLevel(level zapcore.Level) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap != nil {
		s.snap.atom.SetLevel(level)
	}
}

func (s *loggerState) replaceSnapshot(next *loggerSnapshot) {
	old := s.update(next)
	if old == nil {
		return
	}
	s.shutdownWg.Add(1)
	go func() {
		defer s.shutdownWg.Done()
		if err := shutdownSnapshot(old); err != nil {
			facade.L().Error("old logger snapshot shutdown error",
				zap.Error(apperrors.Wrap(err, apperrors.ErrInternal, "logger.snapshot")),
			)
		}
	}()
}

func shutdownSnapshot(snap *loggerSnapshot) error {
	if snap == nil {
		return nil
	}
	var err error
	if snap.cleanup != nil {
		err = snap.cleanup()
	}
	if snap.stop != nil {
		snap.stop()
	}
	return err
}

func Bootstrap(mgr *cfg.Manager) (func() error, error) {
	appCfg := mgr.Get()

	snap, err := buildSnapshot(appCfg)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrInternal, "logger.bootstrap")
	}

	state := &loggerState{}
	state.update(snap)

	unsubscribe := mgr.AddListener(func(old, newCfg *cfg.AppConfig) {
		levelChanged := old == nil || old.App.LogLevel != newCfg.App.LogLevel
		lokiChanged := old == nil || old.Observability.LokiEndpoint != newCfg.Observability.LokiEndpoint

		if levelChanged && !lokiChanged {
			state.setLevel(logcore.ParseLevel(newCfg.App.LogLevel))
			return
		}

		if !levelChanged && !lokiChanged {
			return
		}

		next, buildErr := buildSnapshot(newCfg)
		if buildErr != nil {
			fmt.Fprintf(os.Stderr, "[logger] rebuild failed: %v\n", buildErr)
			return
		}

		state.replaceSnapshot(next)
	})

	return func() error {
		unsubscribe()
		if err := state.shutdown(); err != nil {
			return apperrors.Wrap(err, apperrors.ErrInternal, "logger.shutdown")
		}
		return nil
	}, nil
}

func buildSnapshot(appCfg *cfg.AppConfig) (*loggerSnapshot, error) {
	level := logcore.ParseLevel(appCfg.App.LogLevel)
	atom := zap.NewAtomicLevelAt(level)
	atomPtr := &atom

	var extraCores []zapcore.Core
	stop := func() {}

	if appCfg.Observability.LokiEndpoint != "" {
		lokiWriter := loki.NewWriter(
			appCfg.Observability.LokiEndpoint,
			loki.Labels{
				"app": appCfg.App.Name,
				"env": appCfg.App.Env,
			},
		)

		stopCh := make(chan struct{})
		runDone := make(chan struct{})

		var once sync.Once
		stop = func() {
			once.Do(func() { close(stopCh) })
			<-runDone
		}

		go func() {
			defer close(runDone)
			lokiWriter.Run(0, stopCh)
		}()

		extraCores = append(extraCores, loki.NewCore(lokiWriter, atomPtr))
	}

	l, cleanup := logcore.Build(logcore.BuildOptions{
		AtomicLevel: atom,
		ExtraCores:  extraCores,
		Async:       true,
		Sampling:    appCfg.App.Env != "local",
	})

	l = l.With(
		zap.String("service", appCfg.App.Name),
		zap.String("env", appCfg.App.Env),
	)

	return &loggerSnapshot{
		logger:  l,
		cleanup: cleanup,
		stop:    stop,
		atom:    atomPtr,
	}, nil
}
