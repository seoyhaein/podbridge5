```aiignore
package podbridge5

import (
	"compress/gzip"
	"context"
	"fmt"
	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	is "github.com/containers/image/v5/storage"
	imageTypes "github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
	"github.com/seoyhaein/utils"
	"io"
	"os"
	"path/filepath"
)

// Deprecated: 사용하지 않음.
// InitForBuildah initializes buildah for rootless mode. 사용하지 않음.
// TODO 이렇게 하면 에러 남. 그냥 메서드만 빠져나감. 다시 시작 되지 않음. 경고를 위해서 남겨둠.
func InitForBuildah() {
	Log.Info("Initializing buildah for rootless mode")
	if buildah.InitReexec() {
		Log.Info("Reexec initiated")
		return
	}
	Log.Info("Proceeding with MaybeReexecUsingUserNamespace")
	unshare.MaybeReexecUsingUserNamespace(false)
	Log.Info("Initialization complete")
}

// Deprecated: 사용하지 않음.
// NewBuilder_old
func NewBuilder_old(ctx context.Context, store storage.Store, opts ...func(*buildah.BuilderOptions)) (context.Context, *buildah.Builder, error) {
	// Create a new BuilderOptions with the provided settings
	builderOpts := &buildah.BuilderOptions{}
	for _, applyOpt := range opts {
		applyOpt(builderOpts)
	}
	// Create the buildah.Builder
	builder, err := buildah.NewBuilder(ctx, store, *builderOpts)
	if err != nil {
		return nil, nil, err
	}

	return ctx, builder, nil
}

// Deprecated: 사용하지 않음.
// WithCapabilities_old
func WithCapabilities_old() func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		cap, _ := capabilities()
		opts.Capabilities = cap
	}
}

// Deprecated: 사용하지 않음.
// WithArg_old sets an argument for the build
func WithArg_old(key, value string) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if opts.Args == nil {
			opts.Args = make(map[string]string)
		}
		if _, ok := opts.Args[key]; !ok {
			opts.Args[key] = value
		}
	}
}

// Deprecated: 사용하지 않음.
// WithFromImage_old sets the base image for the build
func WithFromImage_old(image string) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if !utils.IsEmptyString(image) {
			opts.FromImage = image
		}
	}
}

// Deprecated: 사용하지 않음.
// WithIsolation_old sets the isolation mode for the builder options.
func WithIsolation_old(isolation define.Isolation) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		opts.Isolation = isolation
	}
}

// Deprecated: 사용하지 않음.
// WithCommonBuildOptions_old sets the common build options such as CPU and memory limits.
func WithCommonBuildOptions_old(options *buildah.CommonBuildOptions) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if options != nil {
			opts.CommonBuildOpts = options
		} else {
			opts.CommonBuildOpts = &buildah.CommonBuildOptions{}
		}
	}
}

// Deprecated: 사용하지 않음.
// WithSystemContext_old sets the system context for the builder options.
func WithSystemContext_old(sysCtx *imageTypes.SystemContext) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if sysCtx != nil {
			opts.SystemContext = sysCtx
		} else {
			opts.SystemContext = &imageTypes.SystemContext{}
		}
	}
}

// Deprecated: 사용하지 않음.
// WithNetworkConfiguration_old sets the network configuration policy for the builder options.
func WithNetworkConfiguration_old(policy define.NetworkConfigurationPolicy) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		opts.ConfigureNetwork = policy
	}
}

// Deprecated: 사용하지 않음.
// WithFormat_old sets the format for the container image to be committed.
func WithFormat_old(format string) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		opts.Format = format
	}
}

// ensureShebang  TODO 삭제하지만 주석으로 남겨둔다.
/*func ensureShebang(scriptContent string) string {
	// 스크립트에서 #!이 처음 등장하는 위치를 찾음
	shebangIndex := strings.Index(scriptContent, "#!")

	// #!이 첫 번째 줄이 아닌 경우, 앞에 불필요한 내용 제거
	if shebangIndex > 0 {
		// shebang 앞의 공백이나 불필요한 내용을 제거
		scriptContent = scriptContent[shebangIndex:]
	}
	return strings.TrimSpace(scriptContent)
}*/

// TestEnsureShebang ensureShebang 사용하지 않음. 삭제하지 않고  주석으로 남김.
/*func TestEnsureShebang(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{"#!/bin/bash\necho 'Hello, World!'", "#!/bin/bash\necho 'Hello, World!'"},
		{"\n#!/bin/bash\necho 'Hello, World!'", "#!/bin/bash\necho 'Hello, World!'"},
		{"echo 'Hello, World!'", "#!/bin/sh\necho 'Hello, World!'"}, // 기본 shebang이 추가되어야 함
	}

	for _, test := range tests {
		result := ensureShebang(test.input)
		if result != test.expectedOutput {
			t.Errorf("Unexpected result for ensureShebang. Got: %s, Want: %s", result, test.expectedOutput)
		}
	}
}*/

// CreateImage creates an image based on a default OS.
func CreateImage(ctx context.Context, store storage.Store, config BuildConfig) (*buildah.Builder, string, error) {

	builder, err := newBuilder(ctx, store, config.SourceImageName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// 공통 디렉토리 생성 함수 호출
	directories := []string{"/app", "/app/scripts"}
	err = createDirectories(builder, directories)
	if err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}

	// 스크립트 복사 test 진행 중 이후 삭제해야 함.
	scripts := map[string][]string{
		"/app":         {config.ExecutorShell, config.HealthcheckShell, config.InstallShell},
		"/app/scripts": {config.UserScriptShell},
	}

	err = copyScripts(builder, scripts)
	if err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// 스크립트 권한 설정
	err = setFilePermissions(builder, []string{
		"/app/executor.sh",
		"/app/install.sh",
		"/app/healthcheck.sh",
		"/app/scripts/user_script.sh",
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	err = installDependencies(builder)
	if err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// TODO: 데이터가 없는 이미지를 만들때는 CMD 는 없어야 한다. 데이터가 들어간 이미지를 만들어 주는 메서드를 만들때 넣어준다.
	// CMD 설정 (executor.sh 실행)
	builder.SetWorkDir("/app")
	// TODO: 테스트 때문에 주석 풀음. 여기서는 이거 풀면 안됨. 즉, 여기서는 컨테이너 만들면 안됨.
	builder.SetCmd([]string{"/bin/sh", "-c", "/app/executor.sh"})

	// 이미지 참조 생성
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, imageId, false)
	if err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// CreateImage3 creates an image based on BuildConfig.
func (config *BuildConfig) CreateImage3(ctx context.Context, store storage.Store) (*buildah.Builder, string, error) {
	// 새로운 빌더 생성
	builder, err := newBuilder(ctx, store, config.SourceImageName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// 공통 디렉토리 생성 함수 호출
	directories := []string{"/app", "/app/scripts"}
	err = createDirectories(builder, directories)
	if err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}

	// 스크립트 복사 (test 용이므로 이후 삭제)
	scripts := map[string][]string{
		"/app":         {config.ExecutorShell, config.HealthcheckShell, config.InstallShell},
		"/app/scripts": {config.UserScriptShell},
	}
	err = copyScripts(builder, scripts)
	if err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// 스크립트 권한 설정
	err = setFilePermissions(builder, []string{
		"/app/executor.sh",
		"/app/install.sh",
		"/app/healthcheck.sh",
		"/app/scripts/user_script.sh",
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	// 종속성 설치
	err = installDependencies(builder)
	if err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// CMD 설정 (데이터가 없는 기본 이미지를 만들 경우 CMD 설정은 생략해야 함)
	builder.SetWorkDir("/app")
	// TODO: 테스트 중임으로 주석 처리 풀었으나 최종 코드에서는 CMD 제거 필요
	builder.SetCmd([]string{"/bin/sh", "-c", "/app/executor.sh"})

	// 이미지 참조 생성
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, imageId, false)
	if err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

func (config *BuildConfig) CreateImage1(ctx context.Context, store storage.Store) (*buildah.Builder, string, error) {
	// 새로운 빌더 생성 (SourceImageName을 기반으로)
	builder, err := newBuilder(ctx, store, config.SourceImageName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// BuildSettings에 지정된 공통 디렉토리 생성
	err = createDirectories(builder, config.BuildSettings.Directories)
	if err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}

	// BuildSettings에 지정된 스크립트 복사
	err = copyScripts(builder, config.BuildSettings.ScriptMap)
	if err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// BuildSettings에 지정된 파일 권한 설정
	err = setFilePermissions(builder, config.BuildSettings.PermissionFiles)
	if err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	// 종속성 설치
	err = installDependencies(builder)
	if err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// 작업 디렉토리 및 CMD 설정
	builder.SetWorkDir(config.BuildSettings.WorkDir)
	builder.SetCmd(config.BuildSettings.CMD)

	// 이미지 참조 생성
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, imageId, false)
	if err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// CreateImage2 메서드는 BuildSettings에 설정된 값들을 반영하여 이미지를 생성합니다.
func (config *BuildConfig) CreateImage2(ctx context.Context, store storage.Store) (*buildah.Builder, string, error) {
	// 새로운 빌더 생성 (SourceImageName을 베이스 이미지로 사용)
	builder, err := newBuilder(ctx, store, config.SourceImageName)
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

	// 이미지 참조 생성 (ImageName을 기반으로)
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
	if err = saveImage(ctx, config.ImageSavePath, config.ImageName, imageId, false); err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// ------------------------------------------------------
// Unused Code (주석 처리된 함수들)
// ------------------------------------------------------
/*
// Run 사용하지 않음. 삭제하지 않고 주석 처리함.
func (b *Builder) Run(s string) error {

	logger := GetLoggerWriter()
	runOptions := buildah.RunOptions{
		Stdout:    logger,
		Stderr:    logger,
		Isolation: define.IsolationChroot,
	}
	var (
		ac [][]string
		c  []string
	)
	command := strings.Split(s, " ")
	for i := 0; i < len(command); i++ {
		if command[i] == "&&" {
			ac = append(ac, c)
			c = nil
		} else {
			c = append(c, command[i])
		}
	}
	if len(c) > 0 {
		ac = append(ac, c)
	}
	for j := 0; j < len(ac); j++ {
		err := b.builder.Run(ac[j], runOptions)
		if err != nil {
			return fmt.Errorf("error while runnning command: %v", err)
		}
	}
	return nil
}

// WorkDir 사용하지 않음. 삭제하지 않고 주석 처리함.
func (b *Builder) WorkDir(path string) error {
	if utils.IsEmptyString(path) {
		return fmt.Errorf("path is empty")
	}
	b.builder.SetWorkDir(path)
	return nil
}

// Env 사용하지 않음. 삭제하지 않고 주석 처리함.
func (b *Builder) Env(k, v string) error {
	if utils.IsEmptyString(k) || utils.IsEmptyString(v) {
		return fmt.Errorf("key or valeu is empty")
	}

	b.builder.SetEnv(k, v)
	return nil
}

// User 사용하지 않음. 삭제하지 않고 주석 처리함.
func (b *Builder) User(u string) error {
	if utils.IsEmptyString(u) {
		return fmt.Errorf("user is empty")
	}

	b.builder.SetUser(u)
	return nil
}

// Expose 사용하지 않음. 삭제하지 않고 주석 처리함.
func (b *Builder) Expose(port string) error {
	if utils.IsEmptyString(port) {
		return fmt.Errorf("port is empty")
	}
	b.builder.SetPort(port)
	return nil
}

// Cmd 사용하지 않음. 삭제하지 않고 주석 처리함.
func (b *Builder) Cmd(cmd ...string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command is empty")
	}
	b.builder.SetCmd(cmd)
	return nil
}

// CommitImage 사용하지 않음. 삭제하지 않고 주석 처리함.
func (b *Builder) CommitImage(ctx context.Context, preferredManifestType string, sysCtx *imageTypes.SystemContext, repository string) (*string, error) {

	imageRef, err := is.Transport.ParseStoreReference(b.store, repository)
	if err != nil {
		return nil, err
	}

	imageId, _, _, err := b.builder.Commit(ctx, imageRef, buildah.CommitOptions{
		PreferredManifestType: preferredManifestType,
		SystemContext:         sysCtx,
	})

	return &imageId, err
}

// GetLoggerWriter 사용하지 않음. 삭제하지 않고 주석 처리함.
func GetLoggerWriter() io.Writer {
	if Verbose || Debug {
		return os.Stdout
	} else {
		return NopLogger{}
	}
}

type NopLogger struct{}

func (n NopLogger) Write(p []byte) (int, error) {
	return len(p), nil
}
*/

// saveImage1 saves the built image to an archive file.
func saveImage1(ctx context.Context, path, imageName, imageTag, imageId string, compress bool) error {
	if imageTag == "" {
		imageTag = "latest"
	}

	// 파일명 설정
	extension := ".tar"
	if compress {
		extension = ".tar.gz"
	}
	archiveFileName := fmt.Sprintf("%s-%s%s", imageName, imageTag, extension)
	archivePath := filepath.Join(path, archiveFileName)

	dir := filepath.Dir(archivePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	// 파일을 생성하고 권한을 명시적으로 설정
	outputFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", archivePath, err)
	}

	defer func() {
		if cErr := outputFile.Close(); cErr != nil {
			Log.Warnf("Failed to close output file: %v", cErr)
		}
	}()

	var writer io.Writer = outputFile

	if compress {
		// gzip.Writer 를 사용하여 데이터를 압축
		gzipWriter := gzip.NewWriter(outputFile)
		defer func() {
			if zCerr := gzipWriter.Close(); zCerr != nil {
				Log.Errorf("Failed to close gzip writer: %v", zCerr)
			}
		}()
		writer = gzipWriter
	}
	exportOptions := &images.ExportOptions{
		// 필요한 경우 추가 옵션을 설정
	}

	if err := images.Export(ctx, []string{imageId}, writer, exportOptions); err != nil {
		return fmt.Errorf("failed to export image %s: %w", imageId, err)
	}
	return nil
}

// CreateImageWithDockerfile2 builds an image using Dockerfile and then copies required scripts.
func CreateImageWithDockerfile2(ctx context.Context, store storage.Store, config BuildConfig) (*buildah.Builder, string, error) {
	// Dockerfile 로부터 이미지를 빌드 (alpine 이미지 사용)
	ctx, id, err := buildImageFromDockerfile(ctx, config.DockerfilePath)
	if err != nil {
		Log.Errorf("Failed to build image from Dockerfile: %v", err)
		return nil, "", err
	}

	// 새로운 빌더 생성
	builder, err := newBuilder(ctx, store, id)
	if err != nil {
		Log.Errorf("Failed to create new builder: %v", err)
		return nil, "", err
	}

	// WORKDIR 설정
	err = builder.Run([]string{"mkdir", "-p", "/app"}, buildah.RunOptions{
		User:      "root",
		Isolation: define.IsolationOCI,
		Runtime:   "runc",
	})
	if err != nil {
		Log.Errorf("Failed to create /app directory: %v", err)
		return builder, "", fmt.Errorf("failed to create /app directory: %w", err)
	}

	// workdir set.
	builder.SetWorkDir("/app")

	// /app/scripts 디렉토리 생성
	err = builder.Run([]string{"mkdir", "-p", "/app/scripts"}, buildah.RunOptions{
		User:      "root",
		Isolation: define.IsolationOCI,
		Runtime:   "runc",
	})
	if err != nil {
		Log.Errorf("Failed to create /app/scripts directory: %v", err)
		return builder, "", fmt.Errorf("failed to create /app/scripts directory: %w", err)
	}

	// 스크립트 복사 (executor.sh, healthcheck.sh, user_script.sh)
	options := newAddAndCopyOptions()
	err = builder.Add("/app", false, options, config.ExecutorShell) // executor.sh 복사
	if err != nil {
		Log.Errorf("Failed to add executor.sh to /app: %v", err)
		return builder, "", fmt.Errorf("failed to add executor.sh: %w", err)
	}

	err = builder.Add("/app", false, options, config.HealthcheckShell) // healthcheck.sh 복사
	if err != nil {
		Log.Errorf("Failed to add healthcheck.sh to /app: %v", err)
		return builder, "", fmt.Errorf("failed to add healthcheck.sh: %w", err)
	}

	err = builder.Add("/app/scripts", false, options, config.UserScriptShell) // user_script.sh 복사
	if err != nil {
		Log.Errorf("Failed to add user_script.sh to /app/scripts: %v", err)
		return builder, "", fmt.Errorf("failed to add user_script.sh: %w", err)
	}

	// 파일 권한 777로 설정
	err = builder.Run([]string{"chmod", "777", "/app/executor.sh", "/app/healthcheck.sh", "/app/scripts/user_script.sh"}, buildah.RunOptions{
		User:      "root",
		Isolation: define.IsolationOCI,
		Runtime:   "runc",
	})
	if err != nil {
		Log.Errorf("Failed to set permissions on scripts: %v", err)
		return builder, "", fmt.Errorf("failed to set permissions: %w", err)
	}

	// CMD 설정 (executor.sh 실행)
	builder.SetCmd([]string{"/bin/sh", "-c", "/app/executor.sh"})

	// 이미지 참조 생성
	imageRef, err := is.Transport.ParseReference(config.ImageName)
	if err != nil {
		Log.Errorf("Failed to parse image reference: %v", err)
		return builder, "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// 이미지를 커밋
	imageId, _, _, err := builder.Commit(ctx, imageRef, buildah.CommitOptions{
		PreferredManifestType: buildah.Dockerv2ImageManifest,
		SystemContext:         &imageTypes.SystemContext{},
	})
	if err != nil {
		Log.Errorf("Failed to commit image: %v", err)
		return builder, "", fmt.Errorf("failed to commit image: %w", err)
	}

	// 이미지를 저장
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, imageId, false)
	if err != nil {
		Log.Errorf("Failed to save image: %v", err)
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// CreateImageWithDockerfile builds an image from a Dockerfile and performs additional steps.
func CreateImageWithDockerfile(ctx context.Context, store storage.Store, config BuildConfig) (*buildah.Builder, string, error) {
	ctx, id, err := buildImageFromDockerfile(ctx, config.DockerfilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build image from Dockerfile: %w", err)
	}

	// 새로운 빌더 생성
	builder, err := newBuilder(ctx, store, id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// 공통 디렉토리 생성 함수 호출
	directories := []string{"/app", "/app/scripts"}
	err = createDirectories(builder, directories)
	if err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}
	// 이때 이미 user_script.sh, healthcheck.sh, executor.sh 이 만들어져 있어야 함.
	// 스크립트 복사
	scripts := map[string][]string{
		"/app":         {config.ExecutorShell, config.HealthcheckShell, config.InstallShell},
		"/app/scripts": {config.UserScriptShell},
	}
	// TODO: 여기서 파일이 있는지 검사하는게 낫겠다.
	err = copyScripts(builder, scripts)
	if err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// 스크립트 권한 설정
	err = setFilePermissions(builder, []string{
		"/app/executor.sh",
		"/app/healthcheck.sh",
		"/app/install.sh",
		"/app/scripts/user_script.sh",
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	err = installDependencies(builder)
	if err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// CMD 설정 (executor.sh 실행)
	builder.SetWorkDir("/app")
	// TODO: 테스트 때문에 주석 풀음. 여기서는 이거 풀면 안됨. 즉, 여기서는 컨테이너 만들면 안됨.
	builder.SetCmd([]string{"/bin/sh", "-c", "/app/executor.sh"})

	// 이미지 참조 생성
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, imageId, false)
	if err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// CreateImageWithDockerfile1 builds an image from a Dockerfile using the BuildConfig method.
func (config *BuildConfig) CreateImageWithDockerfile1(ctx context.Context, store storage.Store) (*buildah.Builder, string, error) {
	ctx, id, err := buildImageFromDockerfile(ctx, config.DockerfilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build image from Dockerfile: %w", err)
	}

	// 새로운 빌더 생성
	builder, err := newBuilder(ctx, store, id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create new builder: %w", err)
	}

	// 공통 디렉토리 생성 함수 호출
	directories := []string{"/app", "/app/scripts"}
	err = createDirectories(builder, directories)
	if err != nil {
		return builder, "", fmt.Errorf("failed to create directories: %w", err)
	}

	// 스크립트 복사
	scripts := map[string][]string{
		"/app":         {config.ExecutorShell, config.HealthcheckShell, config.InstallShell},
		"/app/scripts": {config.UserScriptShell},
	}
	err = copyScripts(builder, scripts)
	if err != nil {
		return builder, "", fmt.Errorf("failed to copy scripts: %w", err)
	}

	// 스크립트 권한 설정
	err = setFilePermissions(builder, []string{
		"/app/executor.sh",
		"/app/install.sh",
		"/app/healthcheck.sh",
		"/app/scripts/user_script.sh",
	})
	if err != nil {
		return builder, "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	err = installDependencies(builder)
	if err != nil {
		return builder, "", fmt.Errorf("failed to install dependency: %w", err)
	}

	// CMD 설정 (executor.sh 실행)
	builder.SetWorkDir("/app")
	builder.SetCmd([]string{"/bin/sh", "-c", "/app/executor.sh"})

	// 이미지 참조 생성
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, imageId, false)
	if err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

```