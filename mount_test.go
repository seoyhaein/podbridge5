package podbridge5

import (
	"github.com/containers/podman/v5/pkg/specgen"
	"os"
	"path/filepath"
	"testing"
)

func TestWithMount(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("host path does not exist", func(t *testing.T) {
		spec := &specgen.SpecGenerator{}
		err := WithMount(
			filepath.Join(tmpDir, "no-such-dir"),
			"/container/path",
			"bind",
		)(spec)
		if err == nil {
			t.Fatal("expected error when source path does not exist, got nil")
		}
	})

	t.Run("host path is a file, not directory", func(t *testing.T) {
		// tmpDir 안에 임시 파일을 하나 만든다
		tmpFile := filepath.Join(tmpDir, "somefile.txt")
		if f, err := os.Create(tmpFile); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		} else {
			f.Close()
		}

		spec := &specgen.SpecGenerator{}
		err := WithMount(
			tmpFile,
			"/container/path",
			"bind",
		)(spec)
		if err == nil {
			t.Fatal("expected error when source is a file, got nil")
		}
	})

	t.Run("host path is a directory", func(t *testing.T) {
		// tmpDir 안에 하위 디렉토리를 하나 만든다
		hostDir := filepath.Join(tmpDir, "somedir")
		if err := os.Mkdir(hostDir, 0o755); err != nil {
			t.Fatalf("failed to mkdir temp dir: %v", err)
		}

		spec := &specgen.SpecGenerator{}
		if err := WithMount(
			hostDir,
			"/container/path",
			"bind",
		)(spec); err != nil {
			t.Fatalf("expected no error for valid directory, got: %v", err)
		}

		// spec.Mounts 에 하나의 항목이 들어갔는지, 필드들이 올바른지 확인
		if len(spec.Mounts) != 1 {
			t.Fatalf("expected 1 mount, got %d", len(spec.Mounts))
		}
		m := spec.Mounts[0]
		if m.Type != "bind" {
			t.Errorf("expected mount.Type = %q, got %q", "bind", m.Type)
		}
		if m.Source != hostDir {
			t.Errorf("expected mount.Source = %q, got %q", hostDir, m.Source)
		}
		if m.Destination != "/container/path" {
			t.Errorf("expected mount.Destination = %q, got %q", "/container/path", m.Destination)
		}
	})
}
