//go:build integration

package podbridge5

import (
	"github.com/containers/storage/pkg/unshare"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 테스트 문제있는데 이거 중요하게 살펴봐야 함.
// https://docs.redhat.com/ko/documentation/red_hat_openshift_dev_spaces/3.17/html/administration_guide/configuring-fuse
// 로컬에서 생성하는게 힘들 수 있는데, 내부에서 생성해주는 방법도 생각해보고, 다양한 방법을 고려해보자.
// fuse-overlayfs2

func canMountFUSE() bool {
	f, err := os.OpenFile("/dev/fuse", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func hasUserAllowOther() bool {
	data, err := os.ReadFile("/etc/fuse.conf")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		// Ignore commented lines
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "user_allow_other" || strings.HasPrefix(trimmed, "user_allow_other ") {
			return true
		}
	}
	return false
}

// TestMountOverlayScenarios skips tests if running rootless without FUSE support.
// /dev/fuse 에 접근이 가능한지 테스트, 접근 불가능하면 fuse-overlayfs 를 사용할 수 없음.
func TestMountOverlayScenarios(t *testing.T) {
	t.Helper()
	// Skip if rootless without FUSE support
	if unshare.IsRootless() && !canMountFUSE() {
		t.Skip("/dev/fuse not accessible; skipping OverlayFS tests")
	}
}

// TestFuseConfUserAllowOther verifies that /etc/fuse.conf contains the "user_allow_other" directive.
// If the file is missing or the directive is absent, the test is skipped.
func TestFuseConfUserAllowOther(t *testing.T) {
	t.Helper()
	const fuseConf = "/etc/fuse.conf"
	data, err := os.ReadFile(fuseConf)
	if err != nil {
		t.Skipf("Cannot read %s: %v; skipping FUSE config test", fuseConf, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		// Ignore commented lines
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "user_allow_other" || strings.HasPrefix(trimmed, "user_allow_other ") {
			return
		}
	}
	t.Skipf("%s does not contain 'user_allow_other'; skipping FUSE config test", fuseConf)
}

// setupOverlay mounts and returns lower, upper, work, merged dirs, and a cleanup function.
func setupOverlay(t *testing.T) (lower, upper, work, merged string, cleanup func()) {
	t.Helper()

	if unshare.IsRootless() {
		if !canMountFUSE() || !hasUserAllowOther() {
			t.Skip("FUSE overlay not supported in this environment")
		}
	}

	base := t.TempDir()
	lower = filepath.Join(base, "lower")
	upper = filepath.Join(base, "upper")
	work = filepath.Join(base, "work")
	merged = filepath.Join(base, "merged")

	// Prepare input and directories
	if err := os.MkdirAll(lower, 0755); err != nil {
		t.Fatalf("failed to create lower dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lower, "input.txt"), []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write input file: %v", err)
	}

	// Mount overlay; skip test if not supported
	if err := MountOverlay(lower, upper, work, merged); err != nil {
		t.Skipf("Skipping test: Overlay mount not supported: %v", err)
	}

	return lower, upper, work, merged, func() {
		_ = unix.Unmount(merged, 0)
	}
}

// TestCaseA_Output writes to /out under merged and verifies upper captures it.
func TestCaseA_Output(t *testing.T) {
	_, upper, _, merged, cleanup := setupOverlay(t)
	defer cleanup()

	// Simulate tool writing to /app/data/out
	outDir := filepath.Join(merged, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("failed to create outDir: %v", err)
	}
	result := []byte("data")
	if err := os.WriteFile(filepath.Join(outDir, "result.txt"), result, 0644); err != nil {
		t.Fatalf("failed to write result file: %v", err)
	}

	// Verify result in upper
	upperFile := filepath.Join(upper, "out", "result.txt")
	data, err := os.ReadFile(upperFile)
	if err != nil {
		t.Fatalf("expected file in upper, got error: %v", err)
	}
	if string(data) != string(result) {
		t.Errorf("upper result mismatch, got %s", data)
	}
}

// TestCaseB1_Sidecar creates a sidecar file next to input and verifies only upper contains it.
func TestCaseB1_Sidecar(t *testing.T) {
	lower, upper, _, merged, cleanup := setupOverlay(t)
	defer cleanup()

	sideFile := filepath.Join(merged, "index.bai")
	content := []byte("bai")
	if err := os.WriteFile(sideFile, content, 0644); err != nil {
		t.Fatalf("failed to write sidecar file: %v", err)
	}

	// Lower should not have it
	if _, err := os.Stat(filepath.Join(lower, "index.bai")); !os.IsNotExist(err) {
		t.Errorf("lower should not contain sidecar, err: %v", err)
	}
	// Upper should have it
	got, err := os.ReadFile(filepath.Join(upper, "index.bai"))
	if err != nil {
		t.Fatalf("upper missing sidecar: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("upper sidecar content mismatch, got %s", got)
	}
}

// TestCaseB2_Transparency ensures reads come from lower and writes to merged see correct separate layers.
func TestCaseB2_Transparency(t *testing.T) {
	lower, upper, _, merged, cleanup := setupOverlay(t)
	defer cleanup()

	// Verify read sees original
	data, err := os.ReadFile(filepath.Join(merged, "input.txt"))
	if err != nil {
		t.Fatalf("failed to read merged input: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("merged read mismatch, got %s", data)
	}

	// Write new file and verify it goes to upper only
	newFile := filepath.Join(merged, "new.txt")
	if err := os.WriteFile(newFile, []byte("newdata"), 0644); err != nil {
		t.Fatalf("failed to write new file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(upper, "new.txt")); err != nil {
		t.Errorf("upper should have new file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lower, "new.txt")); !os.IsNotExist(err) {
		t.Errorf("lower should not have new file, err: %v", err)
	}
}

// TestCaseC_ModifyInput modifies existing file and verifies diff stored in upper only.
func TestCaseC_ModifyInput(t *testing.T) {
	lower, _, _, merged, cleanup := setupOverlay(t)
	defer cleanup()

	mergedInput := filepath.Join(merged, "input.txt")
	if err := os.WriteFile(mergedInput, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify merged input: %v", err)
	}
	// Lower should stay original
	baseData, err := os.ReadFile(filepath.Join(lower, "input.txt"))
	if err != nil {
		t.Fatalf("failed to read lower input: %v", err)
	}
	if string(baseData) != "original" {
		t.Errorf("lower modified, expected original, got %s", baseData)
	}
	// Merged read should reflect modification
	modData, err := os.ReadFile(mergedInput)
	if err != nil {
		t.Fatalf("failed to read merged input: %v", err)
	}
	if string(modData) != "modified" {
		t.Errorf("merged input not modified, got %s", modData)
	}
}
