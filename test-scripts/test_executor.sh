#!/usr/bin/env bash
set -euo pipefail

RESULT_LOG="/app/result.log"
STATUS_LOG="/app/exit_code.log"
LOCK_FILE="${STATUS_LOG}.lock"

# 로그 초기화
: > "$RESULT_LOG"

# 상태 기록 함수 (실패 시 바로 호출)
record_failure() {
    local code=$1
    echo "exit_code:${code}" > "${STATUS_LOG}.tmp"
    {
        flock -x 200
        mv "${STATUS_LOG}.tmp" "${STATUS_LOG}"
    } 200> "$LOCK_FILE"
    echo "Syntax error detected, exit_code=${code}" | tee -a "$RESULT_LOG"
    exit "${code}"
}

# 1) ./user_script.sh 존재 및 문법 검사
if [[ ! -f "./user_script.sh" ]]; then
    echo "Error: ./user_script.sh not found" | tee -a "$RESULT_LOG"
    record_failure 1
fi

if ! bash -n "./user_script.sh"; then
    echo "Syntax error in ./user_script.sh" | tee -a "$RESULT_LOG"
    record_failure 1
fi

# 2) 실제 실행 및 정상 흐름
bash "./user_script.sh" 2>&1 | tee -a "$RESULT_LOG"
EXIT_CODE=${PIPESTATUS[0]}

# 3) 상태 기록 (정상/실패 공통)
echo "exit_code:${EXIT_CODE}" > "${STATUS_LOG}.tmp"
{
    flock -x 200
    mv "${STATUS_LOG}.tmp" "${STATUS_LOG}"
} 200> "$LOCK_FILE"

# 4) 최종 로그
if (( EXIT_CODE != 0 )); then
    echo "Task failed with exit code ${EXIT_CODE}" | tee -a "$RESULT_LOG"
else
    echo "Task completed successfully" | tee -a "$RESULT_LOG"
fi

exit "${EXIT_CODE}"