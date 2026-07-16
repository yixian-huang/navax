package maintenance

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type UpdateSupervisor struct {
	service        *UpdateService
	requestRestart func()
	now            func() time.Time
	tickInterval   time.Duration
	checkInterval  time.Duration
}

func NewUpdateSupervisor(service *UpdateService, requestRestart func()) *UpdateSupervisor {
	return &UpdateSupervisor{
		service: service, requestRestart: requestRestart, now: time.Now,
		tickInterval: time.Minute, checkInterval: 6 * time.Hour,
	}
}

func (s *UpdateSupervisor) Run(ctx context.Context) {
	if s == nil || s.service == nil {
		return
	}
	if err := s.Tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.WarnContext(ctx, "automatic update cycle failed", "error", err)
	}
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.WarnContext(ctx, "automatic update cycle failed", "error", err)
			}
		}
	}
}

func (s *UpdateSupervisor) Tick(ctx context.Context) error {
	state, err := s.service.State(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	if state.AutoCheck && (state.CheckedAt == nil || now.Sub(*state.CheckedAt) >= s.checkInterval) {
		state, err = s.service.Check(ctx)
		if errors.Is(err, ErrUpdateNotConfigured) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	if !state.AutoApply || state.Status != "available" || state.LatestVersion == nil || !insideMaintenanceWindow(now, state.MaintenanceWindow) {
		return nil
	}
	state, err = s.service.Apply(ctx, *state.LatestVersion, "")
	if err != nil {
		return err
	}
	if state.Status == "restart-required" && s.requestRestart != nil {
		s.requestRestart()
	}
	return nil
}

func insideMaintenanceWindow(now time.Time, window *string) bool {
	if window == nil || *window == "" {
		return true
	}
	start, end, err := parseMaintenanceWindow(*window)
	if err != nil {
		return false
	}
	minute := now.Hour()*60 + now.Minute()
	if start < end {
		return minute >= start && minute < end
	}
	return minute >= start || minute < end
}
