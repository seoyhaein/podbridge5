package podbridge5

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/opencontainers/runtime-spec/specs-go"
	"log"
	"os"
	"strconv"
	"time"
)

// TODO 테스트 아직 안해봄.

// LogContainerStatsToCSV 지정한 컨테이너의 리소스 사용량(CPU, Memory 등)을 일정 간격으로 CSV 파일에 기록
// interval 은 일단 5 sec 으로 해준다. 밑에 그렇게 적용했음. TODO 하지만 이 값도 측정해보고 결정하자.
func LogContainerStatsToCSV(ctx context.Context, containerID string, interval int) error {
	// CSV 파일 이름을 containerID를 기반으로 생성 (예: "<containerID>_stats.csv")
	csvFilePath := containerID + "_stats.csv"

	// CSV 파일을 열거나 없으면 생성, 이어쓰기 모드로 엽니다.
	file, err := os.OpenFile(csvFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		cErr := file.Close()
		if cErr != nil {
			Log.Infof("Failed to close file: %v", cErr)
		}
	}()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 헤더 기록 (이미 기록되어 있다면 중복 기록 방지 로직 추가 가능)
	header := []string{"Timestamp", "CPUNano", "MemUsage"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// 컨테이너 ID를 슬라이스로 설정
	containerIDs := []string{containerID}

	// Stats 옵션 설정:
	// - WithAll(false): 지정한 컨테이너에 대해서만 통계 수집
	// - WithStream(true): 스트리밍 모드 활성화 (API가 주기적으로 stats 데이터를 채널로 전송)
	// - WithInterval(5): 5초 간격으로 데이터를 전송
	statsOpts := new(containers.StatsOptions).WithAll(false).WithStream(true).WithInterval(interval)

	// 컨테이너의 stats 데이터를 스트리밍 채널로 받음.
	statsChan, err := containers.Stats(ctx, containerIDs, statsOpts)
	if err != nil {
		return fmt.Errorf("컨테이너 %s stats 조회 실패: %w", containerID, err)
	}

	// 스트리밍 채널에서 stats 데이터를 수신하며 CSV 파일에 기록
	//	참고
	//	type ContainerStatsReport struct {
	//		// Error from reading stats.
	//		Error error
	//		// Results, set when there is no error.
	//		Stats []define.ContainerStats
	//	}
	// 여기서 Stats []define.ContainerStats 인 이유는 containers.Stats 이 메서드 자체가 여러 컨테이너에 대해서 stat 데이터를 가져올 수 있도록 만들어졌기 때문.
	for statReport := range statsChan {
		// 만약 리포트에 에러가 있다면 로그로 남기고 건너뜁니다.
		if statReport.Error != nil {
			log.Printf("stats 리포트 에러: %v", statReport.Error)
			continue
		}

		// stats 슬라이스가 비어 있다면 건너뜁니다.
		if len(statReport.Stats) == 0 {
			log.Printf("stats 데이터가 비어 있습니다.")
			continue
		}

		// 참고 여기서는 확인할 컨테이너의 수가 하나임으로 첫번째 값을 선택함.
		cs := statReport.Stats[0]
		// 예시: CPUNano 와 MemUsage 를 문자열로 변환하여 기록합니다.
		cpuUsageStr := strconv.FormatUint(cs.CPUNano, 10)
		memoryUsageStr := strconv.FormatUint(cs.MemUsage, 10)

		// 현재 시간을 타임스탬프로 사용합니다.
		t := time.Now()

		record := []string{
			t.Format(time.RFC3339),
			cpuUsageStr,
			memoryUsageStr,
		}

		if err := writer.Write(record); err != nil {
			log.Printf("CSV 기록 실패: %v", err)
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

// RunContainerWithStats TODO 수정 많이 해야함. 그냥 spec 을 입력 값으로 받는 것을 만드는게 낳을 듯.
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
