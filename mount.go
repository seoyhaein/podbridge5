package podbridge5

import (
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/storage/pkg/unshare"
	specgo "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
	"os"
	"path"
	"syscall"
)

// WithMount 컨테이너에서 bind mount 같은 경우 마운트할 디렉토리가 없으면 자동으로 만들어줌. 또한 stop 이나 remove 하면 자동으로 unmount 됨.
func WithMount(source, destination, mountType string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		// 1) 호스트 경로가 실제로 있는지 검사
		if fi, err := os.Stat(source); err != nil {
			return fmt.Errorf("host path %q does not exist: %w", source, err)
		} else if !fi.IsDir() {
			return fmt.Errorf("host path %q exists but is not a directory", source)
		}
		// 2) spec.Mounts 초기화
		if spec.Mounts == nil {
			spec.Mounts = []specgo.Mount{}
		}
		// 3) mountType("bind" 등)과 source, destination 설정
		spec.Mounts = append(spec.Mounts, specgo.Mount{
			Type:        mountType,   // "bind"
			Source:      source,      // 호스트 경로
			Destination: destination, // 컨테이너 내부 경로
			Options:     []string{"ro"},
		})
		return nil
	}
}

// MountOverlay mounts an OverlayFS at mergedDir, using lowerDir as read-only data
// and upperDir for writable data, with workDir for internal overlay operations.
// It handles both root and rootless environments, attempting native overlay in rootless
// and falling back to fuse-overlayfs if needed.
func MountOverlay(lowerDir, upperDir, workDir, mergedDir string) error {
	// 1. Ensure all necessary directories exist
	for _, dir := range []string{lowerDir, upperDir, workDir, mergedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// 2. Prepare mount options
	baseOpts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)

	// 3. Handle root vs rootless cases
	if !unshare.IsRootless() {
		// --- Root Case ---
		Log.Infoln("Running as root, using native 'overlay'.")
		if err := unix.Mount("overlay", mergedDir, "overlay", 0, baseOpts); err != nil {
			return fmt.Errorf("failed to mount overlayfs as root: %w", err)
		}
		return nil
	}

	// --- Rootless Case ---
	Log.Infoln("Running as rootless, attempting native 'overlay' first.")
	err := unix.Mount("overlay", mergedDir, "overlay", 0, baseOpts)
	if err == nil {
		Log.Infoln("Successfully mounted with native rootless 'overlay'.")
		return nil
	}

	// 4. Fallback on expected errors for older kernels
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EINVAL) {
		Log.Infof("Native rootless mount failed (expected on older kernels): %v. Falling back to 'fuse-overlayfs'.", err)
		fuseOpts := baseOpts + ",mount_program=/usr/bin/fuse-overlayfs"
		if fErr := unix.Mount("overlay", mergedDir, "overlay", 0, fuseOpts); fErr != nil {
			return fmt.Errorf("fallback mount with 'fuse-overlayfs' failed: %w", fErr)
		}
		Log.Infoln("Successfully mounted with fallback 'fuse-overlayfs'.")
		return nil
	}

	// Unexpected error
	return fmt.Errorf("native rootless overlayfs mount failed unexpectedly: %w", err)
}

// WithFileBindings mounts each host file from cellColumns into the container
// under bindDir using the header name as the filename, and sets an environment
// variable <HEADER> pointing to that path.
// 수정해야함.
// 사용자는 flock_id 를 통해서 위치를 알 수 있음, 그리고 column_headers 이름을 통해서, cell_columns 의 값 즉 파일이름을 알 수 있음.
// 사용자가 주는 것은 sample 이름, 이고 flock 정보를 줄 수 있음. 이정보를 가지고 바인딩 해야 하는데..
// 이정보를 proto 정보에 만들어야 겠네. 서버에는 이미 datablcok 있으므로 flock_id 만 있으면 되고, 샘플 이름들만 알면 되네. 사용자가 헤더 이름을 작성하나? 작성할 필요 없음. 이미 저장되어 있음으로.
// 사용자가 헤더이름으로 작성하지 않으면 어차피 찾아 지지도 않음. 에러 처리해야함. 따라서 flock_id 만 알고 있으면 됨. flock_id 를 통해서 사용자가 어떠한 파일블럭을 선택했는지 알게 됨. 그리고 파일도 알게 됨.
// 시스템은 flock_id 를 가지고 샘플이름(헤더이름), 실제 파일이름 및 위치 까지 알게 됨. 이걸 row 에 맞게 해주면 됨.
// 별도의 메서드가 필요함. 바인딩은 저 위 메서드에서도 가능함.
// flock_id 가 인풋으로 들어가면, datablock 에서 샘플 이름하고, 실제 파일 이름이 나오면 되는 형태임.
// 실제 파이프라인도 받아야 하지만, 이 단계에서는 그냥 flock_id 만 받아서 컨테이너들을 만드는 것을 하고, 테스트 쉘 스크립트를 만드는 걸로 하면 될듯.
func WithFileBindings(bindDir string, cellColumns map[string]string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		// Ensure the working directory is set
		spec.WorkDir = bindDir

		// Mount each file and export as env var
		for header, hostPath := range cellColumns {
			// Destination inside container: bindDir/<header>
			dest := path.Join(bindDir, header)
			// Bind mount as read-only
			mount := specgo.Mount{
				Source:      hostPath,
				Destination: dest,
				Options:     []string{"ro"},
			}
			spec.Mounts = append(spec.Mounts, mount)
			// Set environment variable <HEADER> equal to the container file path
			// This allows shell scripts to use $HEADER to access the mounted file
			spec.Env[header] = dest
		}
		return nil
	}
}
