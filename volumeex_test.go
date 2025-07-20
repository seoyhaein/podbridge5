package podbridge5

import (
	"context"
	"errors"
	"testing"
	"time"
)

// sentinel errors
var (
	errAlways    = errors.New("always fails")
	errTransient = errors.New("transient fails then success")
)

func TestWithRetry(t *testing.T) {
	type result struct {
		err           error
		calls         int
		expectedErr   error
		expectedCalls int
	}

	tests := []struct {
		name        string
		attempts    int
		baseDelay   time.Duration
		ctxFunc     func() context.Context
		fnFactory   func(*result) func() error
		expectErrIs error
	}{
		{
			name:      "success-first-try",
			attempts:  3,
			baseDelay: 10 * time.Millisecond,
			ctxFunc:   context.Background,
			fnFactory: func(r *result) func() error {
				return func() error {
					r.calls++
					return nil
				}
			},
			expectErrIs: nil,
		},
		{
			name:      "eventual-success-after-failures",
			attempts:  5,
			baseDelay: 1 * time.Millisecond,
			ctxFunc:   context.Background,
			fnFactory: func(r *result) func() error {
				failures := 3
				return func() error {
					r.calls++
					if r.calls <= failures {
						return errTransient
					}
					return nil
				}
			},
			expectErrIs: nil,
		},
		{
			name:      "always-fails",
			attempts:  4,
			baseDelay: 1 * time.Millisecond,
			ctxFunc:   context.Background,
			fnFactory: func(r *result) func() error {
				return func() error {
					r.calls++
					return errAlways
				}
			},
			expectErrIs: errAlways,
		},
		{
			name:      "attempts-leq-zero-treated-as-one",
			attempts:  0,
			baseDelay: 0,
			ctxFunc:   context.Background,
			fnFactory: func(r *result) func() error {
				return func() error {
					r.calls++
					return errAlways
				}
			},
			expectErrIs: errAlways,
		},
		{
			name:      "context-cancelled-before-call",
			attempts:  5,
			baseDelay: 10 * time.Millisecond,
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			fnFactory: func(r *result) func() error {
				return func() error {
					r.calls++
					return nil
				}
			},
			expectErrIs: context.Canceled,
		},
		{
			name:      "context-cancelled-during-backoff",
			attempts:  5,
			baseDelay: 50 * time.Millisecond,
			ctxFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				// cancel after a short delay so first call runs then cancel hits before 2nd delay completes
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
				return ctx
			},
			fnFactory: func(r *result) func() error {
				return func() error {
					r.calls++
					return errAlways
				}
			},
			expectErrIs: context.Canceled,
		},
		{
			name:      "baseDelay-zero-no-sleep-fast-retries",
			attempts:  3,
			baseDelay: 0,
			ctxFunc:   context.Background,
			fnFactory: func(r *result) func() error {
				return func() error {
					r.calls++
					return errAlways
				}
			},
			expectErrIs: errAlways,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r := &result{}
			ctx := tc.ctxFunc()
			start := time.Now()
			err := withRetry(ctx, tc.attempts, tc.baseDelay, tc.fnFactory(r))
			elapsed := time.Since(start)

			// Error expectation
			if tc.expectErrIs == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
			} else {
				if !errors.Is(err, tc.expectErrIs) && !errors.Is(err, context.Canceled) {
					t.Fatalf("expected error %v, got %v", tc.expectErrIs, err)
				}
			}

			// Attempts/calls assertions (heuristic)
			switch tc.name {
			case "success-first-try":
				if r.calls != 1 {
					t.Fatalf("expected 1 call, got %d", r.calls)
				}
			case "eventual-success-after-failures":
				if r.calls < 4 || r.calls > 5 {
					t.Fatalf("expected 4 (3 failures + 1 success) or 5 calls, got %d", r.calls)
				}
			case "always-fails":
				if r.calls != tc.attempts {
					t.Fatalf("expected %d calls, got %d", tc.attempts, r.calls)
				}
			case "attempts-leq-zero-treated-as-one":
				if r.calls != 1 {
					t.Fatalf("expected 1 call when attempts<=0, got %d", r.calls)
				}
			case "context-cancelled-before-call":
				if r.calls != 0 {
					t.Fatalf("expected 0 calls (ctx canceled before), got %d", r.calls)
				}
			case "context-cancelled-during-backoff":
				if r.calls < 1 {
					t.Fatalf("expected at least 1 call before cancellation, got %d", r.calls)
				}
			case "baseDelay-zero-no-sleep-fast-retries":
				// Should try attempts times quickly
				if r.calls != tc.attempts {
					t.Fatalf("expected %d calls, got %d", tc.attempts, r.calls)
				}
				if elapsed > 50*time.Millisecond {
					t.Fatalf("expected fast retries with zero delay, but elapsed=%v", elapsed)
				}
			}
		})
	}
}
