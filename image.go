package podbridge5

import (
	"compress/gzip"
	"context"
	"fmt"
	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/common/pkg/config"
	is "github.com/containers/image/v5/storage"
	imageTypes "github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
	"github.com/opencontainers/go-digest"
	"github.com/seoyhaein/utils"
	"io"
	"os"
	"path/filepath"
)

var (
	digester = digest.Canonical.Digester()

	// 사용하지 않음 주석처리함. 삭제하지 않음.
	/*	Verbose = true
		Debug   = true*/

)

// 사용하지 않음 주석 처리함. 삭제하지 않음.
/*type Builder struct {
	store   storage.Store
	builder *buildah.Builder
}*/

// WithArg sets an argument for the build
func WithArg(key, value string) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if opts.Args == nil {
			opts.Args = make(map[string]string)
		}
		if _, ok := opts.Args[key]; !ok {
			opts.Args[key] = value
		}
	}
}

// WithFromImage sets the base image for the build
func WithFromImage(image string) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if !utils.IsEmptyString(image) {
			opts.FromImage = image
		}
	}
}

// WithIsolation sets the isolation mode for the builder options.
func WithIsolation(isolation define.Isolation) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		opts.Isolation = isolation
	}
}

// WithCommonBuildOptions sets the common build options such as CPU and memory limits.
func WithCommonBuildOptions(options *buildah.CommonBuildOptions) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if options != nil {
			opts.CommonBuildOpts = options
		} else {
			opts.CommonBuildOpts = &buildah.CommonBuildOptions{}
		}
	}
}

// WithSystemContext sets the system context for the builder options.
func WithSystemContext(sysCtx *imageTypes.SystemContext) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		if sysCtx != nil {
			opts.SystemContext = sysCtx
		} else {
			opts.SystemContext = &imageTypes.SystemContext{}
		}
	}
}

// WithNetworkConfiguration sets the network configuration policy for the builder options.
func WithNetworkConfiguration(policy define.NetworkConfigurationPolicy) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		opts.ConfigureNetwork = policy
	}
}

// WithFormat sets the format for the container image to be committed.
func WithFormat(format string) func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		opts.Format = format
	}
}

// WithCapabilities Container 에서 Run 해줄려면 root 권한 줘야 함.
func WithCapabilities() func(*buildah.BuilderOptions) {
	return func(opts *buildah.BuilderOptions) {
		cap, _ := capabilities()
		opts.Capabilities = cap
	}
}

func capabilities() ([]string, error) {
	conf, err := config.Default()
	if err != nil {
		return nil, fmt.Errorf("failed to get default config: %w", err)
	}
	capabilitiesForRoot, err := conf.Capabilities("root", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get capabilities for root: %w", err)
	}

	return capabilitiesForRoot, nil
}

// NewBuilder creates a new Builder with the specified options
func NewBuilder(ctx context.Context, store storage.Store, opts ...func(*buildah.BuilderOptions)) (context.Context, *buildah.Builder, error) {
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

// NewStore creates and initializes a new storage.Store object
func NewStore() (storage.Store, error) {
	// Get default store options
	buildStoreOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		Log.Errorf("failed to get default store options: %v", err)
		return nil, err
	}
	// Check if running in rootless mode and using overlay driver
	if unshare.IsRootless() && buildStoreOptions.GraphDriverName == "overlay" {
		option := "overlay.mount_program=/usr/bin/fuse-overlayfs"
		// Add the overlay mount program option if it is not already present
		if !utils.Contains(buildStoreOptions.GraphDriverOptions, option) {
			buildStoreOptions.GraphDriverOptions = append(buildStoreOptions.GraphDriverOptions, option)
		}
	}
	// Get the storage store
	buildStore, err := storage.GetStore(buildStoreOptions)
	if err != nil {
		Log.Errorf("failed to get store: %v", err)
		return nil, err
	}
	return buildStore, nil
}

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

// WithChmod sets the Chmod option for AddAndCopyOptions.
func WithChmod(chmod string) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.Chmod = chmod
	}
}

// WithChown sets the Chown option for AddAndCopyOptions.
func WithChown(chown string) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.Chown = chown
	}
}

// WithPreserveOwnership sets the PreserveOwnership option for AddAndCopyOptions.
func WithPreserveOwnership(preserve bool) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.PreserveOwnership = preserve
	}
}

// WithHasher sets the Hasher option for AddAndCopyOptions.
func WithHasher(hasher io.Writer) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.Hasher = hasher
	}
}

// WithExcludes sets the Excludes option for AddAndCopyOptions.
func WithExcludes(excludes []string) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.Excludes = excludes
	}
}

// WithIgnoreFile sets the IgnoreFile option for AddAndCopyOptions.
func WithIgnoreFile(ignoreFile string) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.IgnoreFile = ignoreFile
	}
}

// WithContextDir sets the ContextDir option for AddAndCopyOptions.
func WithContextDir(contextDir string) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.ContextDir = contextDir
	}
}

// WithIDMappingOptions sets the IDMappingOptions option for AddAndCopyOptions.
func WithIDMappingOptions(idMappingOptions *define.IDMappingOptions) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.IDMappingOptions = idMappingOptions
	}
}

// WithDryRun sets the DryRun option for AddAndCopyOptions.
func WithDryRun(dryRun bool) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.DryRun = dryRun
	}
}

// WithStripSetuidBit sets the StripSetuidBit option for AddAndCopyOptions.
func WithStripSetuidBit(strip bool) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.StripSetuidBit = strip
	}
}

// WithStripSetgidBit sets the StripSetgidBit option for AddAndCopyOptions.
func WithStripSetgidBit(strip bool) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.StripSetgidBit = strip
	}
}

// WithStripStickyBit sets the StripStickyBit option for AddAndCopyOptions.
func WithStripStickyBit(strip bool) func(*buildah.AddAndCopyOptions) {
	return func(opts *buildah.AddAndCopyOptions) {
		opts.StripStickyBit = strip
	}
}

// NewAddAndCopyOptions creates a new AddAndCopyOptions with the specified options applied.
func NewAddAndCopyOptions(opts ...func(*buildah.AddAndCopyOptions)) buildah.AddAndCopyOptions {
	options := &buildah.AddAndCopyOptions{}
	for _, applyOpt := range opts {
		applyOpt(options)
	}
	return *options
}

// buildImageFromDockerfile builds an image from the provided Dockerfile
func buildImageFromDockerfile(ctx context.Context, dockerfilePath string) (context.Context, string, error) {
	// Define build options
	options := types.BuildOptions{
		BuildOptions: define.BuildOptions{
			ContextDirectory: ".",
			PullPolicy:       define.PullIfMissing,
			Isolation:        define.IsolationOCI,
			SystemContext:    &imageTypes.SystemContext{},
		},
		ContainerFiles: []string{dockerfilePath},
	}
	// Build the Dockerfile
	r, err := images.Build(ctx, options.ContainerFiles, options)
	if err != nil {
		return ctx, "", err
	}

	return ctx, r.ID, nil
}

// TODO 옵션 확인해보자.
// 아래 함수들을 따로 정리해서 모아두자.
func newBuilder(ctx context.Context, store storage.Store, idname string) (context.Context, *buildah.Builder, error) {
	return NewBuilder(ctx, store,
		WithFromImage(idname),
		WithIsolation(define.IsolationOCI),
		WithCommonBuildOptions(nil),
		WithSystemContext(nil),
		WithNetworkConfiguration(buildah.NetworkDefault),
		WithFormat(buildah.Dockerv2ImageManifest),
		WithCapabilities())
}

// newAddAndCopyOptions 컨테이너 내부에서 루트 권한으로 아래와 같이 실행 함.
func newAddAndCopyOptions() buildah.AddAndCopyOptions {
	return NewAddAndCopyOptions(
		WithChmod("0755"),
		WithChown("0:0"),
		WithHasher(digester.Hash()),
		WithContextDir("."),
		WithDryRun(false),
	)
}

type BuildConfig struct {
	HealthcheckDir   string
	SourceImageName  string
	ImageName        string
	ImageSavePath    string
	HealthcheckShell string
	DockerfilePath   string
	ExecutorShell    string ``
	UserScriptShell  string
	InstallShell     string
}

/*
// CreateImageWithDockerfile1 TODO: 수정 해야 함. 아직 최적화 하지 않음.
// TODO: 이건 Dockerfile 과 동일하게 나와야 함. => Dockerfile.alpine.executor
// TODO: 이 메서드는 개념이 혼재 되어 있는데 Dockerfile 을 사용자가 작성한 경우에 해당한다. 만약 사용자가 os 만 선택한 경우도 생각해야 한다. 이름을 CreateImageWithDockerfile 이라고 하자.
// TODO: 다른 건 CreateImage 라고 하자.
func CreateImageWithDockerfile1(ctx context.Context, store storage.Store, archive string, config BuildConfig) (*buildah.Builder, string, error) {
	// Dockerfile로부터 이미지를 빌드
	ctx, id, err := buildImageFromDockerfile(ctx, config.DockerfilePath)
	if err != nil {
		Log.Errorf("Failed to build image from Dockerfile: %v", err)
		return nil, "", err
	}

	// 새로운 빌더 생성
	ctx, builder, err := newBuilder(ctx, store, id)
	if err != nil {
		Log.Errorf("Failed to create new builder: %v", err)
		return nil, "", err
	}

	// /app/healthcheck 디렉토리 생성
	// TODO: User: "root" 이거 생각 해야 함. 루트리스와 루트 구분해줘야 하는데 이게 어떤 영향을 미치는 지 테스트 해봐야 함.
	err = builder.Run([]string{"mkdir", "-p", config.HealthcheckDir}, buildah.RunOptions{
		User:      "root",
		Isolation: define.IsolationOCI,
		Runtime:   "runc",
	})
	if err != nil {
		Log.Errorf("Failed to create %s directory: %v", config.HealthcheckDir, err)
		return builder, "", fmt.Errorf("failed to create %s directory: %w", config.HealthcheckDir, err)
	}
	// /app workdir 설정
	builder.SetWorkDir("/app")

	// healthChecker 파일 추가
	// ADD 로 했는데 COPY 로 바꾸자.
	options := newAddAndCopyOptions()
	err = builder.Add(config.HealthcheckDir, false, options, config.HealthcheckShell)
	if err != nil {
		Log.Errorf("Failed to add health checker to %s: %v", config.HealthcheckDir, err)
		return builder, "", fmt.Errorf("failed to add health checker to %s: %w", config.HealthcheckDir, err)
	}

	// 이미지 참조 생성
	// TODO: archive 같은 경우는 사용자의 user_script.sh 에 따라 이름을 바꿔야 할듯. os_스크립트이름 으로 하자.
	// TODO: 일단 해당 스크립트는 user_script.sh 를 변경시키는 것이 아닌 이미지를 변경시키는 것으로 한다.
	// TODO: docker.io 로 고정했는데 localhost 로 할지 다른 것로 할지 사용자가 정하도록 하자.
	imageRef, err := is.Transport.ParseReference("docker.io/" + archive)
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
	err = saveImage(ctx, config.ImageSavePath, "custom", "", imageId, false)
	if err != nil {
		Log.Errorf("Failed to save image: %v", err)
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}
*/

func CreateImageWithDockerfile2(ctx context.Context, store storage.Store, config BuildConfig) (*buildah.Builder, string, error) {
	// Dockerfile로부터 이미지를 빌드 (alpine 이미지 사용)
	ctx, id, err := buildImageFromDockerfile(ctx, config.DockerfilePath)
	if err != nil {
		Log.Errorf("Failed to build image from Dockerfile: %v", err)
		return nil, "", err
	}

	// 새로운 빌더 생성
	ctx, builder, err := newBuilder(ctx, store, id)
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
	//imageRef, err := is.Transport.ParseReference("docker.io/" + archive)
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, "", imageId, false)
	if err != nil {
		Log.Errorf("Failed to save image: %v", err)
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// CreateImageWithDockerfile TODO: 테스트 진행 해야 함.
// TODO: 여기서 사용된 ImageName 은 사용자에게 보여줘야 함. 하지만 노출은 하지 않음. DB 로 저장
// TODO: string 으로 저장한 것 config 로 빼자.
// TODO: buildImageFromDockerfile 테스트 진행
func CreateImageWithDockerfile(ctx context.Context, store storage.Store, config BuildConfig) (*buildah.Builder, string, error) {
	ctx, id, err := buildImageFromDockerfile(ctx, config.DockerfilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build image from Dockerfile: %w", err)
	}

	// 새로운 빌더 생성
	ctx, builder, err := newBuilder(ctx, store, id)
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
	// TODO: 해당 파일이 있는지 검사해야함.
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
	// TODO: 데이터가 없는 이미지를 만들때는 CMD 는 없어야 한다. 데이터가 들어간 이미지를 만들어 주는 메서드를 만들때 넣어준다.
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, "", imageId, false)
	if err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// CreateImage 기본 os 를 바탕으로 제작, 이러한 기본 os 같은 경우도 포함 될 utils 들에 대해서 생각해줘야 함.
func CreateImage(ctx context.Context, store storage.Store, config BuildConfig) (*buildah.Builder, string, error) {

	ctx, builder, err := newBuilder(ctx, store, config.SourceImageName)
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
		"/app":         {config.ExecutorShell, config.HealthcheckShell, config.InstallShell}, // 두 개의 스크립트를 같은 경로에 복사
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
	err = saveImage(ctx, config.ImageSavePath, config.ImageName, "", imageId, false)
	if err != nil {
		return builder, imageId, fmt.Errorf("failed to save image: %w", err)
	}

	return builder, imageId, nil
}

// 공통 디렉토리 생성 함수
func createDirectories(builder *buildah.Builder, dirs []string) error {
	for _, dir := range dirs {
		err := builder.Run([]string{"mkdir", "-p", dir}, buildah.RunOptions{
			User:      "root",
			Isolation: define.IsolationOCI,
			Runtime:   "runc",
		})
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// 스크립트 복사 함수
func copyScripts(builder *buildah.Builder, scripts map[string][]string) error {
	options := newAddAndCopyOptions()
	for dest, srcList := range scripts {
		for _, src := range srcList {
			err := builder.Add(dest, false, options, src)
			if err != nil {
				return fmt.Errorf("failed to copy script %s to %s: %w", src, dest, err)
			}
		}
	}
	return nil
}

// 파일 권한 설정 함수
func setFilePermissions(builder *buildah.Builder, files []string) error {
	chmodArgs := append([]string{"chmod", "777"}, files...)
	err := builder.Run(chmodArgs, buildah.RunOptions{
		User:      "root",
		Isolation: define.IsolationOCI,
		Runtime:   "runc",
	})
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	return nil
}

// install.sh 실행
func installDependencies(builder *buildah.Builder) error {
	chmodArgs := []string{"/app/install.sh"}
	err := builder.Run(chmodArgs, buildah.RunOptions{
		User:      "root",
		Isolation: define.IsolationOCI,
		Runtime:   "runc",
	})
	if err != nil {
		return fmt.Errorf("failed to run install.sh: %w", err)
	}
	return nil
}

// saveImage TODO defer 에서 error 리턴하는 것 생각하기
func saveImage(ctx context.Context, path, imageName, imageTag, imageId string, compress bool) error {
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
	defer outputFile.Close()

	var writer io.Writer = outputFile

	if compress {
		// gzip.Writer를 사용하여 데이터를 압축
		gzipWriter := gzip.NewWriter(outputFile)
		defer func() {
			if err := gzipWriter.Close(); err != nil {
				Log.Errorf("Failed to close gzip writer: %v", err)
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

// Run 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func (b *Builder) Run(s string) error {

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
}*/

// WorkDir 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func (b *Builder) WorkDir(path string) error {
	if utils.IsEmptyString(path) {
		return fmt.Errorf("path is empty")
	}
	b.builder.SetWorkDir(path)
	return nil
}*/

// Env 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func (b *Builder) Env(k, v string) error {
	if utils.IsEmptyString(k) || utils.IsEmptyString(v) {
		return fmt.Errorf("key or valeu is empty")
	}

	b.builder.SetEnv(k, v)
	return nil
}*/

// User 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func (b *Builder) User(u string) error {
	if utils.IsEmptyString(u) {
		return fmt.Errorf("user is empty")
	}

	b.builder.SetUser(u)
	return nil
}*/

// Expose 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func (b *Builder) Expose(port string) error {
	if utils.IsEmptyString(port) {
		return fmt.Errorf("port is empty")
	}
	b.builder.SetPort(port)
	return nil
}*/

// Cmd 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func (b *Builder) Cmd(cmd ...string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command is empty")
	}
	b.builder.SetCmd(cmd)
	return nil
}*/

// CommitImage 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func (b *Builder) CommitImage(ctx context.Context, preferredManifestType string, sysCtx *imageTypes.SystemContext, repository string) (*string, error) {

	imageRef, err := is.Transport.ParseStoreReference(b.store, repository)
	if err != nil {
		return nil, err
	}

	imageId, _, _, err := b.builder.Commit(ctx, imageRef, buildah.CommitOptions{
		PreferredManifestType: preferredManifestType,
		SystemContext:         sysCtx,
	})

	return &imageId, err
}*/

// GetLoggerWriter 사용하지 않음. 삭제하지 않고 주석 처리함.
/*func GetLoggerWriter() io.Writer {
	if Verbose || Debug {
		return os.Stdout
	} else {
		return NopLogger{}
	}
}

type NopLogger struct{}

func (n NopLogger) Write(p []byte) (int, error) {
	return len(p), nil
}*/
