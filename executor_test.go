package podbridge5

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateExecutor(t *testing.T) {
	// 테스트 경로와 파일명 설정
	testPath := "./test-scripts"
	fileName := "test_executor.sh"
	cmd := "./user_script.sh" // 컨테이내에서  실행 명령어

	// 디렉토리 생성
	if err := os.MkdirAll(testPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testPath) // 테스트 후 디렉토리 삭제

	// generateExecutor 실행
	_, executorPath, err := GenerateExecutor(testPath, fileName, cmd)
	if err != nil {
		t.Fatalf("generateExecutor failed: %v", err)
	}

	// 결과 파일 확인
	if _, err := os.Stat(*executorPath); os.IsNotExist(err) {
		t.Fatalf("Executor file was not created: %s", *executorPath)
	} else {
		t.Logf("Executor file created at: %s", *executorPath)
	}
}

func TestProcessScript(t *testing.T) {
	// 테스트 경로 설정
	testPath := "./test-scripts"

	scriptContent := `sleep 5
echo "Hello, World!dfdfdfdfd"`
	/*	scriptContent := `#!/bin/bash
		echo "Hello, World!"`*/

	// 디렉토리 생성
	if err := os.MkdirAll(testPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testPath) // 테스트 후 디렉토리 삭제

	// processScript 실행
	scriptPath, err := ProcessScript(scriptContent, testPath)
	if err != nil {
		t.Fatalf("processScript failed: %v", err)
	}

	// 결과 파일 확인
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Fatalf("Script file was not created: %s", scriptPath)
	} else {
		t.Logf("Script file created at: %s", scriptPath)
	}

	// 파일 내용 확인
	expectedContent := "#!/bin/bash\necho \"Hello, World!\""
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read script file: %v", err)
	}

	if string(data) != expectedContent {
		t.Errorf("unexpected script content. Got: %s, Want: %s", string(data), expectedContent)
	}
}

func TestCompareFiles(t *testing.T) {
	// 테스트 경로 설정
	testPath := "./test-scripts"
	file1 := filepath.Join(testPath, "file1.txt")
	file2 := filepath.Join(testPath, "file2.txt")

	// 디렉토리 생성
	if err := os.MkdirAll(testPath, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testPath) // 테스트 후 디렉토리 삭제

	// 파일 생성 및 내용 작성
	content1 := []byte("Test content for file comparison.")
	if err := os.WriteFile(file1, content1, 0644); err != nil {
		t.Fatalf("failed to write to file1: %v", err)
	}

	// 동일한 내용으로 file2 작성
	if err := os.WriteFile(file2, content1, 0644); err != nil {
		t.Fatalf("failed to write to file2: %v", err)
	}

	// 파일 비교 테스트 (동일한 파일)
	same, err := compareFiles(file1, file2)
	if err != nil {
		t.Fatalf("compareFiles failed: %v", err)
	}
	if !same {
		t.Errorf("Expected files to be the same, but they are different")
	}

	// file2에 다른 내용 작성
	content2 := []byte("Different content")
	if err := os.WriteFile(file2, content2, 0644); err != nil {
		t.Fatalf("failed to write to file2: %v", err)
	}

	// 파일 비교 테스트 (다른 파일)
	same, err = compareFiles(file1, file2)
	if err != nil {
		t.Fatalf("compareFiles failed: %v", err)
	}
	if same {
		t.Errorf("Expected files to be different, but they are the same")
	}
}
