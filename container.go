package podbridge5

import (
	"context"
	"errors"
	"fmt"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/seoyhaein/utils"
	"strings"
	"time"
)

type ContainerStatus int

const (
	Created   ContainerStatus = iota //0
	Running                          // 1
	Exited                           // 2
	ExitedErr                        // 3
	Healthy                          // 4
	Unhealthy                        // 5
	Dead                             // 6
	Paused                           // 7
	UnKnown                          // 8
	None                             // 9
)

type ContainerOptions func(spec *specgen.SpecGenerator) error

// CreateContainerResult 컨테이너 생성 정보를 담는 구조체
type (
	CreateContainerResult struct {
		Name     string
		ID       string
		Warnings []string
		Status   ContainerStatus
	}
)

// NewSpec creates a new SpecGenerator.
func NewSpec(opts ...ContainerOptions) (*specgen.SpecGenerator, error) {
	spec := &specgen.SpecGenerator{}
	for _, opt := range opts {
		if err := opt(spec); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

func WithImageName(imgName string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		spec.Image = imgName
		return nil
	}
}

func WithName(name string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		spec.Name = name
		return nil
	}
}

func WithTerminal(terminal bool) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		spec.Terminal = &terminal
		return nil
	}
}

func WithCommand(cmd []string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		spec.Command = cmd
		return nil
	}
}

// WithHealthChecker healthcheck 설정에 문제가 발생하면 에러를 반환
func WithHealthChecker(inCmd, interval string, retries uint, timeout, startPeriod string) ContainerOptions {
	// 한 번만 파싱/검증
	hc, err := setHealthChecker(inCmd, interval, retries, timeout, startPeriod)
	return func(spec *specgen.SpecGenerator) error {
		if err != nil {
			// 옵션 생성 시점에 실패 원인을 그대로 반환
			return fmt.Errorf("invalid healthcheck config: %w", err)
		}
		spec.HealthConfig = hc
		return nil
	}
}

// StartContainer 컨테이너를 만들고 시작함.
func StartContainer(ctx context.Context, spec *specgen.SpecGenerator) (string, error) {
	if ctx == nil {
		return "", errors.New("context is nil")
	}
	if spec == nil {
		return "", errors.New("spec is nil")
	}

	ccr, err := CreateContainer(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	if err := containers.Start(ctx, ccr.ID, &containers.StartOptions{}); err != nil {
		return "", fmt.Errorf("start container: %w", err)
	}

	return ccr.ID, nil
}

// CreateContainer 컨테이너 생성
func CreateContainer(ctx context.Context, conSpec *specgen.SpecGenerator) (*CreateContainerResult, error) {
	if err := conSpec.Validate(); err != nil {
		Log.Errorf("validation failed: %v", err)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	if utils.IsEmptyString(conSpec.Name) || utils.IsEmptyString(conSpec.Image) {
		Log.Error("Container's name or image's name is not set")
		return nil, errors.New("container name or image's name is not set")
	}

	// 컨테이너가 local storage 에 존재하는지 확인
	containerExists, err := containers.Exists(ctx, conSpec.Name, &containers.ExistsOptions{External: utils.PFalse})
	if err != nil {
		Log.Errorf("Failed to check if container exists: %v", err)
		return nil, fmt.Errorf("failed to check if container exists: %w", err)
	}

	if containerExists {
		return handleExistingContainer(ctx, conSpec.Name)
	}

	// 이미지가 존재하는지 확인
	imageExists, err := images.Exists(ctx, conSpec.Image, nil)
	if err != nil {
		Log.Errorf("Failed to check if image exists: %v", err)
		return nil, fmt.Errorf("failed to check if image exists: %w", err)
	}

	if !imageExists {
		Log.Infof("Pulling %s image...", conSpec.Image)
		if _, err := images.Pull(ctx, conSpec.Image, &images.PullOptions{}); err != nil {
			Log.Errorf("Failed to pull image: %v", err)
			return nil, fmt.Errorf("failed to pull image: %w", err)
		}
	}

	Log.Infof("Creating %s container using %s image...", conSpec.Name, conSpec.Image)
	createResponse, err := containers.CreateWithSpec(ctx, conSpec, &containers.CreateOptions{})
	if err != nil {
		Log.Errorf("Failed to create container: %v", err)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	return &CreateContainerResult{
		Name:     conSpec.Name,
		ID:       createResponse.ID,
		Warnings: createResponse.Warnings,
		Status:   Created,
	}, nil
}

func InspectContainer(containerId string) (*define.InspectContainerData, error) {
	// pbCtx 는 전역 context 임.
	if pbCtx == nil {
		return nil, errors.New("pbCtx is nil")
	}

	var containerInspectOptions containers.InspectOptions
	containerInspectOptions.Size = utils.PFalse
	containerData, err := containers.Inspect(pbCtx, containerId, &containerInspectOptions)

	return containerData, err
}

// HealthCheckContainer TODO 추후 테스트 필요. 확인 필요.
func HealthCheckContainer(containerId string) (status *string, exitCode *int, err error) {
	// 컨테이너 데이터 조회
	containerData, err := InspectContainer(containerId)
	if err != nil {
		// 오류 발생 시 상태와 종료 코드를 nil 로 반환
		return nil, nil, err
	}

	// 상태 값이 비어 있는지 확인
	containerStatus := containerData.State.Status
	if utils.IsEmptyString(containerStatus) {
		err = fmt.Errorf("container state status is empty")
		return
	}
	status = &containerStatus

	// Health 가 nil 이면, 종료 코드 없이 상태만 반환
	if containerData.State.Health == nil {
		return
	}

	// Health 로그를 확인하여 ExitCode 가 0이 아닌 첫 번째 로그 반환
	for _, log := range containerData.State.Health.Log {
		if log.ExitCode != 0 {
			exitCode = &log.ExitCode
			return
		}
	}

	// 모든 로그가 정상일 경우 0 종료 코드 반환
	defaultExitCode := 0
	exitCode = &defaultExitCode
	return
}

// handleExistingContainer 컨테이너가 존재했을 경우 해당 컨테이너의 정보를 리턴함.
func handleExistingContainer(ctx context.Context, containerName string) (*CreateContainerResult, error) {
	info, err := containers.Inspect(ctx, containerName, &containers.InspectOptions{Size: utils.PFalse})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %q: %w", containerName, err)
	}

	s := info.State
	var status ContainerStatus

	switch {
	case s.Running:
		status = Running
	case s.Paused:
		status = Paused
	case s.Dead:
		status = Dead
	case s.ExitCode >= 0:
		// 프로세스가 종료된 상태
		if s.ExitCode == 0 {
			status = Exited
		} else {
			status = ExitedErr
		}
	default:
		// 생성만 되고 아직 시작되지 않은 상태
		status = Created
	}

	return &CreateContainerResult{
		Name:   containerName,
		ID:     info.ID,
		Status: status,
	}, nil
}

func setHealthChecker(inCmd, interval string, retries uint, timeout, startPeriod string) (*manifest.Schema2HealthConfig, error) {
	// inCmd 는 항상 "CMD-SHELL /app/healthcheck.sh" 형식으로만 들어온다고 가정
	cmdArr := strings.Fields(inCmd) // 공백을 기준으로 명령어를 분리

	// 명령어가 "CMD-SHELL"로 시작하는지 확인
	if len(cmdArr) < 2 || cmdArr[0] != "CMD-SHELL" {
		return nil, errors.New("invalid command format: must start with CMD-SHELL")
	}

	// healthcheck 는 Test 필드가 명령어 배열로 되어 있어야 함
	hc := manifest.Schema2HealthConfig{
		Test: cmdArr,
	}

	// Interval 설정 (disable 로 설정되면 0으로 처리)
	if interval == "disable" {
		interval = "0"
	}
	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid healthcheck-interval: %w", err)
	}
	hc.Interval = intervalDuration

	// Retries 는 1 이상이어야 함
	if retries < 1 {
		return nil, errors.New("healthcheck-retries must be greater than 0")
	}
	hc.Retries = int(retries)

	// Timeout 설정 (최소 1초 이상이어야 함)
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid healthcheck-timeout: %w", err)
	}
	if timeoutDuration < time.Second {
		return nil, errors.New("healthcheck-timeout must be at least 1 second")
	}
	hc.Timeout = timeoutDuration

	// StartPeriod 설정 (0초 이상이어야 함)
	startPeriodDuration, err := time.ParseDuration(startPeriod)
	if err != nil {
		return nil, fmt.Errorf("invalid healthcheck-start-period: %w", err)
	}
	if startPeriodDuration < 0 {
		return nil, errors.New("healthcheck-start-period must be 0 seconds or greater")
	}
	hc.StartPeriod = startPeriodDuration

	return &hc, nil
}
