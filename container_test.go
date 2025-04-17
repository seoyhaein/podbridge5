package podbridge5

import (
	"context"
	"fmt"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/google/uuid"
	"reflect"
	"testing"
	"time"
)

func TestCreateContainerIntegration(t *testing.T) {
	// 1) Podman 클라이언트 컨텍스트 생성
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("failed to connect to podman: %v", err)
	}

	// 2) 유니크한 컨테이너 이름 생성
	contName := fmt.Sprintf("test-create-%s", uuid.New().String())

	// 3) SpecGenerator 준비
	spec := specgen.NewSpecGenerator("docker.io/library/alpine:latest", false)

	spec.Name = contName
	// 간단히 sleep infinity 로 띄워두기
	spec.Command = []string{"sleep", "infinity"}

	// 4) CreateContainer 호출
	result, err := CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	// 생성된 ID 확인
	if result.ID == "" {
		t.Fatal("expected non-empty container ID")
	}

	// 5) 컨테이너 존재 여부 검사
	exists, err := containers.Exists(ctx, contName, &containers.ExistsOptions{})
	if err != nil {
		t.Fatalf("containers.Exists error: %v", err)
	}
	if !exists {
		t.Fatalf("container %q should exist after creation", contName)
	}

	// 6) 정리: 컨테이너 중지 및 삭제
	// t.Cleanup 으로 해 두면, 테스트가 끝날 때 자동으로 실행
	t.Cleanup(func() {
		// Stop: 타임아웃 10초, 없으면 무시
		ignore := true
		timeout := uint(10)
		stopOpts := &containers.StopOptions{
			Ignore:  &ignore,
			Timeout: &timeout,
		}
		if err := containers.Stop(ctx, result.ID, stopOpts); err != nil {
			t.Logf("cleanup: stop error (non-fatal): %v", err)
		}

		// Remove: 강제 삭제
		removeForce := true
		removeVolumes := false
		removeOpts := &containers.RemoveOptions{
			Force:   &removeForce,
			Volumes: &removeVolumes,
		}
		if _, err := containers.Remove(ctx, result.ID, removeOpts); err != nil {
			t.Logf("cleanup: remove error (non-fatal): %v", err)
		}
	})

	// 7) (옵션) 이미 존재하는 이름으로 CreateContainer 를 한 번 더 부르면
	// handleExistingContainer 로직이 호출되어 동일 ID를 반환해야 합
	result2, err := CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("CreateContainer on existing name failed: %v", err)
	}
	if result2.ID != result.ID {
		t.Errorf("expected same ID on reuse, got %q vs %q", result.ID, result2.ID)
	}
}

func TestCreateContainer(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("failed to connect to podman: %v", err)
	}

	// 유니크한 이름으로 충돌 방지
	name := "test-" + uuid.New().String()

	spec, err := NewSpec(
		WithImageName("docker.io/library/busybox:latest"),
		WithName(name),
		WithTerminal(true),
	)
	if err != nil {
		t.Fatalf("failed to build spec: %v", err)
	}

	// 테스트 종료 시 컨테이너 정리
	t.Cleanup(func() {
		ignore := true
		timeout := uint(5)
		_ = containers.Stop(ctx, name, &containers.StopOptions{
			Ignore:  &ignore,
			Timeout: &timeout,
		})
		force := true
		vols := false
		_, _ = containers.Remove(ctx, name, &containers.RemoveOptions{
			Force:   &force,
			Volumes: &vols,
			Ignore:  &ignore,
		})
	})

	res, err := CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("CreateContainer failed: %v", err)
	}
	if res.ID == "" {
		t.Fatal("expected container ID, got empty")
	}
	if res.Name != name {
		t.Errorf("expected name %q, got %q", name, res.Name)
	}

	// 실제로 존재하는지 확인
	exists, err := containers.Exists(ctx, name, nil)
	if err != nil {
		t.Fatalf("failed to check existence: %v", err)
	}
	if !exists {
		t.Fatalf("container %q should exist", name)
	}

	t.Logf("created container %s (%s)", res.Name, res.ID)
}

func TestSetHealthChecker(t *testing.T) {
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
		{
			name:        "Invalid retries (zero)",
			inCmd:       "CMD-SHELL /app/healthcheck.sh",
			interval:    "30s",
			retries:     0,
			timeout:     "5s",
			startPeriod: "0s",
			expectErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := setHealthChecker(tc.inCmd, tc.interval, tc.retries, tc.timeout, tc.startPeriod)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("mismatch:\n expected: %+v\n      got: %+v", tc.expected, got)
			}
		})
	}
}

func TestHandleExistingContainer_RunningContainer(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	// 1) 임시 컨테이너 생성·시작
	name := "test-existing-" + uuid.New().String()
	spec, err := NewSpec(
		WithImageName("docker.io/library/busybox:latest"),
		WithName(name),
		WithTerminal(false),
		WithCommand([]string{"sleep", "60"}),
	)
	if err != nil {
		t.Fatalf("failed to build spec: %v", err)
	}
	res, err := CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	id := res.ID
	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start container %s: %v", id, err)
	}

	// 2) 테스트 종료 시 정리
	t.Cleanup(func() {
		ignore := true
		timeout := uint(5)
		_ = containers.Stop(ctx, name, &containers.StopOptions{
			Ignore:  &ignore,
			Timeout: &timeout,
		})

		force := true
		vols := true
		// 여기만 _, _ = 로 받도록 변경
		_, _ = containers.Remove(ctx, name, &containers.RemoveOptions{
			Force:   &force,
			Volumes: &vols,
			Ignore:  &ignore,
		})
	})

	// 3) handleExistingContainer 호출 및 검증
	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer returned error: %v", err)
	}
	if got.Status != Running {
		t.Errorf("expected status Running, got %v", got.Status)
	}
	if got.ID != id {
		t.Errorf("expected ID %s, got %s", id, got.ID)
	}
	if got.Name != name {
		t.Errorf("expected Name %s, got %s", name, got.Name)
	}
}

func TestHandleExistingContainer_ExitedContainer(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	// 1) 유니크한 이름으로 컨테이너 생성(커맨드 true → 즉시 Exit 0)
	name := "test-exited-" + uuid.New().String()
	spec, err := NewSpec(
		WithImageName("docker.io/library/busybox:latest"),
		WithName(name),
		WithTerminal(false),
		WithCommand([]string{"true"}),
	)
	if err != nil {
		t.Fatalf("failed to build spec: %v", err)
	}
	res, err := CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	id := res.ID

	// 2) Cleanup: 필요 시 강제 삭제만
	t.Cleanup(func() {
		ignore := true
		force := true
		vols := true
		_, _ = containers.Remove(ctx, id, &containers.RemoveOptions{
			Force:   &force,
			Volumes: &vols,
			Ignore:  &ignore,
		})
	})

	// 3) Start → 바로 exit(0)
	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start container %s: %v", id, err)
	}
	// 잠시 대기해서 exit 시키기
	time.Sleep(500 * time.Millisecond)

	// 4) handleExistingContainer 호출
	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer returned error: %v", err)
	}

	// 5) 검증: ExitCode == 0 이므로 Exited
	if got.Status != Exited {
		t.Errorf("expected status Exited for normal exit, got %v", got.Status)
	}
	if got.ID != id {
		t.Errorf("expected ID %q, got %q", id, got.ID)
	}
	if got.Name != name {
		t.Errorf("expected Name %q, got %q", name, got.Name)
	}
}

// 존재하지 않는 컨테이너
func TestHandleExistingContainer_NonExistentContainer(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	// 랜덤한 이름으로 존재하지 않는 컨테이너 보장
	name := "test-nonexistent-" + uuid.New().String()
	// 호출 시 에러가 반환되어야 함
	_, err = handleExistingContainer(ctx, name)
	if err == nil {
		t.Fatalf("expected error for non-existent container %q, got none", name)
	}
	t.Logf("handleExistingContainer correctly failed for %q: %v", name, err)
}

func createTestContainer(t *testing.T, ctx context.Context, cmd []string) (name, id string) {
	name = "test-" + uuid.New().String()
	spec, err := NewSpec(
		WithImageName("docker.io/library/busybox:latest"),
		WithName(name),
		WithTerminal(false),
		WithCommand(cmd),
	)
	if err != nil {
		t.Fatalf("failed to build spec for %s: %v", name, err)
	}
	res, err := CreateContainer(ctx, spec)
	if err != nil {
		t.Fatalf("failed to create container %s: %v", name, err)
	}
	id = res.ID
	return
}

func cleanupContainer(t *testing.T, ctx context.Context, id string) {
	ignore := true
	force := true
	vols := true
	timeout := uint(5)
	_ = containers.Stop(ctx, id, &containers.StopOptions{Ignore: &ignore, Timeout: &timeout})
	_, _ = containers.Remove(ctx, id, &containers.RemoveOptions{
		Force:   &force,
		Volumes: &vols,
		Ignore:  &ignore,
	})
}

func TestHandleExistingContainer_Created(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	name, id := createTestContainer(t, ctx, []string{"true"})
	// do not start: status should be Created
	t.Cleanup(func() { cleanupContainer(t, ctx, id) })

	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer error: %v", err)
	}
	if got.Status != Created {
		t.Errorf("expected Created, got %v", got.Status)
	}
	if got.ID != id || got.Name != name {
		t.Errorf("expected Name/ID %s/%s, got %s/%s", name, id, got.Name, got.ID)
	}
}

func TestHandleExistingContainer_Running(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	name, id := createTestContainer(t, ctx, []string{"sleep", "60"})
	t.Cleanup(func() { cleanupContainer(t, ctx, id) })
	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start %s: %v", id, err)
	}

	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer error: %v", err)
	}
	if got.Status != Running {
		t.Errorf("expected Running, got %v", got.Status)
	}
	if got.ID != id || got.Name != name {
		t.Errorf("expected Name/ID %s/%s, got %s/%s", name, id, got.Name, got.ID)
	}
}

func TestHandleExistingContainer_Exited(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	name, id := createTestContainer(t, ctx, []string{"true"})
	t.Cleanup(func() { cleanupContainer(t, ctx, id) })
	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start %s: %v", id, err)
	}
	// wait for exit
	time.Sleep(200 * time.Millisecond)

	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer error: %v", err)
	}
	if got.Status != Exited {
		t.Errorf("expected Exited, got %v", got.Status)
	}
}

func TestHandleExistingContainer_ExitedErr(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	name, id := createTestContainer(t, ctx, []string{"false"})
	t.Cleanup(func() { cleanupContainer(t, ctx, id) })
	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start %s: %v", id, err)
	}
	time.Sleep(200 * time.Millisecond)

	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer error: %v", err)
	}
	if got.Status != ExitedErr {
		t.Errorf("expected ExitedErr, got %v", got.Status)
	}
}

func TestHandleExistingContainer_Paused(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	name, id := createTestContainer(t, ctx, []string{"sleep", "60"})
	t.Cleanup(func() { cleanupContainer(t, ctx, id) })
	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start %s: %v", id, err)
	}
	if err := containers.Pause(ctx, id, nil); err != nil {
		t.Fatalf("failed to pause %s: %v", id, err)
	}

	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer error: %v", err)
	}
	if got.Status != Paused {
		t.Errorf("expected Paused, got %v", got.Status)
	}
}

func TestHandleExistingContainer_Killed(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	name, id := createTestContainer(t, ctx, []string{"sleep", "60"})
	t.Cleanup(func() { cleanupContainer(t, ctx, id) })

	if err := containers.Start(ctx, id, nil); err != nil {
		t.Fatalf("failed to start %s: %v", id, err)
	}

	// force kill
	sig := "SIGKILL"
	if err := containers.Kill(ctx, id, &containers.KillOptions{
		Signal: &sig,
	}); err != nil {
		t.Fatalf("failed to kill %s: %v", id, err)
	}

	got, err := handleExistingContainer(ctx, name)
	if err != nil {
		t.Fatalf("handleExistingContainer error: %v", err)
	}
	if got.Status != ExitedErr {
		t.Errorf("expected ExitedErr for killed container, got %v", got.Status)
	}
}

func TestHandleExistingContainer_NonExistent(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	name := "test-nonexistent-" + uuid.New().String()
	_, err = handleExistingContainer(ctx, name)
	if err == nil {
		t.Fatalf("expected error for non-existent container %q, got none", name)
	}
}
