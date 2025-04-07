package podbridge5

import (
	"context"
	"github.com/containers/image/v5/manifest"
	"reflect"
	"testing"
	"time"
)

func TestCreateContainer(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	// WithHealthChecker 는 주석 처리되어 있으므로, 기본 옵션만 사용
	spec, err := NewSpec(
		WithImageName("docker.io/library/busybox:latest"),
		WithName("testerhaha"),
		WithTerminal(true),
	)
	if err != nil {
		t.Fatalf("failed to create ContainerSpec: %v", err)
	}
	if spec == nil {
		t.Fatal("spec is nil")
	}

	// CreateContainer 실행
	container, err := CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("fail to create container: %v", err)
	}

	// 추가 검증이 필요한 경우 container 에 대한 추가 검사를 여기에 추가할 수 있음
	t.Logf("Container created successfully: %+v", container)
}

func TestNewSetHealthChecker(t *testing.T) {
	tests := []struct {
		name        string
		inCmd       string
		interval    string
		retries     uint
		timeout     string
		startPeriod string
		expectErr   bool
		expected    *manifest.Schema2HealthConfig
	}{
		{
			name:        "Valid healthcheck with default settings",
			inCmd:       "CMD-SHELL /app/healthcheck.sh",
			interval:    "30s",
			retries:     3,
			timeout:     "5s",
			startPeriod: "0s",
			expectErr:   false,
			expected: &manifest.Schema2HealthConfig{
				Test:        []string{"CMD-SHELL", "/app/healthcheck.sh"},
				Interval:    30 * time.Second,
				Retries:     3,
				Timeout:     5 * time.Second,
				StartPeriod: 0,
			},
		},
		{
			name:        "Healthcheck with disabled interval",
			inCmd:       "CMD-SHELL /app/healthcheck.sh",
			interval:    "disable",
			retries:     2,
			timeout:     "10s",
			startPeriod: "5s",
			expectErr:   false,
			expected: &manifest.Schema2HealthConfig{
				Test:        []string{"CMD-SHELL", "/app/healthcheck.sh"},
				Interval:    0,
				Retries:     2,
				Timeout:     10 * time.Second,
				StartPeriod: 5 * time.Second,
			},
		},
		{
			name:        "Invalid command (missing CMD-SHELL)",
			inCmd:       "/app/healthcheck.sh",
			interval:    "30s",
			retries:     3,
			timeout:     "5s",
			startPeriod: "0s",
			expectErr:   true,
		},
		{
			name:        "Invalid interval format",
			inCmd:       "CMD-SHELL /app/healthcheck.sh",
			interval:    "abc",
			retries:     3,
			timeout:     "5s",
			startPeriod: "0s",
			expectErr:   true,
		},
		{
			name:        "Invalid timeout (less than 1 second)",
			inCmd:       "CMD-SHELL /app/healthcheck.sh",
			interval:    "30s",
			retries:     3,
			timeout:     "500ms",
			startPeriod: "0s",
			expectErr:   true,
		},
		{
			name:        "StartPeriod less than 0",
			inCmd:       "CMD-SHELL /app/healthcheck.sh",
			interval:    "30s",
			retries:     3,
			timeout:     "5s",
			startPeriod: "-1s",
			expectErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hc, err := setHealthChecker(test.inCmd, test.interval, test.retries, test.timeout, test.startPeriod)
			if (err != nil) != test.expectErr {
				t.Errorf("expected error: %v, got: %v", test.expectErr, err)
				return
			}

			if err == nil && !reflect.DeepEqual(hc, test.expected) {
				t.Errorf("expected: %+v, got: %+v", test.expected, hc)
			}
		})
	}
}

// TODO: running-container 만들어주고 테스트 해야 함
func TestHandleExistingContainer_RunningContainer(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	containerName := "running-container" // 실제 실행 중인 컨테이너

	// handleExistingContainer 호출
	result, err := handleExistingContainer(ctx, containerName)
	if err != nil {
		t.Fatalf("Error while handling running container: %v", err)
	}

	if result.Status != Running {
		t.Errorf("Expected status Running, got %v", result.Status)
	}

	t.Logf("Container %s is running, ID: %s", result.Name, result.ID)
}

// TODO: stopped-container 만들어주고 테스트 해야 함
func TestHandleExistingContainer_CreatedContainer(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	containerName := "stopped-container" // 실제 정지된 컨테이너

	// handleExistingContainer 호출
	result, err := handleExistingContainer(ctx, containerName)
	if err != nil {
		t.Fatalf("Error while handling stopped container: %v", err)
	}

	if result.Status != Created {
		t.Errorf("Expected status Created, got %v", result.Status)
	}

	t.Logf("Container %s is created but stopped, ID: %s", result.Name, result.ID)
}

// TODO: already-running-container 만들어주고 테스트 해야 함

// 존재하지 않는 컨테이너
func TestHandleExistingContainer_NonExistentContainer(t *testing.T) {

	ctx, err := NewConnectionLinux5(context.Background())
	containerName := "non-existent-container" // 존재하지 않는 컨테이너

	_, err = handleExistingContainer(ctx, containerName)
	if err == nil {
		t.Fatalf("Expected error for non-existent container, but got none")
	}

	t.Logf("Expected error for non-existent container: %v", err)
}
