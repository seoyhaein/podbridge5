package podbridge5

import (
	"bytes"
	"fmt"
	"github.com/seoyhaein/utils"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateExecutor TODO 통일성을 위해서 defer 리턴 구문 정리
// path 생성될 executor.sh 의 path, fileName "executor.sh", userScriptPath 컨테이너내에서 executor.sh 가 실행 할 user_script.sh 의 위치
func GenerateExecutor(path, fileName, userScriptPath string) (*os.File, *string, error) {
	if utils.IsEmptyString(path) || utils.IsEmptyString(fileName) {
		return nil, nil, fmt.Errorf("path or file name is empty")
	}

	var (
		tmpFile      *os.File
		err          error
		executorPath string
		tmpFilePath  string
	)

	defer func() {
		// 파일을 닫는 처리
		if tmpFile != nil {
			if err = tmpFile.Close(); err != nil {
				Log.Errorf("Failed to create temporary file: %v", err)
			}
		}
	}()

	executorPath = fmt.Sprintf("%s/%s", path, fileName)
	tmpFilePath = fmt.Sprintf("%s/%s.tmp", path, fileName) // 임시 파일 경로 설정

	// 임시 파일 생성
	tmpFile, err = os.Create(tmpFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	// 임시 파일에 스크립트 내용을 작성
	scriptContent := fmt.Sprintf(`#!/usr/bin/env bash

result_log="/app/result.log"
temp_status_log="/app/exit_code_temp.log"  # 임시 로그 파일
status_log="/app/exit_code.log"  # 종료 코드 기록용 로그 파일
> "$result_log"
> "$status_log"
> "$temp_status_log"

# long_task 함수
long_task() {
    if ! bash -n %s; then
        echo "Syntax error in user_script.sh" | tee -a "$result_log"
        return 1
    fi

    bash %s 2>&1 | tee -a "$result_log"
    task_exit_code=${PIPESTATUS[0]}
    return $task_exit_code
}

long_task
task_exit_code=$?

# 임시 파일에 종료 코드 기록
{
    flock -e 200
    echo "exit_code:$task_exit_code" > "$temp_status_log"
} 200>"$temp_status_log.lock"

# 임시 파일을 최종 파일로 이동
mv "$temp_status_log" "$status_log"

# 헬스체크를 위해서 넣음. TODO 추후 조정 필요
sleep 10

# 종료 코드 확인 및 에러 처리
if [ "$task_exit_code" -ne 0 ]; then
    echo "Task failed with exit code $task_exit_code" | tee -a "$result_log"
else
    echo "Task completed successfully" | tee -a "$result_log"
fi

exit $task_exit_code`, userScriptPath, userScriptPath) // cmd 값을 삽입하여 실행할 명령어를 설정

	_, err = tmpFile.Write([]byte(scriptContent))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write script content to temporary file: %w", err)
	}

	// 파일을 디스크에 동기화
	if err := tmpFile.Sync(); err != nil {
		return nil, nil, fmt.Errorf("failed to sync temporary file: %w", err)
	}

	// 기존 파일이 있는지 확인
	if exists, _, err := utils.FileExists(executorPath); err != nil {
		return nil, nil, fmt.Errorf("failed to check if original file exists: %w", err)
	} else if exists {
		// 기존 파일과 임시 파일 내용 비교
		same, err := compareFiles(executorPath, tmpFilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to compare files: %w", err)
		}

		// 파일이 같으면 임시 파일 삭제
		if same {
			if err := os.Remove(tmpFilePath); err != nil {
				Log.Errorf("Failed to remove temporary file %s: %v", tmpFilePath, err)
				return nil, nil, err
			}
			return nil, &executorPath, nil
		}
	}

	// 파일이 다르거나 기존 파일이 없는 경우, 임시 파일을 최종 파일로 교체
	err = os.Rename(tmpFilePath, executorPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to rename temporary file to final file: %w", err)
	}

	// 파일 권한 설정
	err = os.Chmod(executorPath, 0777)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to set file permissions: %w", err)
	}

	return tmpFile, &executorPath, nil
}

// compareFiles 두 파일의 내용을 비교하는 함수
func compareFiles(file1, file2 string) (bool, error) {
	f1, err := os.Open(file1)
	if err != nil {
		return false, err
	}

	defer func() {
		if err = f1.Close(); err != nil {
			Log.Errorf("Failed to create file1: %v", err)
		}
	}()

	f2, err := os.Open(file2)
	if err != nil {
		return false, err
	}

	defer func() {
		if err = f2.Close(); err != nil {
			Log.Errorf("Failed to create file2: %v", err)
		}
	}()

	f1Stat, err := f1.Stat()
	if err != nil {
		return false, err
	}

	f2Stat, err := f2.Stat()
	if err != nil {
		return false, err
	}

	// 파일 크기가 다르면 내용이 다르다고 간주
	if f1Stat.Size() != f2Stat.Size() {
		return false, nil
	}

	// 파일 내용을 비교
	buf1 := make([]byte, 1024)
	buf2 := make([]byte, 1024)

	for {
		n1, err1 := f1.Read(buf1)
		n2, err2 := f2.Read(buf2)

		if err1 != nil && err1 != io.EOF {
			return false, err1
		}
		if err2 != nil && err2 != io.EOF {
			return false, err2
		}

		if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n1]) {
			return false, nil
		}

		if err1 == io.EOF && err2 == io.EOF {
			break
		}
	}

	return true, nil
}

// ProcessScript use_script.sh 만들어 주는 메서드
// 기존 파일이 있을 경우, 파일을 삭제하고 새로 생성할지, 비교할지 고민하자. 일단 버그가 있음.
// TODO ProcessScript(testScript, "./scripts/") 이런식으로 해야하는데 이건 개선하는 방향으로 하자.
func ProcessScript(scriptContent string, path string) (string, error) {
	// 디렉토리가 존재하지 않으면 생성
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// 스크립트 내용의 앞뒤 공백 및 들여쓰기 제거
	//scriptContent = ensureShebang(scriptContent)

	// 받은 스크립트를 텍스트 파일로 저장 (보관용)
	txtFilePath := filepath.Join(path, "user_script.txt")
	if err := os.WriteFile(txtFilePath, []byte(scriptContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write script content to txt file: %w", err)
	}

	// 임시 파일 생성
	tmpFile, err := os.CreateTemp("/tmp", "user_script_*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil { // 문법 검사 실패 시 임시 파일 삭제
			Log.Errorf("Failed to remove temporary file %s: %v", tmpFile.Name(), err)
			err = fmt.Errorf("failed to remove temporary file %s: %w", tmpFile.Name(), err)
		}
	}()

	// 쉘 스크립트 내용을 임시 파일에 씀
	if _, err = tmpFile.WriteString(scriptContent); err != nil {
		return "", fmt.Errorf("failed to write script content to temp file: %w", err)
	}

	// 파일을 닫고 저장
	if err = tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// 문법 검사 수행
	if err = exec.Command("bash", "-n", tmpFile.Name()).Run(); err != nil {
		// 문법 오류가 있으면 .sh 파일을 남기지 않고 에러 반환
		return "", fmt.Errorf("syntax error in user script: %w", err)
	}

	// 문법 검사가 통과되었으므로 임시 파일을 최종 위치로 이동
	shFilePath := filepath.Join(path, "user_script.sh")
	if err = os.Rename(tmpFile.Name(), shFilePath); err != nil {
		return "", fmt.Errorf("failed to move temp file to final location: %w", err)
	}

	if err = os.Chmod(shFilePath, 0777); err != nil {
		return "", fmt.Errorf("failed to set file permissions: %w", err)
	}

	// 문법 검사가 성공했을 때 .sh 파일 경로 반환
	// 마지막 err 의 경우 defer 에서 nil 이 아닐 경우 err 를 반환한다.
	return shFilePath, err
}

// ensureShebang  TODO 삭제하지만 주석으로 남겨둔다.
/*func ensureShebang(scriptContent string) string {
	// 스크립트에서 #!이 처음 등장하는 위치를 찾음
	shebangIndex := strings.Index(scriptContent, "#!")

	// #!이 첫 번째 줄이 아닌 경우, 앞에 불필요한 내용 제거
	if shebangIndex > 0 {
		// shebang 앞의 공백이나 불필요한 내용을 제거
		scriptContent = scriptContent[shebangIndex:]
	}
	return strings.TrimSpace(scriptContent)
}*/
