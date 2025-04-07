package podbridge5

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containers/buildah"
	is "github.com/containers/image/v5/storage"
	imageTypes "github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/seoyhaein/utils"
	"os"
)

// BuildConfig holds basic configuration for building an image,
type BuildConfig struct {
	Image     ImageConfig     `json:"image"`
	Container ContainerConfig `json:"container"`
}

/*
type BuildConfig struct {
	SourceImageName  string             `json:"sourceImageName"`  // 기본 베이스 이미지 (예: "docker.io/library/alpine:latest")
	ImageName        string             `json:"imageName"`        // 최종 이미지 이름 (예: "tester")
	ImageSavePath    string             `json:"imageSavePath"`    // 이미지를 저장할 경로 (예: "/opt/images")
	ExecutorShell    string             `json:"executorShell"`    // executor 스크립트 경로 (예: "./executor.sh")
	DockerfilePath   string             `json:"dockerfilePath"`   // Dockerfile 경로 (예: "./Dockerfile")
	HealthcheckShell string             `json:"healthcheckShell"` // healthcheck 스크립트 경로 (예: "./healthcheck.sh")
	InstallShell     string             `json:"installShell"`     // install 스크립트 경로 (예: "./install.sh")
	UserScriptShell  string             `json:"userScriptShell"`  // user script 스크립트 경로 (예: "./scripts/user_script.sh")
	BuildSettings    ImageBuildSettings `json:"buildSettings"`    // 이미지 빌드에 사용되는 추가 설정들
}
*/

// ImageConfig 이미지 빌드에 필요한 필드들
type ImageConfig struct {
	SourceImageName string              `json:"sourceImageName"` // 예: "docker.io/library/ubuntu:latest"
	ImageName       string              `json:"imageName"`       // 최종 생성될 이미지 이름
	ImageSavePath   string              `json:"imageSavePath"`   // 이미지 저장 경로
	DockerfilePath  string              `json:"dockerfilePath"`  // Dockerfile 경로
	Directories     []string            `json:"directories"`     // 빌드 과정 중 컨테이너 내부에서 생성할 디렉토리 목록
	ScriptMap       map[string][]string `json:"scriptMap"`       // 각 디렉토리에 복사할 스크립트 파일 목록
	PermissionFiles []string            `json:"permissionFiles"` // 파일 권한 설정이 필요한 파일 목록 (최종 경로 기준)
	WorkDir         string              `json:"workDir"`         // 빌드 시 컨테이너의 작업 디렉토리
	CMD             []string            `json:"cmd"`             // 빌드 완료 후 컨테이너 시작 시 실행할 명령어
}

/*
type ImageConfig struct {
	SourceImageName string `json:"sourceImageName"` // 예: "docker.io/library/ubuntu:latest"
	ImageName       string `json:"imageName"`       // 최종 생성될 이미지 이름
	ImageSavePath   string `json:"imageSavePath"`   // 이미지 저장 경로
	DockerfilePath  string `json:"dockerfilePath"`  // Dockerfile 경로
}
*/

// ContainerConfig 컨테이너 실행에 필요한 설정들을 포함
type ContainerConfig struct {
	ExecutorShell    string              `json:"executorShell"`    // 예: "./executor.sh"
	HealthcheckShell string              `json:"healthcheckShell"` // 예: "./healthcheck.sh"
	InstallShell     string              `json:"installShell"`     // 예: "./install.sh"
	UserScriptShell  string              `json:"userScriptShell"`  // 예: "./scripts/user_script.sh"
	Directories      []string            `json:"directories"`      // 컨테이너 실행 시 내부에서 미리 생성할 디렉토리 목록
	ScriptMap        map[string][]string `json:"scriptMap"`        // 각 디렉토리에 복사할 스크립트 파일 목록
	PermissionFiles  []string            `json:"permissionFiles"`  // 파일 권한 설정이 필요한 파일 목록 (최종 경로 기준)
	WorkDir          string              `json:"workDir"`          // 컨테이너의 작업 디렉토리
	Cmd              []string            `json:"cmd"`              // 컨테이너 시작 시 실행할 명령어
	Resources        ResourceSettings    `json:"resources"`        // 컨테이너 리소스 제한 설정
	Volumes          []VolumeConfig      `json:"volumes"`          // 볼륨 마운트 설정
}

/*
type ContainerConfig struct {
	ExecutorShell    string                 `json:"executorShell"`    // 예: "./executor.sh"
	HealthcheckShell string                 `json:"healthcheckShell"` // 예: "./healthcheck.sh"
	InstallShell     string                 `json:"installShell"`     // 예: "./install.sh"
	UserScriptShell  string                 `json:"userScriptShell"`  // 예: "./scripts/user_script.sh"
	BuildSettings    ContainerBuildSettings `json:"buildSettings"`    // 컨테이너 내부 구성에 필요한 설정
	Resources        ResourceSettings       `json:"resources"`        // 컨테이너 리소스 제한 설정
	Volumes          []VolumeConfig         `json:"volumes"`          // 볼륨 마운트 설정
}
*/

// ResourceSettings 컨테이너 실행 시 적용할 리소스 제한 설정
type ResourceSettings struct {
	CPU struct {
		CPUQuota  int64  `json:"cpuQuota"`  // 한 주기 동안 사용할 수 있는 최대 CPU 시간 (마이크로초 단위)
		CPUPeriod uint64 `json:"cpuPeriod"` // CPU 제한 주기 (마이크로초 단위)
		CPUShares uint64 `json:"cpuShares"` // 컨테이너의 상대적인 CPU 가중치
	} `json:"cpu"`
	Memory struct {
		MemLimit int64 `json:"memLimit"` // 메모리 제한 (바이트 단위)
	} `json:"memory"`
	OOMScore int `json:"oomScore"` // OOM 발생 시 우선 보호 효과를 위한 값 (음수일수록 보호 효과 큼)
}

// VolumeConfig 컨테이너와 호스트 간의 볼륨 마운트 설정
type VolumeConfig struct {
	HostPath      string `json:"hostPath"`
	ContainerPath string `json:"containerPath"`
}

// NewConfig creates a new BuildConfig using the provided sourceImageName.
// TODO 리소스 설정하는 부분은 수정할 필요 있음.
func NewConfig(sourceImageName string) *BuildConfig {
	internalImgName := internalizeImageName(sourceImageName)
	return &BuildConfig{
		Image: ImageConfig{
			SourceImageName: sourceImageName,
			ImageName:       internalImgName,
			ImageSavePath:   "/opt/images",
			DockerfilePath:  "./Dockerfile",
			Directories:     []string{"/app", "/app/scripts"},
			ScriptMap: map[string][]string{
				"/app":         {"./executor.sh", "./healthcheck.sh", "./install.sh"},
				"/app/scripts": {"./scripts/user_script.sh"},
			},
			PermissionFiles: []string{
				"/app/executor.sh",
				"/app/install.sh",
				"/app/healthcheck.sh",
				"/app/scripts/user_script.sh",
			},
			WorkDir: "/app",
			CMD:     []string{"/bin/sh", "-c", "/app/executor.sh"},
		},
		Container: ContainerConfig{
			ExecutorShell:    "./executor.sh",
			HealthcheckShell: "./healthcheck.sh",
			InstallShell:     "./install.sh",
			UserScriptShell:  "./scripts/user_script.sh",
			Directories:      []string{"/app", "/app/scripts"},
			ScriptMap: map[string][]string{
				"/app":         {"./executor.sh", "./healthcheck.sh", "./install.sh"},
				"/app/scripts": {"./scripts/user_script.sh"},
			},
			PermissionFiles: []string{
				"/app/executor.sh",
				"/app/install.sh",
				"/app/healthcheck.sh",
				"/app/scripts/user_script.sh",
			},
			WorkDir: "/app",
			Cmd:     []string{"/bin/sh", "-c", "/app/executor.sh"},
			Resources: ResourceSettings{
				CPU: struct {
					CPUQuota  int64  `json:"cpuQuota"`
					CPUPeriod uint64 `json:"cpuPeriod"`
					CPUShares uint64 `json:"cpuShares"`
				}{
					CPUQuota:  50000,
					CPUPeriod: 100000,
					CPUShares: 1024,
				},
				Memory: struct {
					MemLimit int64 `json:"memLimit"`
				}{
					MemLimit: 536870912,
				},
				OOMScore: -500,
			},
			Volumes: []VolumeConfig{
				{
					HostPath:      "/data/pipelineA/node1/output",
					ContainerPath: "/app/input",
				},
			},
		},
	}
}

/*
func NewConfig(sourceImageName string) *BuildConfig {
	internalImgName := internalizeImageName(sourceImageName)
	return &BuildConfig{
		Image: ImageConfig{
			SourceImageName: sourceImageName,
			ImageName:       internalImgName,
			ImageSavePath:   "/opt/images",
			DockerfilePath:  "./Dockerfile",
			Directories:     []string{"/app", "/app/scripts"},
			ScriptMap: map[string][]string{
				"/app":         {"./executor.sh", "./healthcheck.sh", "./install.sh"},
				"/app/scripts": {"./scripts/user_script.sh"},
			},
			PermissionFiles: []string{
				"/app/executor.sh",
				"/app/install.sh",
				"/app/healthcheck.sh",
				"/app/scripts/user_script.sh",
			},
			WorkDir: "/app",
			CMD:     []string{"/bin/sh", "-c", "/app/executor.sh"},
		},
		Container: ContainerConfig{
			ExecutorShell:    "./executor.sh",
			HealthcheckShell: "./healthcheck.sh",
			InstallShell:     "./install.sh",
			UserScriptShell:  "./scripts/user_script.sh",
			Directories:      []string{"/app", "/app/scripts"},
			ScriptMap: map[string][]string{
				"/app":         {"./executor.sh", "./healthcheck.sh", "./install.sh"},
				"/app/scripts": {"./scripts/user_script.sh"},
			},
			PermissionFiles: []string{
				"/app/executor.sh",
				"/app/install.sh",
				"/app/healthcheck.sh",
				"/app/scripts/user_script.sh",
			},
			WorkDir: "/app",
			Cmd:     []string{"/bin/sh", "-c", "/app/executor.sh"},
			Resources: ResourceSettings{
				CPU: struct {
					CPUQuota  int64  `json:"cpuQuota"`
					CPUPeriod uint64 `json:"cpuPeriod"`
					CPUShares uint64 `json:"cpuShares"`
				}{
					CPUQuota:  50000,
					CPUPeriod: 100000,
					CPUShares: 1024,
				},
				Memory: struct {
					MemLimit int64 `json:"memLimit"`
				}{
					MemLimit: 536870912,
				},
				OOMScore: -500,
			},
			Volumes: []VolumeConfig{
				{
					HostPath:      "/data/pipelineA/node1/output",
					ContainerPath: "/app/input",
				},
			},
		},
		BuildSettings: ImageBuildSettings{
			Directories: []string{"/app", "/app/scripts"},
			ScriptMap: map[string][]string{
				"/app":         {"./executor.sh", "./healthcheck.sh", "./install.sh"},
				"/app/scripts": {"./scripts/user_script.sh"},
			},
			PermissionFiles: []string{
				"/app/executor.sh",
				"/app/install.sh",
				"/app/healthcheck.sh",
				"/app/scripts/user_script.sh",
			},
			WorkDir: "/app",
			CMD:     []string{"/bin/sh", "-c", "/app/executor.sh"},
		},
	}
}
*/

// NewConfigFromFile 은 지정된 파일에서 설정을 읽어와 BuildConfig 구조체를 생성
// Important: config.json 는 SourceImageName 와 ImageName 는 기본적으로 설정이 안되어 있다. 이 필드들은 사용자로 부터 받아야 하기 때문에 이렇게 처리했다.
func NewConfigFromFile(configPath string) (*BuildConfig, error) {
	configPath, err := utils.CheckPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check path: %w", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var cfg BuildConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode configuration: %w", err)
	}

	return &cfg, nil
}

/*func NewConfigFromFile(configPath string) (*BuildConfig, error) {
	configPath, err := utils.CheckPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check path: %w", err)
	}
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if cErr := file.Close(); cErr != nil && err == nil {
			Log.Warnf("failed to close file: %v", cErr)
		}
	}()

	decoder := json.NewDecoder(file)
	var cfg BuildConfig
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode configuration: %w", err)
	}

	return &cfg, nil
}*/

// 각 설정값을 동적으로 설정할 수 있는 Setter 메서드들

func (config *BuildConfig) SetSourceImageName(sourceImageName string) {
	config.Image.SourceImageName = sourceImageName
}

func (config *BuildConfig) SetImageName(imageName string) {
	config.Image.ImageName = imageName
}

func (config *BuildConfig) SetSourceImageNameAndImageName(sourceImageName string) {
	config.Image.SourceImageName = sourceImageName
	imgName := internalizeImageName(sourceImageName)
	config.Image.ImageName = imgName
}

// SetDirectories 설정 시, 이미지와 컨테이너 양쪽에 동일한 디렉토리 목록을 적용합니다.
func (config *BuildConfig) SetDirectories(directories []string) {
	config.Image.Directories = directories
	config.Container.Directories = directories
}

// SetScriptMap 설정 시, 이미지와 컨테이너 양쪽에 동일한 스크립트 맵을 적용합니다.
func (config *BuildConfig) SetScriptMap(scriptMap map[string][]string) {
	config.Image.ScriptMap = scriptMap
	config.Container.ScriptMap = scriptMap
}

// SetPermissionFiles 설정 시, 이미지와 컨테이너 양쪽에 동일한 파일 권한 목록을 적용합니다.
func (config *BuildConfig) SetPermissionFiles(permissionFiles []string) {
	config.Image.PermissionFiles = permissionFiles
	config.Container.PermissionFiles = permissionFiles
}

// SetWorkDir 설정 시, 이미지와 컨테이너 모두의 작업 디렉토리를 설정합니다.
func (config *BuildConfig) SetWorkDir(workDir string) {
	config.Image.WorkDir = workDir
	config.Container.WorkDir = workDir
}

// SetCMD 설정 시, 이미지와 컨테이너 모두의 CMD 값을 설정합니다.
func (config *BuildConfig) SetCMD(cmd []string) {
	config.Image.CMD = cmd
	config.Container.Cmd = cmd
}

// ------------------------------------------------------
// BuildConfig and Image Creation Functions
// ------------------------------------------------------

// CreateImage 메서드는 BuildSettings 에 설정된 값들을 반영하여 이미지를 생성
func (config *BuildConfig) CreateImage() (*buildah.Builder, string, error) {
	if pbCtx == nil {
		return nil, "", fmt.Errorf("pbCtx is nil")
	}

	// 새로운 빌더 생성 (SourceImageName 을 베이스로 사용)
	builder, err := newBuilder(pbCtx, pbStore, config.Image.SourceImageName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// ImageConfig.Directories 에 지정된 디렉토리 생성
	if err = createDirectories(builder, config.Image.Directories); err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}

	// ImageConfig.ScriptMap 에 지정된 스크립트 복사
	if err = copyScripts(builder, config.Image.ScriptMap); err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// ImageConfig.PermissionFiles 에 지정된 파일 권한 설정
	if err = setFilePermissions(builder, config.Image.PermissionFiles); err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	// 종속성 설치
	if err = installDependencies(builder); err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// 작업 디렉토리 및 CMD 설정 (ImageConfig.WorkDir, CMD)
	builder.SetWorkDir(config.Image.WorkDir)
	builder.SetCmd(config.Image.CMD)

	// 이미지 참조 생성 (ImageName 기반으로)
	imageRef, err := is.Transport.ParseReference(config.Image.ImageName)
	if err != nil {
		return builder, "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// 이미지를 커밋
	imageID, _, _, err := builder.Commit(pbCtx, imageRef, buildah.CommitOptions{
		PreferredManifestType: buildah.Dockerv2ImageManifest,
		SystemContext:         &imageTypes.SystemContext{},
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to commit image: %w", err)
	}

	// 이미지를 저장
	if err = saveImage(pbCtx, config.Image.ImageSavePath, config.Image.ImageName, imageID, false); err != nil {
		return builder, imageID, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageID, nil
}

// TODO  만약 사용자가 os 만 선택한 경우도 생각해야 한다.

// CreateImageWithDockerfile builds an image from a Dockerfile using the BuildConfig.
func (config *BuildConfig) CreateImageWithDockerfile(ctx context.Context, store storage.Store) (*buildah.Builder, string, error) {
	// Dockerfile 경로를 기반으로 이미지를 빌드
	id, err := buildImageFromDockerfile(ctx, config.Image.DockerfilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build image from Dockerfile: %w", err)
	}

	// 새로운 빌더 생성
	builder, err := newBuilder(ctx, store, id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// ImageConfig.Directories 에 지정된 디렉토리 생성
	if err = createDirectories(builder, config.Image.Directories); err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}

	// ImageConfig.ScriptMap 에 지정된 스크립트 복사
	if err = copyScripts(builder, config.Image.ScriptMap); err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// ImageConfig.PermissionFiles 에 지정된 파일 권한 설정
	if err = setFilePermissions(builder, config.Image.PermissionFiles); err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	// 종속성 설치
	if err = installDependencies(builder); err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// 작업 디렉토리 및 CMD 설정 (ImageConfig.WorkDir, CMD)
	builder.SetWorkDir(config.Image.WorkDir)
	builder.SetCmd(config.Image.CMD)

	// 이미지 참조 생성
	imageRef, err := is.Transport.ParseReference(config.Image.ImageName)
	if err != nil {
		return builder, "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// 이미지를 커밋
	imageID, _, _, err := builder.Commit(ctx, imageRef, buildah.CommitOptions{
		PreferredManifestType: buildah.Dockerv2ImageManifest,
		SystemContext:         &imageTypes.SystemContext{},
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to commit image: %w", err)
	}

	// 이미지를 저장
	if err = saveImage(ctx, config.Image.ImageSavePath, config.Image.ImageName, imageID, false); err != nil {
		return builder, imageID, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageID, nil
}

/*func (img *ImageConfig) CreateImage(ctx context.Context, store storage.Store) (*buildah.Builder, string, error) {
	if pbCtx == nil {
		return nil, "", fmt.Errorf("pbCtx is nil")
	}

	var builder *buildah.Builder
	var err error
	var baseID string

	// DockerfilePath가 설정되어 있으면 Dockerfile 기반 빌드 진행
	if img.DockerfilePath != "" {
		baseID, err = buildImageFromDockerfile(ctx, img.DockerfilePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to build image from Dockerfile: %w", err)
		}
	} else {
		// 그렇지 않으면, SourceImageName을 베이스로 사용
		baseID = img.SourceImageName
	}

	// 새로운 빌더 생성 (baseID를 사용)
	builder, err = newBuilder(ctx, pbStore, baseID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// 이미지 참조 생성 (ImageName 기반)
	imageRef, err := is.Transport.ParseReference(img.ImageName)
	if err != nil {
		return builder, "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// 이미지를 커밋
	imageID, _, _, err := builder.Commit(ctx, imageRef, buildah.CommitOptions{
		PreferredManifestType: buildah.Dockerv2ImageManifest,
		SystemContext:         &imageTypes.SystemContext{},
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to commit image: %w", err)
	}

	// 이미지를 저장
	if err = saveImage(ctx, img.ImageSavePath, img.ImageName, imageID, false); err != nil {
		return builder, imageID, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageID, nil
}*/

//TODO 생각하기 ContainerConfig 로 할 필요가 있을까??

// SetupContainer sets up the container environment based on ContainerConfig.
func (c *ContainerConfig) SetupContainer(builder *buildah.Builder) error {
	// ContainerConfig.Directories 에 지정된 디렉토리 생성
	if err := createDirectories(builder, c.Directories); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// ContainerConfig.ScriptMap 에 지정된 스크립트 복사
	if err := copyScripts(builder, c.ScriptMap); err != nil {
		return fmt.Errorf("failed to copy scripts: %w", err)
	}

	// ContainerConfig.PermissionFiles 에 지정된 파일 권한 설정
	if err := setFilePermissions(builder, c.PermissionFiles); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// 종속성 설치
	if err := installDependencies(builder); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	// 작업 디렉토리 및 CMD 설정
	builder.SetWorkDir(c.WorkDir)
	builder.SetCmd(c.Cmd)

	return nil
}
