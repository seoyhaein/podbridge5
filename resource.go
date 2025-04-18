package podbridge5

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	"os"
	"strconv"
	"time"
)

// LogContainerStatsToCSV 지정한 컨테이너의 리소스 사용량(CPU, Memory 등)을 일정 간격으로 CSV 파일에 기록
func LogContainerStatsToCSV(ctx context.Context, containerID string, intervalSec int) error {
	csvPath := containerID + "_stats.csv"

	// 1) 기존 파일이 있으면 삭제
	if _, err := os.Stat(csvPath); err == nil {
		// 파일이 존재한다면
		if err := os.Remove(csvPath); err != nil {
			return fmt.Errorf("failed to remove existing CSV file %q: %w", csvPath, err)
		}
	} else if !os.IsNotExist(err) {
		// Stat 자체가 에러인데 NotExist 가 아니라면
		return fmt.Errorf("failed to stat CSV file %q: %w", csvPath, err)
	}

	// 2) 새로 파일 생성 (쓰기 전용)
	file, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create CSV file %q: %w", csvPath, err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil {
			Log.Infof("Failed to close file: %v", cErr)
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 3) 헤더 기록
	header := []string{"Timestamp", "CPUNano", "MemUsage"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV header: %w", err)
	}

	// 4) Stats 스트리밍 구독 & 기록 (기존 로직 유지)
	statsOpts := new(containers.StatsOptions).
		WithAll(false).
		WithStream(true).
		WithInterval(intervalSec)
	statsCh, err := containers.Stats(ctx, []string{containerID}, statsOpts)
	if err != nil {
		return fmt.Errorf("failed to subscribe stats for container %s: %w", containerID, err)
	}

	for report := range statsCh {
		if report.Error != nil {
			if errors.Is(report.Error, context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("stats report error: %w", report.Error)
		}
		if len(report.Stats) == 0 {
			Log.Warnf("stats data is empty")
			continue
		}
		cs := report.Stats[0]
		rec := []string{
			time.Now().Format(time.RFC3339),
			strconv.FormatUint(cs.CPUNano, 10),
			strconv.FormatUint(cs.MemUsage, 10),
		}
		if err := writer.Write(rec); err != nil {
			Log.Warnf("failed to write CSV record: %v", err)
		}
		writer.Flush()
	}

	return nil
}

// WithCPULimits 설정은 컨테이너의 CPU 관련 제한을 구성
// cpuQuota: 한 주기 동안 사용할 수 있는 최대 CPU 시간(마이크로초 단위)
// cpuPeriod: CPU 제한 주기(마이크로초 단위)
// cpuShares: 컨테이너의 상대적인 CPU 가중치
// WithCPULimits(50000, 100000, 1024),  // 예: --cpu-quota=50000, --cpu-period=100000, --cpu-shares=1024 일 경우 cpu 50% 사용.
// WithCPULimits(100000, 100000, 1024),  // 예: --cpu-quota=100000, --cpu-period=100000, --cpu-shares=1024 일 경우 1 core 사용.
// WithCPULimits(200000, 100000, 1024),  // 예: --cpu-quota=200000, --cpu-period=100000, --cpu-shares=1024 일 경우 2 core 사용.

// CPU quota 와 period 를 이용해 200% (예: cpu-quota=200000, cpu-period=100000)로 설정하면,
// 컨테이너는 2코어에 해당하는 CPU 시간을 사용할 수 있도록 허용됨.
// 다만,
// 이 경우 컨테이너는 특정 코어에 "고정(pinning)"되는 것이 아니라,
// 시스템의 가용 코어 전체에서 2코어 분량의 CPU 시간을 할당받게 됩.
// 이 내용은 확인해봐야 함.

func WithCPULimits(cpuQuota int64, cpuPeriod uint64, cpuShares uint64) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		if spec.ResourceLimits == nil {
			spec.ResourceLimits = &specs.LinuxResources{}
		}
		if spec.ResourceLimits.CPU == nil {
			spec.ResourceLimits.CPU = &specs.LinuxCPU{}
		}
		spec.ResourceLimits.CPU.Quota = &cpuQuota
		spec.ResourceLimits.CPU.Period = &cpuPeriod
		spec.ResourceLimits.CPU.Shares = &cpuShares
		return nil
	}
}

// WithNanoCPUs cgroup v2 호환 방식으로 NanoCPUs를 설정
// nanoCPUs: 1 CPU = 1_000_000_000 (1e9), 0.05 CPU = 50_000_000, 등
func WithNanoCPUs(nanoCPUs int64) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		// 1) ContainerResourceConfig.ResourceLimits 초기화
		if spec.ResourceLimits == nil {
			spec.ResourceLimits = &specs.LinuxResources{}
		}
		if spec.ResourceLimits.CPU == nil {
			spec.ResourceLimits.CPU = &specs.LinuxCPU{}
		}

		// 2) cgroup v1/v2 모두 기본 period 100_000µs 사용
		const defaultPeriod = 100_000

		// 3) nanoCPUs → quota(us) 계산: nanoCPUs × period / 1e9
		quota := int64(nanoCPUs * int64(defaultPeriod) / 1_000_000_000)

		// 4) 설정
		spec.ResourceLimits.CPU.Period = ptrUint64(defaultPeriod)
		spec.ResourceLimits.CPU.Quota = &quota

		return nil
	}
}

// helper functions to take address of primitives
func ptrInt64(v int64) *int64    { return &v }
func ptrUint64(v uint64) *uint64 { return &v }
func ptrInt(v int) *int          { return &v }

// WithMemoryLimit 설정은 컨테이너의 메모리 제한을 구성
// memLimit: 메모리 제한 값(바이트 단위)
func WithMemoryLimit(memLimit int64) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		if spec.ResourceLimits == nil {
			spec.ResourceLimits = &specs.LinuxResources{}
		}
		if spec.ResourceLimits.Memory == nil {
			spec.ResourceLimits.Memory = &specs.LinuxMemory{}
		}
		spec.ResourceLimits.Memory.Limit = &memLimit
		return nil
	}
}

// WithOOMScoreAdj 설정은 OOM(Out-Of-Memory) 상황 시 커널의 OOM 킬러가
// 컨테이너를 종료할 우선순위를 조정
// 음수 값은 보호 효과(더 늦게 죽임), 양수 값은 먼저 죽일 가능성이 높음
func WithOOMScoreAdj(oomScore int) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		spec.OOMScoreAdj = &oomScore
		return nil
	}
}

/*
spec, err := NewSpec(
	WithImageName("ubuntu:latest"),
	WithName("sample-container"),
	WithTerminal(true),
	WithCPULimits(50000, 100000, 1024),  // 예: --cpu-quota=50000, --cpu-period=100000, --cpu-shares=1024
	WithMemoryLimit(2 * 1024 * 1024 * 1024), // 예: --memory=2g (2GB)
	WithOOMScoreAdj(-500),               // OOM 상황에서 보호 효과 적용
)
*/

// RunContainerWithStats TODO 수정 많이 해야함. 그냥 spec 을 입력 값으로 받는 것을 만드는게 낳을 듯. 삭제 예정.
func RunContainerWithStats(
	internalImageName, containerName string,
	tty, logStats bool,
	cpuQuota int64, cpuPeriod uint64, cpuShares uint64,
	memoryLimit int64, oomScore int,
) (string, error) {
	// pbCtx 는 전역 context
	if pbCtx == nil {
		return "", errors.New("pbCtx is nil")
	}

	// Spec 생성: 이미지, 이름, 터미널, HealthChecker 옵션과 함께 리소스 제한 옵션들을 입력 파라미터로 설정
	spec, err := NewSpec(
		WithImageName(internalImageName),
		WithName(containerName),
		WithTerminal(tty),
		WithHealthChecker("CMD-SHELL bash /app/healthcheck.sh", "1s", 1, "30s", "0s"),
		WithCPULimits(cpuQuota, cpuPeriod, cpuShares), // 입력받은 cpuQuota, cpuPeriod, cpuShares 사용
		WithMemoryLimit(memoryLimit),                  // 입력받은 memoryLimit 사용
		WithOOMScoreAdj(oomScore),                     // 입력받은 oomScore 사용
	)
	if err != nil {
		Log.Errorf("failed to create spec: %v", err)
		return "", fmt.Errorf("failed to create spec: %w", err)
	}

	// 컨테이너 생성
	ccr, err := CreateContainer(pbCtx, spec)
	if err != nil {
		Log.Errorf("failed to create container: %v", err)
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 컨테이너 시작
	if err := containers.Start(pbCtx, ccr.ID, &containers.StartOptions{}); err != nil {
		Log.Errorf("failed to start container: %v", err)
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	// logStats 가 true 이면, 별도의 고루틴에서 stats 데이터를 CSV 파일에 기록
	if logStats {
		go func(id string) {
			// 예시로, 10초 간격으로 stats 데이터를 containerID 기반 CSV 파일에 기록
			if err := LogContainerStatsToCSV(pbCtx, id, 5); err != nil {
				Log.Errorf("failed to log stats for container %s: %v", id, err)
			}
		}(ccr.ID)
	}

	return ccr.ID, nil
}
