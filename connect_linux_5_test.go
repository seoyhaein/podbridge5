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
	// 1) Podman 소켓에 연결 시도
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}
	t.Log("Podman connection established")

	// 2) 버전 정보 조회
	verOpts := &system.VersionOptions{}
	verReport, err := system.Version(ctx, verOpts)
	if err != nil {
		t.Logf("Warning: could not retrieve Podman version: %v", err)
		return
	}

	// 3) 서버(엔진) 버전 출력
	if verReport.Server != nil {
		t.Logf("Podman server version: %s", verReport.Server.Version)
	} else {
		t.Log("Podman server version: <nil>")
	}

	// 4) 클라이언트 버전 출력
	if verReport.Client != nil {
		t.Logf("Podman client version: %s", verReport.Client.Version)
	} else {
		t.Log("Podman client version: <nil>")
	}
}
