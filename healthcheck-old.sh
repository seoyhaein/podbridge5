#!/usr/bin/env bash

STATUS_LOG_FILE="internal_status.log"
EXIT_CODE_LOG_FILE="/app/exit_code.log"
LOCK_FILE="$EXIT_CODE_LOG_FILE.lock"

# 로그 메시지 기록 함수
log_message() {
    local message="$1"
    echo "$(date --iso-8601=seconds) - $message" >> "$STATUS_LOG_FILE"
}

# 실행 중인 executor.sh 프로세스 확인
pids=($(ps -eo pid,cmd | grep -w "[b]ash /app/executor.sh" | awk '{print $1}'))
if [ ${#pids[@]} -eq 0 ]; then
    log_message "Healthcheck: Process not found (Unhealthy)"
    exit 1
fi

# 모든 PID의 상태 확인
all_healthy=true
for pid in "${pids[@]}"; do
  log_message "Process PID: '$pid'"
  status=$(ps -p "$pid" -o stat= | tr -d ' ')
  log_message "Process status: '$status'"

  case "$status" in
    R*|S*)
      log_message "Healthcheck: Process PID $pid is healthy (running or sleeping)"
      ;;
    D*|Z*|T*|X*|*)
      log_message "Healthcheck: Process PID $pid is unhealthy"
      all_healthy=false
      ;;
  esac
done

# 종료 코드 파일 확인 및 프로세스 상태에 따른 처리
if [ ! -f "$LOCK_FILE" ]; then
    touch "$LOCK_FILE"
fi

# 종료 코드 초기화
exit_code=""

# 종료 코드 파일 확인 및 프로세스 상태에 따른 처리
if [ -s "$EXIT_CODE_LOG_FILE" ]; then
    {
        log_message "$EXIT_CODE_LOG_FILE exists and not empty"
        flock -s 200  # 공유 잠금 설정

        # 파일 내용을 한 줄씩 읽어 종료 코드 추출
        while read -r line || [ -n "$line" ]; do
           log_message "Reading line: $line"  # 읽은 줄을 로그로 기록
            if [[ "$line" == exit_code:* ]]; then
              log_message "Matching line found"  # 디버그 메시지 추가
              exit_code=${line#exit_code:}
              log_message "Extracted exit_code: $exit_code"  # 디버그 메시지 추가
              break  # exit_code를 찾은 후에는 더 이상 읽지 않음
            fi
        done < "$EXIT_CODE_LOG_FILE"
    } 200<"$LOCK_FILE"

    # 종료 코드와 상태에 따른 헬스 체크 결과 처리
    if [[ "$exit_code" -eq 0 && "$all_healthy" = true ]]; then
        log_message "Healthcheck: All processes are healthy, and exit code is 0. exit_code is $exit_code"
        exit 0
    else
        log_message "Healthcheck: Exit code is non-zero or one or more processes are unhealthy"
        exit 1
    fi
else
    # 종료 코드 파일이 없는 경우 실행 중인 상태로 간주
    log_message "$EXIT_CODE_LOG_FILE not exists or empty"
    if [ "$all_healthy" = true ]; then
        log_message "Healthcheck: Process is still running and healthy (no exit code yet)"
        exit 0
    else
        log_message "Healthcheck: Process state unknown (no exit code file and unhealthy status)"
        exit 1
    fi
fi
