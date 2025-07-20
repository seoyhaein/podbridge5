package podbridge5

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type VolumeMode int

const (
	// ModeSkip 기존 데이터가 있으면 아무 작업도 하지 않고 바로 리턴
	ModeSkip VolumeMode = iota
	// ModeUpdate 기존 데이터를 유지하되, tar 안의 파일로 “업데이트”(덮어쓰기)만 수행
	ModeUpdate
	// ModeOverwrite 기존 볼륨을 완전 초기화(삭제 → 새로 생성)한 뒤 tar 를 풀어 씀
	ModeOverwrite
)

func WithNamedVolume(volumeName, dest, subPath string, options ...string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		cleaned := strings.TrimSpace(volumeName)
		if cleaned == "" {
			return fmt.Errorf("WithNamedVolume: empty volumeName for dest %s", dest)
		}

		for _, vol := range spec.Volumes {
			if vol.Dest == dest {
				if vol.Name != cleaned {
					return fmt.Errorf("WithNamedVolume: dest %s already mapped to %s", dest, vol.Name)
				}
				return nil
			}
		}

		spec.Volumes = append(spec.Volumes, &specgen.NamedVolume{
			Name:        cleaned,
			Dest:        dest,
			Options:     options,
			IsAnonymous: false,
			SubPath:     subPath, // 필요 없으면 먼저 제거해 테스트
		})
		return nil
	}
}

// TODO nfs, lustre 로 volume 을 원격지에 둘경우 대응해줘야 함. 지금은 local 만 해줌

// CreateVolume 주어진 볼륨 이름을 기반으로 볼륨 만들어줌. ignoreIfExists true 이면, 동일한 볼륨이 있으면 에러 리턴하지 않고 그대로 사용.
func CreateVolume(ctx context.Context, volumeName string, ignoreIfExists bool) (*types.VolumeConfigResponse, error) {
	volConfig := types.VolumeCreateOptions{
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

// WriteFolderToVolume TODO 일단 테스트 필요 일단 붙이면서 보자.
func WriteFolderToVolume(parentCtx context.Context, volumeName, mountPath, hostDir string, mode VolumeMode) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// 0. hostDir 검증
	st, err := os.Stat(hostDir)
	if err != nil {
		return fmt.Errorf("WriteFolderToVolume: stat hostDir: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("WriteFolderToVolume: hostDir is not directory: %s", hostDir)
	}

	var vcr *types.VolumeConfigResponse

	switch mode {
	case ModeOverwrite:
		vcr, err = OverwriteVolume(ctx, volumeName, func(c context.Context, n string) (*types.VolumeConfigResponse, error) {
			return CreateVolume(c, n, false)
		})
		if err != nil {
			return fmt.Errorf("WriteFolderToVolume: overwrite volume setup: %w", err)
		}

	case ModeSkip:
		exists, err := VolumeExists(ctx, volumeName)
		if err != nil {
			return fmt.Errorf("WriteFolderToVolume: check existence (skip): %w", err)
		}
		if exists {
			return nil
		}
		vcr, err = CreateVolume(ctx, volumeName, false)
		if err != nil {
			return fmt.Errorf("WriteFolderToVolume: create volume (skip path): %w", err)
		}

	case ModeUpdate:
		exists, err := VolumeExists(ctx, volumeName)
		if err != nil {
			return fmt.Errorf("WriteFolderToVolume: check existence (update): %w", err)
		}
		if !exists {
			vcr, err = CreateVolume(ctx, volumeName, false)
			if err != nil {
				return fmt.Errorf("WriteFolderToVolume: create volume (update path): %w", err)
			}
		} else {
			// 기존 볼륨 재사용
			vcr = &types.VolumeConfigResponse{}
			vcr.Name = volumeName
			// 필요하면 Inspect:
			// vcr, _ = volumes.Inspect(ctx, volumeName, nil)
		}

	default:
		return fmt.Errorf("WriteFolderToVolume: unknown mode: %d", mode)
	}

	if vcr == nil {
		vcr = &types.VolumeConfigResponse{}
		vcr.Name = volumeName
	}

	// 1. 임시 컨테이너 spec
	spec, err := NewSpec(
		WithImageName("docker.io/library/alpine:latest"),
		WithName("temp-folder-writer"),
		WithCommand([]string{
			"sh", "-c",
			"mkdir -p \"$1\"; exec sleep infinity",
			"sh", mountPath,
		}),
		WithNamedVolume(vcr.Name, mountPath, ""),
	)
	if err != nil {
		return fmt.Errorf("WriteFolderToVolume3: build container spec: %w", err)
	}

	// 2. 이미지 확인/풀
	ok, err := images.Exists(ctx, spec.Image, nil)
	if err != nil {
		return fmt.Errorf("WriteFolderToVolume: image exists check: %w", err)
	}
	if !ok {
		if _, err := images.Pull(ctx, spec.Image, &images.PullOptions{}); err != nil {
			return fmt.Errorf("WriteFolderToVolume: image pull: %w", err)
		}
	}

	// 3. 컨테이너 생성 & 시작
	createResp, err := containers.CreateWithSpec(ctx, spec, nil)
	if err != nil {
		return fmt.Errorf("WriteFolderToVolume: container create: %w", err)
	}
	containerID := createResp.ID
	defer func() {
		if stopErr := containers.Stop(ctx, containerID, nil); stopErr != nil {
			Log.Warnf("stop container %s: %v", containerID, stopErr)
		}
		if _, rmErr := containers.Remove(ctx, containerID, nil); rmErr != nil {
			Log.Warnf("remove container %s: %v", containerID, rmErr)
		}
	}()
	if err := containers.Start(ctx, containerID, nil); err != nil {
		return fmt.Errorf("WriteFolderToVolume: container start: %w", err)
	}

	// 4. tar 스트리밍 (WalkDir)
	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		tw := tar.NewWriter(pw)
		defer func() {
			if cerr := tw.Close(); cerr != nil {
				Log.Warnf("tar writer close: %v", cerr)
			}
		}()

		var walkErr error
		defer func() {
			if walkErr != nil {
				_ = pw.CloseWithError(walkErr)
			} else {
				_ = pw.Close()
			}
		}()

		walkErr = filepath.WalkDir(hostDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(hostDir, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}

			fi, err := d.Info()
			if err != nil {
				return err
			}

			var linkTarget string
			if fi.Mode()&os.ModeSymlink != 0 {
				if lt, lerr := os.Readlink(path); lerr == nil {
					linkTarget = lt
				} else {
					return lerr
				}
			}

			hdr, err := tar.FileInfoHeader(fi, linkTarget)
			if err != nil {
				return err
			}
			hdr.Name = rel

			if d.IsDir() {
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				return nil
			}

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

			if fi.Mode().IsRegular() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				_, cErr := io.Copy(tw, f)
				_ = f.Close()
				if cErr != nil {
					return cErr
				}
			}
			return nil
		})
	}()

	// 5. tar -> 컨테이너 (mountPath)
	copyFunc, err := containers.CopyFromArchiveWithOptions(ctx, containerID, mountPath, pr, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("WriteFolderToVolume: init copy: %w", err)
	}
	if err := copyFunc(); err != nil {
		cancel()
		return fmt.Errorf("WriteFolderToVolume: copy archive: %w", err)
	}

	wg.Wait()

	// 6. 안정성 대기 (옵션)
	time.Sleep(500 * time.Millisecond)
	return nil
}

// ReadDataFromVolume TODO 이거 생각해보자. 필요한지
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
