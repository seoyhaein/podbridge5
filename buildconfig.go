package podbridge5

import (
	"context"
	"fmt"
	"github.com/containers/buildah"
	is "github.com/containers/image/v5/storage"
	imageTypes "github.com/containers/image/v5/types"
)

// ImageBuildSettings holds additional settings for building an image.
type ImageBuildSettings struct {
	Directories     []string            // 컨테이너 내부에서 생성할 디렉토리 목록
	ScriptMap       map[string][]string // 키: 대상 디렉토리, 값: 복사할 스크립트 파일 목록
	PermissionFiles []string            // 권한 설정을 적용할 파일 경로 목록
	WorkDir         string              // 최종 이미지의 작업 디렉토리
	CMD             []string            // 컨테이너 시작 시 실행할 명령어
}

// BuildConfig holds basic configuration for building an image,
// plus additional build settings.
type BuildConfig struct {
	SourceImageName  string // 기본 베이스 이미지 (예: "docker.io/library/alpine:latest")
	ImageName        string // 최종 이미지 이름 (예: "tester")
	ImageSavePath    string // 이미지를 저장할 경로 (예: "/opt/images")
	ExecutorShell    string // executor 스크립트 경로 (예: "./executor.sh")
	DockerfilePath   string // Dockerfile 경로 (예: "./Dockerfile")
	HealthcheckShell string // healthcheck 스크립트 경로 (예: "./healthcheck.sh")
	InstallShell     string // install 스크립트 경로 (예: "./install.sh")
	UserScriptShell  string // user script 스크립트 경로 (예: "./scripts/user_script.sh")
	BuildSettings    ImageBuildSettings
}

/*// NewImageBuildSettings ImageBuildSettings 생성하는 팩토리 메서드. TODO 여기서는 값을 리턴했다. 생각해보자.
func NewImageBuildSettings(
	directories []string,
	scriptMap map[string][]string,
	permissionFiles []string,
	workDir string,
	cmd []string,
) ImageBuildSettings {
	return ImageBuildSettings{
		Directories:     directories,
		ScriptMap:       scriptMap,
		PermissionFiles: permissionFiles,
		WorkDir:         workDir,
		CMD:             cmd,
	}
}

// NewBuildConfig BuildConfig 생성하는 팩토리 메서드
// 필요한 모든 필드를 인자로 받아서 BuildConfig 인스턴스를 반환
// sourceImageName 와 imageName 는 별도로 설정받아야 함.
func NewBuildConfig(
	imageSavePath,
	executorShell, healthcheckShell, installShell, userScriptShell string,
	settings ImageBuildSettings,
) *BuildConfig {
	return &BuildConfig{
		ImageSavePath:    imageSavePath,
		ExecutorShell:    executorShell,
		HealthcheckShell: healthcheckShell,
		InstallShell:     installShell,
		UserScriptShell:  userScriptShell,
		BuildSettings:    settings,
	}
}*/

// NewConfig sourceImageName 만 동적으로 받고, 나머지 BuildConfig 필드를 기본 값으로 고정함. 내부에서 사용하는 이미지 생성에 필요한 옵션임.
// TODO DockerfilePath 일단 살펴보자.
func NewConfig(sourceImageName string) *BuildConfig {
	return &BuildConfig{
		SourceImageName: sourceImageName,
		// sourceImageName 뒤에 "_internal"을 붙여 내부 이미지 이름으로 사용
		ImageName:        sourceImageName + "_internal",
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

// 각 설정값을 동적으로 설정할 수 있는 Setter 메서드들

func (config *BuildConfig) SetSourceImageName(sourceImageName string) {
	config.SourceImageName = sourceImageName
}

func (config *BuildConfig) SetImageName(imageName string) {
	config.ImageName = imageName
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

// CreateImage3 메서드는 BuildSettings 에 설정된 값들을 반영하여 이미지를 생성, TODO pbStore nil 인지 확인 필요하지 않을까??
func (config *BuildConfig) CreateImage3(ctx context.Context) (*buildah.Builder, string, error) {
	// 새로운 빌더 생성 (SourceImageName 베이스 이미지로 사용)
	ctx, builder, err := newBuilder(ctx, pbStore, config.SourceImageName)
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
	imageId, _, _, err := builder.Commit(ctx, imageRef, buildah.CommitOptions{
		PreferredManifestType: buildah.Dockerv2ImageManifest,
		SystemContext:         &imageTypes.SystemContext{},
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to commit image: %w", err)
	}

	// 이미지를 저장
	if err = saveImage(ctx, config.ImageSavePath, config.ImageName, "", imageId, false); err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}
