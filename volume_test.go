package podbridge5

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCreateAndDeleteVolume tests both the creation and deletion of a volume.
func TestCreateAndDeleteVolume(t *testing.T) {
	// 새로운 Podman 연결 생성 (Linux 환경용 예제)
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	// 고유 볼륨 이름 생성.
	volumeName := fmt.Sprintf("test-volume-%s", uuid.New().String())

	// 1. ignoreIfExists false 로 볼륨 생성 테스트.
	vResp, err := CreateVolume(ctx, volumeName, false)
	if err != nil {
		t.Fatalf("CreateVolume failed: %v", err)
	}
	if vResp == nil {
		t.Fatalf("Expected non-nil VolumeConfigResponse")
	}
	if vResp.Name != volumeName {
		t.Errorf("Volume name mismatch: expected %q, got %q", volumeName, vResp.Name)
	}

	// 2. 생성된 볼륨이 실제로 존재하는지 확인.
	exists, err := volumes.Exists(ctx, volumeName, &volumes.ExistsOptions{})
	if err != nil {
		t.Fatalf("Failed to check volume existence: %v", err)
	}
	if !exists {
		t.Fatalf("Volume %q should exist after creation", volumeName)
	}

	// 3. DeleteVolume 함수 호출 (force=false)
	force := false
	if err := DeleteVolume(ctx, volumeName, &force); err != nil {
		t.Fatalf("Failed to delete volume %q: %v", volumeName, err)
	}

	// 잠시 대기하여 삭제가 완료되었는지 확인.
	time.Sleep(1 * time.Second)

	// 4. 삭제 후, 볼륨이 존재하지 않는지 확인.
	exists, err = volumes.Exists(ctx, volumeName, &volumes.ExistsOptions{})
	if err != nil {
		t.Fatalf("Failed to check volume existence after deletion: %v", err)
	}
	if exists {
		t.Fatalf("Volume %q should be deleted", volumeName)
	}

	// 5. 이미 삭제된 볼륨을 삭제하려고 하면 에러가 발생해야 함.
	err = DeleteVolume(ctx, volumeName, &force)
	if err == nil {
		t.Fatalf("Expected an error when deleting non-existent volume %q", volumeName)
	}
}

// 헬퍼: 볼륨이 있으면 삭제
func ensureVolumeDeleted(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	exists, err := volumes.Exists(ctx, name, &volumes.ExistsOptions{})
	if err != nil {
		t.Fatalf("check volume exists: %v", err)
	}
	if !exists {
		return
	}
	force := false
	if err := DeleteVolume(ctx, name, &force); err != nil {
		t.Fatalf("delete existing volume %q: %v", name, err)
	}
	// 약간의 지연(스토리지 정리)
	time.Sleep(500 * time.Millisecond)
}

// 헬퍼: 임시 컨테이너 없이 단순히 “데이터가 잘 들어갔는지” 확인하기 위해
//   - 작은 임시 컨테이너를 만들고 mountPath 로부터 tar 를 stdout 으로 얻는다.
//
// 여기서는 최소 범위라서 간이 버전으로 구성 (재사용 범용 헬퍼가 있으면 그걸 쓰면 됨).
func readAllFilesFromVolume(t *testing.T, ctx context.Context, volumeName, mountPath string) (map[string]string, error) {
	t.Helper()

	// 임시로 WriteDataToVolume / ReadDataFromVolume 과 동일한 컨테이너 기법 재사용 가능
	// 여기서는 별도 유틸이 없다고 가정하고, 이미 당신이 갖고 있을 법한
	// ‘컨테이너 만들어 tar 로 읽어오기’ 유사 루틴을 간략화해도 됨.
	// 만약 기존에 listVolumeFiles 같은 함수 있다면 교체.

	// 간결성을 위해 여기서는 당신이 이미 만들어 놓은
	// ReadDataFromVolume 수준의 원자 읽기 함수가 없다고 보고,
	// “파일 단위 검증” 대신 host 디렉터리와 동일 파일 집합만 확인하도록
	// WriteDataToVolume 때처럼 단일 파일 비교가 필요하다면 아래 map 대신
	// 그 파일 하나만 tar에서 찾는 로직으로 줄일 수 있음.

	// 재사용을 위해 단일 파일 검증이 아니라 전체 파일 해시 맵 구하는 형태 제시:
	return extractVolumeAsTarMap(t, ctx, volumeName, mountPath)
}

// 실제 tar 추출 구현 (테스트용). 프로젝트 내 공용 헬퍼가 있다면 대체 가능.
func extractVolumeAsTarMap(t *testing.T, ctx context.Context, volumeName, mountPath string) (map[string]string, error) {
	t.Helper()

	spec, err := NewSpec(
		WithImageName("docker.io/library/alpine:latest"),
		WithName("verify-"+volumeName),
		WithCommand([]string{
			"sh", "-c",
			// tar 가 파일 없을 때 0 이 아닌 코드 반환하는 것을 피하려면 || true 유지
			"cd \"$1\" && tar -cf - . || true",
			"sh", mountPath,
		}),
		WithNamedVolume(volumeName, mountPath, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("build spec: %w", err)
	}

	cr, err := containers.CreateWithSpec(ctx, spec, nil)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	cid := cr.ID
	defer func() {
		// 강제 제거(테스트 환경에서는 강제 제거가 흔히 안전)
		_, _ = containers.Remove(ctx, cid, &containers.RemoveOptions{Force: boolPtr(true)})
	}()

	if err := containers.Start(ctx, cid, nil); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// stdout 파이프 구성
	pr, pw := io.Pipe()

	// Attach 고루틴: stdout= pw, stderr 무시(필요하면 별도 writer)
	attachErrCh := make(chan error, 1)
	go func() {
		defer func() {
			_ = pw.Close()
		}()
		// stdin=nil, stdout=pw, stderr=nil, attachReady=nil
		aerr := containers.Attach(ctx, cid, nil, pw, nil, nil, nil)
		attachErrCh <- aerr
	}()

	out := make(map[string]string)
	tr := tar.NewReader(pr)

	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			// Attach 고루틴 에러와 구분 위해 wrap
			return nil, fmt.Errorf("tar read: %w", e)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		name := hdr.Name
		if name == "" || name == "." {
			continue
		}
		// 경로 정규화 (./ 제거)
		name = strings.TrimPrefix(name, "./")

		data, e := io.ReadAll(tr)
		if e != nil {
			return nil, fmt.Errorf("read file %s: %w", name, e)
		}
		sum := sha256.Sum256(data)
		out[name] = hex.EncodeToString(sum[:])
	}

	// Attach 결과 확인 (가능한 EOF 후 이미 끝났을 것)
	select {
	case aerr := <-attachErrCh:
		if aerr != nil {
			// tar 읽기 끝났더라도 attach 중 에러(컨테이너 조기 종료 등) 노출
			return nil, fmt.Errorf("attach: %w", aerr)
		}
	default:
		// 아직 안 끝났다면 기다림
		aerr := <-attachErrCh
		if aerr != nil {
			return nil, fmt.Errorf("attach(wait): %w", aerr)
		}
	}

	// 컨테이너 종료 상태 기다리기 (에러는 참고용)
	_, _ = containers.Wait(ctx, cid, nil)

	return out, nil
}

func boolPtr(b bool) *bool { return &b }

func fileContentHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestWriteFolderToVolume_Simple(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	volumeName := "test_wftv_simple"
	mountPath := "/data"

	// 사전 정리
	ensureVolumeDeleted(t, ctx, volumeName)

	// 호스트 임시 디렉터리 & 테스트 파일 구성
	hostDir := t.TempDir()
	files := map[string]string{
		"hello.txt":     "Hello, Test Data!",
		"dir/nested.md": "# Nested\ncontent",
	}
	for rel, content := range files {
		fp := filepath.Join(hostDir, rel)
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fp, []byte(content), 0o644); err != nil {
			t.Fatalf("write file %s: %v", rel, err)
		}
	}

	// 실행 (초기라 ModeSkip 과 ModeUpdate 는 동일 효과. 여기서는 ModeUpdate 사용)
	if err := WriteFolderToVolume(ctx, volumeName, mountPath, hostDir, ModeUpdate); err != nil {
		t.Fatalf("WriteFolderToVolume failed: %v", err)
	}

	// 검증: 볼륨 내부 파일 해시 비교
	gotMap, err := readAllFilesFromVolume(t, ctx, volumeName, mountPath)
	if err != nil {
		t.Fatalf("read volume contents: %v", err)
	}

	for rel, content := range files {
		wantHash := fileContentHash([]byte(content))
		// tar 추출 시 경로가 "./hello.txt" 형태일 수도 있어 간단 정규화
		// 여기서는 extractVolumeAsTarMap 이 그대로 rel 넣었으니 그대로 비교
		hash, ok := gotMap[rel]
		if !ok {
			// 일부 tar 구현은 "./<name>" 로 줄 수도 있으니 fallback
			if h2, ok2 := gotMap["./"+rel]; ok2 {
				hash = h2
				ok = true
			}
		}
		if !ok {
			t.Fatalf("expected file %s not found in volume", rel)
		}
		if hash != wantHash {
			t.Fatalf("file %s hash mismatch: got %s want %s", rel, hash, wantHash)
		}
	}
}

func TestWriteFolderToVolume_ModeSkip_NoOverwrite(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	volumeName := "test_wftv_skip"
	mountPath := "/data"
	ensureVolumeDeleted(t, ctx, volumeName)

	hostDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(hostDir, "a.txt"), []byte("A1"), 0o644); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	// 최초: 생성
	if err := WriteFolderToVolume(ctx, volumeName, mountPath, hostDir, ModeUpdate); err != nil {
		t.Fatalf("initial write: %v", err)
	}

	// 호스트 변경 (Skip 모드면 반영되면 안 됨)
	if err := os.WriteFile(filepath.Join(hostDir, "b.txt"), []byte("BNEW"), 0o644); err != nil {
		t.Fatalf("add b: %v", err)
	}

	if err := WriteFolderToVolume(ctx, volumeName, mountPath, hostDir, ModeSkip); err != nil {
		t.Fatalf("skip mode call: %v", err)
	}

	gotMap, err := readAllFilesFromVolume(t, ctx, volumeName, mountPath)
	if err != nil {
		t.Fatalf("read volume: %v", err)
	}

	if _, ok := gotMap["a.txt"]; !ok {
		t.Fatalf("a.txt missing")
	}
	if _, ok := gotMap["b.txt"]; ok {
		t.Fatalf("b.txt should NOT be copied in ModeSkip for existing volume")
	}
}

func TestWriteFolderToVolume_ModeOverwrite(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	volumeName := "test_wftv_overwrite"
	mountPath := "/data"
	ensureVolumeDeleted(t, ctx, volumeName)

	// 1) 초기 디렉터리
	dir1 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "old.txt"), []byte("OLD"), 0o644)

	if err := WriteFolderToVolume(ctx, volumeName, mountPath, dir1, ModeUpdate); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	// 2) 새 디렉터리
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "new.txt"), []byte("NEW"), 0o644)
	os.WriteFile(filepath.Join(dir2, "old.txt"), []byte("REPLACED"), 0o644)

	if err := WriteFolderToVolume(ctx, volumeName, mountPath, dir2, ModeOverwrite); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	gotMap, err := readAllFilesFromVolume(t, ctx, volumeName, mountPath)
	if err != nil {
		t.Fatalf("read volume: %v", err)
	}

	// 기대: old.txt 내용 교체, new.txt 존재, 이전 디렉터리에만 있던 파일은 제거
	if h, ok := gotMap["old.txt"]; !ok {
		t.Fatalf("old.txt missing after overwrite")
	} else if h != fileContentHash([]byte("REPLACED")) {
		t.Fatalf("old.txt hash mismatch after overwrite")
	}
	if _, ok := gotMap["new.txt"]; !ok {
		t.Fatalf("new.txt missing after overwrite")
	}
	// 예시에서는 dir1 전용 추가 파일이 없어서 제거 검증이 단조롭지만,
	// 필요하면 dir1 에 “gone.txt” 추가 후 dir2 에서는 생략해 검사 가능.
}
