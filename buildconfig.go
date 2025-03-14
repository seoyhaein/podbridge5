package podbridge5

import (
	"encoding/json"
	"fmt"
	"github.com/containers/buildah"
	is "github.com/containers/image/v5/storage"
	imageTypes "github.com/containers/image/v5/types"
	"github.com/seoyhaein/utils"
	"os"
	"strings"
)

// ImageBuildSettings holds additional settings for building an image.
type ImageBuildSettings struct {
	Directories     []string            `json:"directories"`     // 컨테이너 내부에서 생성할 디렉토리 목록
	ScriptMap       map[string][]string `json:"scriptMap"`       // 대상 디렉토리별 복사할 스크립트 파일 목록
	PermissionFiles []string            `json:"permissionFiles"` // 파일 권한 설정을 적용할 파일 경로 목록 (최종 경로 기준)
	WorkDir         string              `json:"workDir"`         // 컨테이너 작업 디렉토리
	CMD             []string            `json:"cmd"`             // 컨테이너 시작 시 실행할 명령어
}

// BuildConfig holds basic configuration for building an image,
// plus additional build settings.
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

// NewConfig sourceImageName 만 동적으로 받고, 나머지 BuildConfig 필드를 기본 값으로 고정함. 내부에서 사용하는 이미지 생성에 필요한 옵션임.
// TODO DockerfilePath 일단 살펴보자.
func NewConfig(sourceImageName string) *BuildConfig {
	internalImgName := internalizeImageName(sourceImageName)
	return &BuildConfig{
		SourceImageName: sourceImageName,
		// sourceImageName 뒤에 "_internal"을 붙여 내부 이미지 이름으로 사용
		ImageName:        internalImgName,
		ImageSavePath:    "/opt/images",
		ExecutorShell:    "./executor.sh",
		HealthcheckShell: "./healthcheck.sh",
		DockerfilePath:   "./Dockerfile",
		InstallShell:     "./install.sh",
		UserScriptShell:  "./scripts/user_script.sh",
		BuildSettings: ImageBuildSettings{
			Directories: []string{"/app", "/app/scripts"}, // 컨테이너 내부 생성할 디렉토리 목록
			ScriptMap: map[string][]string{ // 대상 디렉토리별 복사할 스크립트 목록
				"/app":         {"./executor.sh", "./healthcheck.sh", "./install.sh"},
				"/app/scripts": {"./scripts/user_script.sh"},
			},
			PermissionFiles: []string{ // 파일 권한 설정을 적용할 파일 목록 (최종 경로 기준)
				"/app/executor.sh",
				"/app/install.sh",
				"/app/healthcheck.sh",
				"/app/scripts/user_script.sh",
			},
			WorkDir: "/app",                                        // 컨테이너 작업 디렉토리
			CMD:     []string{"/bin/sh", "-c", "/app/executor.sh"}, // 컨테이너 시작 시 실행할 명령어
		},
	}
}

func NewConfigFromFile(configPath string) (*BuildConfig, error) {
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

	// 필수 항목 검증
	/*if cfg.SourceImageName == "" {
		return nil, fmt.Errorf("missing 'sourceImageName' in configuration")
	}
	if cfg.ImageSavePath == "" {
		return nil, fmt.Errorf("missing 'imageSavePath' in configuration")
	}*/

	// SourceImageName 뒤에 "_internal"을 붙여 ImageName 설정 (옵션)
	/*if cfg.ImageName == "" {
		cfg.ImageName = cfg.SourceImageName + "_internal"
	}*/

	return &cfg, nil
}

// 각 설정값을 동적으로 설정할 수 있는 Setter 메서드들

func (config *BuildConfig) SetSourceImageName(sourceImageName string) {
	config.SourceImageName = sourceImageName
}

func (config *BuildConfig) SetImageName(imageName string) {
	config.ImageName = imageName
}

func (config *BuildConfig) SetSourceImageNameAndImageName(sourceImageName string) {
	config.SourceImageName = sourceImageName
	config.ImageName = internalizeImageName(sourceImageName)
}

// SetDirectories 컨테이너 내부에서 생성할 디렉토리 목록을 설정합니다.
func (config *BuildConfig) SetDirectories(directories []string) {
	config.BuildSettings.Directories = directories
}

// SetScriptMap 스크립트를 복사할 대상 디렉토리와 파일 목록을 설정합니다.
func (config *BuildConfig) SetScriptMap(scriptMap map[string][]string) {
	config.BuildSettings.ScriptMap = scriptMap
}

// SetPermissionFiles 파일 권한 설정을 적용할 파일 목록을 설정합니다.
func (config *BuildConfig) SetPermissionFiles(permissionFiles []string) {
	config.BuildSettings.PermissionFiles = permissionFiles
}

// SetWorkDir 최종 컨테이너의 작업 디렉토리를 설정합니다.
func (config *BuildConfig) SetWorkDir(workDir string) {
	config.BuildSettings.WorkDir = workDir
}

// SetCMD 컨테이너 시작 시 실행할 명령어(CMD)를 설정합니다.
func (config *BuildConfig) SetCMD(cmd []string) {
	config.BuildSettings.CMD = cmd
}

// CreateImage3 메서드는 BuildSettings 에 설정된 값들을 반영하여 이미지를 생성,
func (config *BuildConfig) CreateImage3() (*buildah.Builder, string, error) {
	if pbCtx == nil {
		return nil, "", fmt.Errorf("pbCtx is nil")
	}

	// 새로운 빌더 생성 (SourceImageName 베이스 이미지로 사용)
	builder, err := newBuilder(pbCtx, pbStore, config.SourceImageName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// BuildSettings.Directories 에 지정된 디렉토리 생성
	if err = createDirectories(builder, config.BuildSettings.Directories); err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}

	// BuildSettings.ScriptMap 에 지정된 스크립트 복사
	if err = copyScripts(builder, config.BuildSettings.ScriptMap); err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// BuildSettings.PermissionFiles 에 지정된 파일 권한 설정
	if err = setFilePermissions(builder, config.BuildSettings.PermissionFiles); err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	// 종속성 설치
	if err = installDependencies(builder); err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// 작업 디렉토리 및 CMD 설정 (BuildSettings.WorkDir, CMD)
	builder.SetWorkDir(config.BuildSettings.WorkDir)
	builder.SetCmd(config.BuildSettings.CMD)

	// 이미지 참조 생성 (ImageName 기반으로)
	imageRef, err := is.Transport.ParseReference(config.ImageName)
	if err != nil {
		return builder, "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// 이미지를 커밋
	imageId, _, _, err := builder.Commit(pbCtx, imageRef, buildah.CommitOptions{
		PreferredManifestType: buildah.Dockerv2ImageManifest,
		SystemContext:         &imageTypes.SystemContext{},
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to commit image: %w", err)
	}

	// 이미지를 저장
	if err = saveImage(pbCtx, config.ImageSavePath, config.ImageName, "", imageId, false); err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// internalizeImageName 은 입력 이미지 이름에서 태그 앞에 "-internal"을 삽입하여 내부 전용 이미지 이름을 생성
// 예: "docker.io/library/alpine:latest" -> "docker.io/library/alpine-internal:latest"
func internalizeImageName(imageName string) string {
	// 마지막 콜론의 인덱스를 찾습니다.
	colonIndex := strings.LastIndex(imageName, ":")
	if colonIndex == -1 {
		// 태그가 없는 경우, 그냥 "-internal"을 추가합니다.
		return imageName + "-internal"
	}

	// 콜론 앞까지의 이미지 이름과 태그를 분리
	baseName := imageName[:colonIndex]
	tag := imageName[colonIndex:] // 콜론 포함

	// 내부 전용 이미지 이름 생성
	return baseName + "-internal" + tag
}
