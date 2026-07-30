package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type testCommandResult struct {
	status string
}

func TestRunCommandExecutionPollsRendersAndValidates(t *testing.T) {
	fetches := 0
	renders := 0
	validated := false
	err := runCommandExecution(context.Background(), commandExecution[testCommandResult]{
		PollInterval: time.Hour,
		PollTimeout:  time.Hour,
		Label:        "test command",
		Submit: func(context.Context) (*testCommandResult, bool, error) {
			return &testCommandResult{status: "pending"}, true, nil
		},
		Fetch: func(context.Context, *testCommandResult) (*testCommandResult, error) {
			fetches++
			return &testCommandResult{status: "success"}, nil
		},
		Status: func(result *testCommandResult) string { return result.status },
		Render: func(result testCommandResult, asyncHint bool) error {
			renders++
			if result.status != "success" || asyncHint {
				t.Fatalf("rendered result = %#v, asyncHint %v", result, asyncHint)
			}
			return nil
		},
		Validate: func(result testCommandResult) error {
			validated = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runCommandExecution: %v", err)
	}
	if fetches != 1 || renders != 1 || !validated {
		t.Fatalf("fetches=%d renders=%d validated=%v", fetches, renders, validated)
	}
}

func TestRunCommandExecutionAsyncStopsAfterPendingRender(t *testing.T) {
	err := runCommandExecution(context.Background(), commandExecution[testCommandResult]{
		Async: true,
		Submit: func(context.Context) (*testCommandResult, bool, error) {
			return &testCommandResult{status: "pending"}, true, nil
		},
		Fetch: func(context.Context, *testCommandResult) (*testCommandResult, error) {
			t.Fatal("async execution fetched a result")
			return nil, nil
		},
		Status: func(result *testCommandResult) string { return result.status },
		Render: func(result testCommandResult, asyncHint bool) error {
			if result.status != "pending" || !asyncHint {
				t.Fatalf("rendered result = %#v, asyncHint %v", result, asyncHint)
			}
			return nil
		},
		Validate: func(testCommandResult) error {
			t.Fatal("async execution performed terminal validation")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runCommandExecution: %v", err)
	}
}

func TestPollUntilCompleteCancellationAndTimeout(t *testing.T) {
	pending := func() (*testCommandResult, error) {
		return &testCommandResult{status: "pending"}, nil
	}
	status := func(result *testCommandResult) string { return result.status }

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pollUntilComplete(canceled, time.Hour, time.Hour, "test command", pending, status)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v, want context.Canceled", err)
	}

	_, err = pollUntilComplete(context.Background(), time.Hour, -time.Nanosecond, "test command", pending, status)
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for test command") {
		t.Fatalf("timeout error = %v", err)
	}
}
