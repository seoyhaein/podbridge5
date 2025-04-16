package podbridge5

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	entitiesTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	"io"
	"log"
	"time"
)

// 주의 사항. 여기서 volumeName 생성이 전제되어 있다고 봐야 한다.

// WithNamedVolume 네임드 볼륨 마운트 옵션
func WithNamedVolume(volumeName, dest, subPath string, options ...string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		// volumeName 이 비어있으면 익명 볼륨(anonymous volume)으로 처리
		isAnonymous := false
		// TODO 아래 내용 확인 해야함.
		// 익명 보륨의 경우,일반적으로 익명 볼륨은 특정 컨테이너에 종속되어 사용되며, 해당 컨테이너가 삭제되면 함께 정리 되어야함.
		// podman rm -v 이러한 방식으로 볼륨을 지울 수 있음.
		if volumeName == "" {
			isAnonymous = true
		}

		// 새로운 NamedVolume 객체 생성 (SubPath 필드 포함)
		newVol := &specgen.NamedVolume{
			Name:        volumeName,
			Dest:        dest,
			Options:     options,
			IsAnonymous: isAnonymous,
			SubPath:     subPath,
		}

		// 이미 같은 Dest 설정되어 있는지 확인
		for _, vol := range spec.Volumes {
			if vol.Dest == dest {
				// 같은 Dest 이미 설정되어 있으면 덮어쓰지 않고 그냥 반환
				return nil
			}
		}

		// SpecGenerator 의 Volumes 필드에 새 볼륨 추가
		spec.Volumes = append(spec.Volumes, newVol)
		return nil
	}
}

// CreateVolume 주어진 볼륨 이름을 기반으로 볼륨 만들어줌. ignoreIfExists true 이면, 동일한 볼륨이 있으면 에러 리턴하지 않고 그대로 사용.
func CreateVolume(ctx context.Context, volumeName string, ignoreIfExists bool) (*entitiesTypes.VolumeConfigResponse, error) {
	volConfig := entitiesTypes.VolumeCreateOptions{
		Name:           volumeName,
		IgnoreIfExists: ignoreIfExists, // 만약 true 이면, 동일한 이름의 볼륨이 있으면 생성하지 않고 기존 볼륨을 사용
	}

	// CreateOptions 객체, 현재 버전에서는 빈 객체임.
	createOptions := &volumes.CreateOptions{}

	// volumes.Create 함수를 호출하여 볼륨 생성
	volumeResp, err := volumes.Create(ctx, volConfig, createOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	return volumeResp, nil
}

func DeleteVolume(ctx context.Context, volumeName string, force *bool) error {
	exists, err := volumes.Exists(ctx, volumeName, &volumes.ExistsOptions{})
	if err != nil {
		return fmt.Errorf("failed to check if volume %q exists: %w", volumeName, err)
	}
	if !exists {
		return fmt.Errorf("volume %q does not exist", volumeName)
	}

	opts := &volumes.RemoveOptions{
		Force: force,
	}

	if err := volumes.Remove(ctx, volumeName, opts); err != nil {
		return fmt.Errorf("failed to remove volume %q: %w", volumeName, err)
	}

	return nil
}

func WriteDataToVolume(ctx context.Context, volumeName, mountPath, fileName string, data []byte) error {
	// 1. Create (or reuse) the volume.
	vcr, err := CreateVolume(ctx, volumeName, false)
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	// 2. Build the container specification.
	// TODO: 추후 이미지 교체 필요.
	spec, err := NewSpec(
		WithImageName("docker.io/library/alpine:latest"),
		WithName("temp-data-writer"),
		WithCommand([]string{"sh", "-c", "mkdir -p /data && sleep infinity"}),
		WithNamedVolume(vcr.Name, mountPath, ""),
	)
	if err != nil {
		return fmt.Errorf("failed to build container spec: %w", err)
	}

	// 3. Check if the image exists; if not, pull it.
	imageExists, err := images.Exists(ctx, spec.Image, nil)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	}
	if !imageExists {
		log.Printf("Pulling image %s...", spec.Image)
		if _, err := images.Pull(ctx, spec.Image, &images.PullOptions{}); err != nil {
			return fmt.Errorf("failed to pull image: %w", err)
		}
	}

	// 4. Create the temporary container.
	createResp, err := containers.CreateWithSpec(ctx, spec, nil)
	if err != nil {
		return fmt.Errorf("failed to create temporary container: %w", err)
	}
	containerID := createResp.ID

	// Ensure the container is stopped and removed afterward.
	defer func() {
		containers.Stop(ctx, containerID, nil)
		containers.Remove(ctx, containerID, nil)
	}()

	// 5. Start the container.
	if err := containers.Start(ctx, containerID, nil); err != nil {
		return fmt.Errorf("failed to start temporary container: %w", err)
	}

	// 6. Prepare a tar archive in memory with the file data.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: fileName, // File will be extracted under the mountPath in the container.
		Mode: 0644,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("failed to write tar content: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	// 7. Use the CopyFromArchiveWithOptions API to copy the tar archive into the container.
	copyFunc, err := containers.CopyFromArchiveWithOptions(ctx, containerID, mountPath, &buf, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize copy process: %w", err)
	}
	if err := copyFunc(); err != nil {
		return fmt.Errorf("failed to copy archive into container: %w", err)
	}

	// Optionally, wait a short period for file extraction to complete.
	time.Sleep(2 * time.Second)
	return nil
}

// ReadDataFromVolume mounts the given volume in a temporary container,
// copies the specified file as a tar archive from the container,
// and returns the content of the file.
func ReadDataFromVolume(ctx context.Context, volumeName, mountPath, fileName string) (string, error) {
	// 1. Build the container specification.
	spec, err := NewSpec(
		WithImageName("docker.io/library/alpine:latest"),
		WithName("temp-data-reader"),
		WithCommand([]string{"sh", "-c", "mkdir -p /data && sleep infinity"}),
		WithNamedVolume(volumeName, mountPath, ""),
	)
	if err != nil {
		return "", fmt.Errorf("failed to build container spec: %w", err)
	}

	// 2. Check if the image exists; if not, pull it.
	imageExists, err := images.Exists(ctx, spec.Image, nil)
	if err != nil {
		return "", fmt.Errorf("failed to check if image %q exists: %w", spec.Image, err)
	}
	if !imageExists {
		log.Printf("Pulling image %s...", spec.Image)
		if _, err := images.Pull(ctx, spec.Image, &images.PullOptions{}); err != nil {
			return "", fmt.Errorf("failed to pull image %q: %w", spec.Image, err)
		}
	}

	// 3. Create the temporary container.
	createResp, err := containers.CreateWithSpec(ctx, spec, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary container: %w", err)
	}
	containerID := createResp.ID

	// Ensure container cleanup.
	defer func() {
		containers.Stop(ctx, containerID, nil)
		containers.Remove(ctx, containerID, nil)
	}()

	// 4. Start the container.
	if err := containers.Start(ctx, containerID, nil); err != nil {
		return "", fmt.Errorf("failed to start temporary container: %w", err)
	}

	// 5. Build the full file path inside the container.
	fullPath := mountPath + "/" + fileName

	// Prepare a buffer to receive the tar archive.
	var outBuf bytes.Buffer

	// 6. Use the CopyToArchive API to fetch the file as a tar archive.
	copyFunc, err := containers.CopyToArchive(ctx, containerID, fullPath, &outBuf)
	if err != nil {
		return "", fmt.Errorf("failed to initialize copy from container: %w", err)
	}
	if err := copyFunc(); err != nil {
		return "", fmt.Errorf("failed during copy from container: %w", err)
	}

	// 7. Extract file content from the tar archive.
	tr := tar.NewReader(&outBuf)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // 아카이브의 끝에 도달
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar archive: %w", err)
		}
		// tar 헤더의 Name이 파일 이름과 일치하면 파일 내용을 읽음.
		if hdr.Name == fileName {
			var fileBuf bytes.Buffer
			if _, err := io.Copy(&fileBuf, tr); err != nil {
				return "", fmt.Errorf("failed to read file data from tar: %w", err)
			}
			return fileBuf.String(), nil
		}
	}

	return "", fmt.Errorf("file %q not found in tar archive", fileName)
}
