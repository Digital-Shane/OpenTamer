package app

import (
	"errors"

	"github.com/Digital-Shane/open-tamer/internal/core"
	apppolicy "github.com/Digital-Shane/open-tamer/internal/policy"
)

type ProcessIdentityLookup interface {
	ProcessIdentity(int) (core.ProcessRef, error)
}

type automaticCPUValidator struct {
	ProcessGenerationValidator
	lookup ProcessIdentityLookup
}

func (validator automaticCPUValidator) ValidateAutomaticProcess(process core.ProcessRef) error {
	current, err := validator.lookup.ProcessIdentity(process.ID.PID)
	if err != nil {
		return err
	}
	if !current.ID.SameGeneration(process.ID) {
		return ErrProcessGenerationMismatch
	}
	if current.ExecutablePath != process.ExecutablePath || current.UID != process.UID || !apppolicy.GlobalCPUProcessSafe(current) {
		return errors.New("process is no longer eligible for automatic CPU limiting")
	}
	return nil
}
