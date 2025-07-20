package podbridge5

import (
	"context"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"strings"
	"time"
)

var (
	ErrVolumeNotFound = errors.New("volume not found")
)

type VolumeRemoveError struct {
	Name  string
	Cause error
}

func (e *VolumeRemoveError) Error() string {
	return fmt.Sprintf("remove volume %s: %v", e.Name, e.Cause)
}
func (e *VolumeRemoveError) Unwrap() error { return e.Cause }

func withRetry(ctx context.Context, attempts int, baseDelay time.Duration, fn func() error) error {
	if attempts <= 0 {
		attempts = 1
	}
	delay := baseDelay
	var err error
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = fn()
		if err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
		if delay < 2*time.Second {
			delay *= 2
		}
	}
	return err
}

func VolumeExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := withRetry(ctx, 3, 80*time.Millisecond, func() error {
		e, err := volumes.Exists(ctx, name, nil)
		if err != nil {
			return err
		}
		exists = e
		return nil
	})
	return exists, err
}

type RemoveBehavior struct {
	Force          bool
	RetryForce     bool
	IgnoreNotFound bool
	Attempts       int
}

func RemoveVolume(ctx context.Context, name string, beh *RemoveBehavior) error {
	if beh == nil {
		beh = &RemoveBehavior{}
	}
	if beh.Attempts <= 0 {
		beh.Attempts = 1
	}

	tryRemove := func(force bool) error {
		return volumes.Remove(ctx, name, &volumes.RemoveOptions{Force: &force})
	}

	err := withRetry(ctx, beh.Attempts, 100*time.Millisecond, func() error {
		err := tryRemove(beh.Force)
		if err == nil {
			return nil
		}
		if beh.RetryForce && !beh.Force {
			// force 재시도
			if ferr := tryRemove(true); ferr == nil {
				return nil
			} else {
				return ferr
			}
		}
		return err
	})
	if err != nil {
		if beh.IgnoreNotFound && isNotFoundErr(err) {
			return nil
		}
		return &VolumeRemoveError{Name: name, Cause: err}
	}
	return nil
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Podman error 문자열 패턴 기반 (필요 시 개선)
	if strings.Contains(msg, "404") || strings.Contains(msg, "not found") {
		return true
	}
	return false
}

type CreateFn func(ctx context.Context, name string) (*types.VolumeConfigResponse, error)

func OverwriteVolume(ctx context.Context, name string, create CreateFn) (*types.VolumeConfigResponse, error) {
	exists, err := VolumeExists(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("check exists: %w", err)
	}
	if exists {
		if err := RemoveVolume(ctx, name, &RemoveBehavior{
			Force:          false,
			RetryForce:     true,
			IgnoreNotFound: true,
			Attempts:       3,
		}); err != nil {
			return nil, fmt.Errorf("remove before overwrite: %w", err)
		}
	}
	vcr, err := create(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("create volume: %w", err)
	}
	return vcr, nil
}
