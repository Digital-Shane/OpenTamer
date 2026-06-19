package app

import (
	"errors"

	"github.com/Digital-Shane/open-tamer/internal/core"
)

var ErrMissingAdapter = errors.New("missing app adapter")

type ApplicationObserver interface {
	AppProcessHints() ([]core.AppProcessHint, error)
	FrontmostAppID() (core.AppID, error)
}

type ProcessEnumerator interface {
	Processes() ([]core.ProcessRef, error)
}

type MetricsSampler interface {
	core.ProcessMetricsSampler
	core.SystemMetricsSampler
}

type AudioActivityObserver interface {
	AudioActive() (bool, error)
}

type Adapters struct {
	Applications ApplicationObserver
	Processes    ProcessEnumerator
	Metrics      MetricsSampler
	Audio        AudioActivityObserver
	SystemPolicy core.SystemPolicyObserver
	Signals      SignalController
	Priority     PriorityController
	Lifecycle    LifecycleController
	Notifier     Notifier
}

func (adapters Adapters) Validate() error {
	switch {
	case adapters.Applications == nil:
		return errors.Join(ErrMissingAdapter, errors.New("applications"))
	case adapters.Processes == nil:
		return errors.Join(ErrMissingAdapter, errors.New("processes"))
	case adapters.Metrics == nil:
		return errors.Join(ErrMissingAdapter, errors.New("metrics"))
	case adapters.Audio == nil:
		return errors.Join(ErrMissingAdapter, errors.New("audio"))
	case adapters.SystemPolicy == nil:
		return errors.Join(ErrMissingAdapter, errors.New("system policy"))
	case adapters.Signals == nil:
		return errors.Join(ErrMissingAdapter, errors.New("signals"))
	case adapters.Priority == nil:
		return errors.Join(ErrMissingAdapter, errors.New("priority"))
	case adapters.Lifecycle == nil:
		return errors.Join(ErrMissingAdapter, errors.New("lifecycle"))
	case adapters.Notifier == nil:
		return errors.Join(ErrMissingAdapter, errors.New("notifier"))
	default:
		return nil
	}
}
