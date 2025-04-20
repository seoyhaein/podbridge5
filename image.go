package podbridge5

import (
	"compress/gzip"
	"context"
	"fmt"
	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/common/pkg/config"
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
	"strings"
)

var (
	digester = digest.Canonical.Digester()

	defaultRunOptions = buildah.RunOptions{
		User:      "root",
		Isolation: define.IsolationOCI,
		Runtime:   "runc",
	}

	// 사용하지 않음 주석처리함. 삭제하지 않음.
	/*	Verbose = true
		Debug   = true*/
)

// ------------------------------------------------------
// Functional Options for buildah.BuilderOptions
// ------------------------------------------------------

type BuilderOption func(*buildah.BuilderOptions) error

// WithArg sets an argument for the build. 함수 수정: 에러 발생 시 이를 반환
func WithArg(key, value string) BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		if opts.Args == nil {
			opts.Args = make(map[string]string)
		}
		if _, ok := opts.Args[key]; !ok {
			opts.Args[key] = value
		}
		return nil
	}
}

// WithFromImage sets the base image for the build. 함수 수정: 에러 발생 시 이를 반환
func WithFromImage(image string) BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		if utils.IsEmptyString(image) {
			return fmt.Errorf("from image cannot be empty")
		}
		opts.FromImage = image
		return nil
	}
}

// WithIsolation sets the isolation mode for the builder options. 함수 수정: 에러 발생 시 이를 반환
func WithIsolation(isolation define.Isolation) BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		opts.Isolation = isolation
		return nil
	}
}

// WithCommonBuildOptions sets the common build options such as CPU and memory limits. 함수 수정: 에러 발생 시 이를 반환
// TODO 확인하자.
func WithCommonBuildOptions(options *buildah.CommonBuildOptions) BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		if options != nil {
			opts.CommonBuildOpts = options
		} else {
			opts.CommonBuildOpts = &buildah.CommonBuildOptions{}
		}
		return nil
	}
}

// WithSystemContext sets the system context for the builder options. 함수 수정: 에러 발생 시 이를 반환
// TODO 확인하자.
func WithSystemContext(sysCtx *imageTypes.SystemContext) BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		if sysCtx != nil {
			opts.SystemContext = sysCtx
		} else {
			opts.SystemContext = &imageTypes.SystemContext{}
		}
		return nil
	}
}

// WithNetworkConfiguration sets the network configuration policy for the builder options. 함수 수정: 에러 발생 시 이를 반환
func WithNetworkConfiguration(policy define.NetworkConfigurationPolicy) BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		opts.ConfigureNetwork = policy
		return nil
	}
}

// WithFormat sets the format for the container image to be committed. 함수 수정: 에러 발생 시 이를 반환
func WithFormat(format string) BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		opts.Format = format
		return nil
	}
}

// WithCapabilities sets capabilities needed for running as root in a container. 함수 수정: 에러 발생 시 이를 반환
func WithCapabilities() BuilderOption {
	return func(opts *buildah.BuilderOptions) error {
		caps, err := capabilities()
		if err != nil {
			return fmt.Errorf("failed to get capabilities: %w", err)
		}
		opts.Capabilities = caps
		return nil
	}
}

// capabilities returns the default capabilities for root.
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

// ------------------------------------------------------
// Core Builder/Store Functions
// ------------------------------------------------------

// NewBuilder creates a new Builder with the specified options, 함수 수정: 각 옵션을 적용할 때 에러를 확인
func NewBuilder(ctx context.Context, store storage.Store, opts ...BuilderOption) (*buildah.Builder, error) {
	builderOpts := &buildah.BuilderOptions{}
	for _, applyOpt := range opts {
		if err := applyOpt(builderOpts); err != nil {
			return nil, err
		}
	}
	builder, err := buildah.NewBuilder(ctx, store, *builderOpts)
	if err != nil {
		return nil, err
	}
	return builder, nil
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

// shutdown force 를 true 로 잡아주면 다른 컨테이너에게도 영향을 줄 수 있음.
// 기본적으로 false 를 유지하도록 하고, 모든 컨테이너가 종료되어 다른 레이어를 사용하지 않는다면 true 로 해줄 수 있음.
func shutdown(store storage.Store, force bool) error {
	if store == nil {
		return fmt.Errorf("storage.Store is nil")
	}
	_, err := store.Shutdown(force)
	if err != nil {
		return fmt.Errorf("Failed to shutdown store: %v\n", err)
	}
	return nil
}

// ------------------------------------------------------
// Functional Options for buildah.AddAndCopyOptions
// ------------------------------------------------------

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

// ------------------------------------------------------
// Image Build Helper Functions
// ------------------------------------------------------
// TODO dockerfilePath 관점에서 ContextDirectory 생각해봐야 한다. 실제 caleb 적용시 수정될 수 있음. (중요)
// buildImageFromDockerfile builds an image from the provided Dockerfile
func buildImageFromDockerfile(ctx context.Context, dockerfilePath string) (string, error) {
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
		return "", err
	}

	return r.ID, nil
}

// newBuilder creates a new builder using the NewBuilder function with default options.
// TODO 좀더 study 필요. 옵션들에 대해서.
func newBuilder(ctx context.Context, store storage.Store, idName string) (*buildah.Builder, error) {
	return NewBuilder(ctx, store,
		WithFromImage(idName),
		WithIsolation(define.IsolationOCI),
		WithCommonBuildOptions(nil),
		WithSystemContext(nil),
		WithNetworkConfiguration(buildah.NetworkDefault),
		WithFormat(buildah.Dockerv2ImageManifest),
		WithCapabilities())
}

// newAddAndCopyOptions creates default add and copy options.
func newAddAndCopyOptions() buildah.AddAndCopyOptions {
	return NewAddAndCopyOptions(
		WithChmod("0755"),
		WithChown("0:0"),
		WithHasher(digester.Hash()),
		WithContextDir("."),
		WithDryRun(false),
	)
}

// ------------------------------------------------------
// Helper Functions for Image Building
// ------------------------------------------------------

// createDirectories creates directories inside the builder.
func createDirectories(builder *buildah.Builder, dirs []string) error {
	for _, dir := range dirs {
		err := builder.Run([]string{"mkdir", "-p", dir}, defaultRunOptions)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// setFilePermissions sets file permissions using chmod.
func setFilePermissions(builder *buildah.Builder, files []string) error {
	chmodArgs := append([]string{"chmod", "777"}, files...)
	err := builder.Run(chmodArgs, defaultRunOptions)
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	return nil
}

// installDependencies runs the install.sh script.
func installDependencies(builder *buildah.Builder) error {
	chmodArgs := []string{"/app/install.sh"}
	err := builder.Run(chmodArgs, defaultRunOptions)
	if err != nil {
		return fmt.Errorf("failed to run install.sh: %w", err)
	}
	return nil
}

// copyScripts copies scripts to the specified destination directories.
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

// saveImage saves the built image to an archive file. TODO 파일 읽는 부분 살펴봐야 함. outputFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
func saveImage(ctx context.Context, path, imageName, imageId string, compress bool) error {
	// imageName 이미 태그를 포함한 완전한 이름이어야 함
	// 예: "docker.io/library/alpine-internal:latest"

	// 압축 여부에 따른 파일 확장자 설정
	extension := ".tar"
	if compress {
		extension = ".tar.gz"
	}

	// imageName 에서 마지막 구성 요소를 추출
	// 예: "docker.io/library/alpine-internal:latest" -> "alpine-internal:latest"
	baseImage := filepath.Base(imageName)
	// 파일명에 콜론(:)은 문제가 될 수 있으므로 하이픈(-)으로 치환
	safeImageName := strings.ReplaceAll(baseImage, ":", "-")
	// 파일명 생성: safeImageName + 확장자
	archiveFileName := fmt.Sprintf("%s%s", safeImageName, extension)
	// 입력받은 path 에 바로 결합 (불필요한 디렉토리 구조가 생성되지 않도록)
	archivePath := filepath.Join(path, archiveFileName)

	// archive 파일이 위치할 디렉토리 생성
	dir := filepath.Dir(archivePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// 출력 파일 생성 (명시적으로 파일 권한 설정)
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
		// gzip.Writer 사용하여 데이터를 압축
		gzipWriter := gzip.NewWriter(outputFile)
		defer func() {
			if zCerr := gzipWriter.Close(); zCerr != nil {
				Log.Errorf("Failed to close gzip writer: %v", zCerr)
			}
		}()
		writer = gzipWriter
	}
	// layer 를 별도로 압축을 할 수 있음.
	exportOptions := &images.ExportOptions{
		// 필요한 경우 추가 옵션을 설정
	}

	if err := images.Export(ctx, []string{imageId}, writer, exportOptions); err != nil {
		return fmt.Errorf("failed to export image %s: %w", imageId, err)
	}
	return nil
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
