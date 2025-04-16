package podbridge5

import (
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	"github.com/google/uuid"
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

// TestWriteDataToVolume tests WriteDataToVolume by writing data and reading it back.
func TestWriteDataToVolume(t *testing.T) {
	ctx, err := NewConnectionLinux5(context.Background())
	if err != nil {
		t.Fatalf("NewConnectionLinux5() failed: %v", err)
	}

	volumeName := "test_volume_for_write"
	mountPath := "/data"
	fileName := "hello.txt"
	expectedData := "Hello, Test Data!"

	// Step 0: Check if the volume already exists; if so, delete it.
	exists, err := volumes.Exists(ctx, volumeName, &volumes.ExistsOptions{})
	if err != nil {
		t.Fatalf("Failed to check volume existence: %v", err)
	}
	if exists {
		force := false
		if err := DeleteVolume(ctx, volumeName, &force); err != nil {
			t.Fatalf("Failed to delete pre-existing volume %q: %v", volumeName, err)
		}
		// 잠시 대기하여 삭제가 완료되도록 함.
		time.Sleep(1 * time.Second)
	}

	// Step 1: Write data to the volume.
	err = WriteDataToVolume(ctx, volumeName, mountPath, fileName, []byte(expectedData))
	if err != nil {
		t.Fatalf("WriteDataToVolume failed: %v", err)
	}

	// Step 2: Read back the file content from the volume.
	retrieved, err := ReadDataFromVolume(ctx, volumeName, mountPath, fileName)
	if err != nil {
		t.Fatalf("Failed to read data from volume: %v", err)
	}

	// Step 3: Compare expected and retrieved data.
	if retrieved != expectedData {
		t.Fatalf("Data mismatch: expected %q, got %q", expectedData, retrieved)
	}
}
