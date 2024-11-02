#!/usr/bin/env bash

# healthcheck.log 파일을 읽어 상태와 종료 코드를 확인하는 스크립트
LOG_FILE="healthcheck.log"
STATUS_LOG_FILE="internal_status.log"

# 로그 파일 초기화
> "$STATUS_LOG_FILE"

TIME_LIMIT=$1  # 사용자가 입력하는 제한 시간 (초 단위), 옵션
exitCode=255   # 기본적으로 비정상 종료 (255)로 설정
start_time=""
status=""
log_exit=""

# 로그 메시지 기록 함수
log_message() {
    local message="$1"
    echo "$(date --iso-8601=seconds) - $message" >> "$STATUS_LOG_FILE"
}

# 현재 시간을 초 단위로 반환하는 함수 (epoch time)
current_time() {
    date +%s
}

# 주어진 시간 문자열을 epoch time으로 변환하는 함수
to_epoch_time() {
    date -d "$1" +%s
}

# LOG_FILE 파일이 존재하지 않으면 exit 5 반환
if [ ! -f "$LOG_FILE" ]; then
    log_message "Error: Log file '$LOG_FILE' does not exist. Exiting with code 5."
    exit 5  # 로그 파일 없음
    else
      log_message "Info: Log file '$LOG_FILE' exists. Continuing with health check."
fi

# 로그 파일 읽기
while read -r line || [ -n "$line" ]; do
    if [[ "$line" == *start:* ]]; then
        start_time=${line#start:}  # 시작 시간을 기록
    fi

    if [[ "$line" == *status:* ]]; then
        status=${line#status:}  # status 값을 저장
    fi

    if [[ "$line" == *exit:* ]]; then
        log_exit=${line#exit:}  # exit 값을 저장
    fi
done < "$LOG_FILE"


# 상태 업데이트를 위한 함수 (동시 쓰기 방지)
#status_update() {
#    local new_status="$1"
#    (
#        flock -e 200
#        sed -i "s/^status:.*/status:$new_status/" "$LOG_FILE"
#    ) 200>"$LOG_FILE.lock"
#}

# status가 completed이고 exit 값이 0이면 exit 0 반환
if [[ "$status" == "completed" && "$log_exit" -eq 0 ]]; then
    log_message "Info: Process completed successfully. Exiting with code 0."
    exit 0
fi

# status가 failed 또는 syntax_error 이고  exit 값이 0이 아닌 경우 로그 파일의 exit 값을 리턴
if [[ ("$status" == "failed" || "$status" == "syntax_error") && "$log_exit" -ne 0 ]]; then
    log_message "Error: Process failed with exit code $log_exit from log file. Exiting with code $log_exit."
    exit "$log_exit"
fi

if [[ "$log_exit" -ne 0 ]]; then
    log_message "Error: Process failed with exit code $log_exit from log file. Exiting with code $log_exit."
    exit "$log_exit"
fi

# status가 running이고 start 시간이 있는 경우, 제한 시간을 초과했는지 확인
if [[ "$status" == "running" && -n "$start_time" && -n "$TIME_LIMIT" ]]; then
    # TIME_LIMIT이 정수인지 확인
    if ! [[ "$TIME_LIMIT" =~ ^[0-9]+$ ]]; then
        log_message "Error: TIME_LIMIT '$TIME_LIMIT' is not a valid integer. Exiting with code 3."
        exit 3
    fi

    start_epoch=$(to_epoch_time "$start_time")
    current_epoch=$(current_time)
    time_diff=$((current_epoch - start_epoch))

    if [ "$time_diff" -gt "$TIME_LIMIT" ]; then
        log_message "Error: Process timed out after $TIME_LIMIT seconds. Exiting with code 2."
        exit 2  # 제한 시간을 초과했으면 exit 2 반환
    fi
fi

# start 또는 status 값이 없으면 exit 3 반환
if [[ -z "$start_time" || -z "$status" ]]; then
    log_message "Error: 'start' or 'status' value missing in log file. Exiting with code 3."
    exit 3  # 필수 로그 항목 누락
fi

# start 시간이 없으면 제한 시간이 설정되지 않았으므로 exit 3 반환
#if [[ "$status" == "running" && -z "$start_time" ]]; then
#    log_message "Error: 'start_time' is missing while status is 'running'. Exiting with code 3."
#    exit 3
#fi

# 그 외의 알 수 없는 경우 비정상 종료 처리 (exit 255)
log_message "Error: Unknown error occurred. Exiting with code $log_exit."
exit $log_exit
