package operationwaiter

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/dolthub/cli/internal/dolthub"
)

const (
	DefaultInterval    = time.Second
	DefaultMaxInterval = 10 * time.Second
)

type Client interface {
	GetOperation(context.Context, string) (dolthub.Operation, error)
	GetOperationURL(context.Context, string) (dolthub.Operation, error)
}

type Fetch func(context.Context) (dolthub.Operation, error)
type Sleep func(context.Context, time.Duration) error

type Waiter struct {
	Client      Client
	Interval    time.Duration
	MaxInterval time.Duration
	Sleep       Sleep
	Random      func() float64
	Observe     func(dolthub.Operation)
}

type FailedError struct{ Operation dolthub.Operation }

func (e *FailedError) Error() string {
	if e.Operation.Error == nil {
		return fmt.Sprintf("operation %s failed", e.Operation.ID)
	}
	message := e.Operation.Error.Title
	if e.Operation.Error.Detail != "" {
		message += ": " + e.Operation.Error.Detail
	}
	if e.Operation.Error.Code != "" {
		message += fmt.Sprintf(" (%s)", e.Operation.Error.Code)
	}
	return message
}

func (w Waiter) Wait(ctx context.Context, ref dolthub.OperationRef) (dolthub.Operation, error) {
	if w.Client == nil {
		return dolthub.Operation{}, errors.New("operation client is required")
	}
	if ref.Href == "" {
		return dolthub.Operation{}, errors.New("operation reference href is required")
	}
	return w.wait(ctx, func(ctx context.Context) (dolthub.Operation, error) { return w.Client.GetOperationURL(ctx, ref.Href) })
}

func (w Waiter) WaitID(ctx context.Context, id string) (dolthub.Operation, error) {
	if w.Client == nil {
		return dolthub.Operation{}, errors.New("operation client is required")
	}
	if id == "" {
		return dolthub.Operation{}, errors.New("operation ID is required")
	}
	return w.wait(ctx, func(ctx context.Context) (dolthub.Operation, error) { return w.Client.GetOperation(ctx, id) })
}

func (w Waiter) wait(ctx context.Context, fetch Fetch) (dolthub.Operation, error) {
	interval := w.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	maximum := w.MaxInterval
	if maximum <= 0 {
		maximum = DefaultMaxInterval
	}
	if maximum < interval {
		maximum = interval
	}
	sleep := w.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	random := w.Random
	if random == nil {
		random = rand.Float64
	}
	for {
		operation, err := fetch(ctx)
		if err != nil {
			return dolthub.Operation{}, err
		}
		if w.Observe != nil {
			w.Observe(operation)
		}
		switch operation.Status {
		case dolthub.OperationSucceeded:
			return operation, nil
		case dolthub.OperationFailed:
			return operation, &FailedError{Operation: operation}
		case dolthub.OperationQueued, dolthub.OperationRunning:
		default:
			return operation, fmt.Errorf("operation %s has unknown status %q", operation.ID, operation.Status)
		}
		delay := time.Duration(float64(interval) * (0.8 + 0.4*random()))
		if delay > maximum {
			delay = maximum
		}
		if delay <= 0 {
			delay = time.Millisecond
		}
		if err := sleep(ctx, delay); err != nil {
			return operation, err
		}
		if interval < maximum {
			interval *= 2
			if interval > maximum {
				interval = maximum
			}
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
