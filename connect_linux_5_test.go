package podbridge5

import (
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/system"
	"os"
	"testing"
)

func TestSocketDirectoryForCurrentUser(t *testing.T) {
	uid := os.Getuid()
	socketDir := fmt.Sprintf("/run/user/%d", uid)
	t.Logf("Expected socket directory: %s", socketDir)

	info, err := os.Stat(socketDir)
	if os.IsNotExist(err) {
		t.Skipf("Socket directory %q does not exist, skipping test", socketDir)
	} else if err != nil {
		t.Fatalf("Failed to stat %q: %v", socketDir, err)
	}

	if !info.IsDir() {
		t.Fatalf("Path %q exists but is not a directory", socketDir)
	}
}

func TestNewConnectionLinux5(t *testing.T) {
	// 1) Context 생성 및 연결 확인
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	t.Log("Podman connection established")

	// 2) (선택) 간단한 바인딩 호출로 실제 연결 검증
	versionOption := &system.VersionOptions{}
	ver, err := system.Version(ctx, versionOption)
	if err != nil {
		t.Logf("Warning: version check failed: %v", err)
	} else {
		t.Logf("Podman server version: %v", ver.Server)
		t.Logf("Podman client version: %v", ver.Client)
	}
}
