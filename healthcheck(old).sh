#!/usr/bin/env bash

# healthcheck.log 파일을 읽어 상태와 종료 코드를 확인하는 스크립트
LOG_FILE="healthcheck.log"
TIME_LIMIT=$1  # 사용자가 입력하는 제한 시간 (초 단위)
exitCode=255  # 기본적으로 비정상 종료 (255)로 설정
start_time=""
status=""
log_exit=""

# 이거 테스트 해봐야 함.
# LOG_FILE 파일이 존재하지 않으면 exit 5 반환
if [ ! -f "$LOG_FILE" ]; then
  exit 5  # 로그 파일 없음
fi

# 현재 시간을 초 단위로 반환하는 함수 (epoch time)
current_time() {
  date +%s
}

# 주어진 시간 문자열을 epoch time으로 변환하는 함수
to_epoch_time() {
  date -d "$1" +%s
}

while read -r line || [ -n "$line" ] ; do
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

# start 또는 status 값이 없으면 exit 3 반환
if [[ -z "$start_time" || -z "$status" ]]; then
  exit 3  # 필수 로그 항목 누락
fi

# 상태 업데이트를 위한 함수 (동시 쓰기 방지)
status_update() {
    local new_status="$1"
    (
        flock -e 200
        sed -i "s/status:running/status:$new_status/" "$LOG_FILE"
    ) 200>"$LOG_FILE.lock"
}

# status 가 completed 이고 exit 값이 0이면 exit 0 반환
if [[ "$status" == "completed" && "$log_exit" -eq 0 ]]; then
    exit 0
fi

# status 가 failed 이고 exit 값이 0이 아닌 경우 로그 파일의 exit 값을 리턴
if [[ "$status" == "failed" && "$log_exit" -ne 0 ]]; then
    exit "$log_exit"
fi

# status 가 running 이고 start 시간이 있는 경우, 제한 시간을 초과했는지 확인
if [[ "$status" == "running" && -n "$start_time" ]]; then
  start_epoch=$(to_epoch_time "$start_time")
  current_epoch=$(current_time)
  time_diff=$((current_epoch - start_epoch))

  if [ "$time_diff" -gt "$TIME_LIMIT" ]; then
    status_update "timeout"
    exit 2  # 제한 시간을 초과했으면 exit 2 반환
  else
    status_update "completed"
    exit 0  # 제한 시간 내면 exit 0 반환
  fi
fi

# start 시간이 없으면 제한 시간이 설정되지 않았으므로 exit 3 반환
if [[ "$status" == "running" && -z "$start_time" ]]; then
  exit 3
fi
# user_script.sh 의 리턴값의 실패시 1 로 처리하거나 기타 다른 코드로 처리하는데 중복되는 경우가 있다.
# user_script.sh 실패시 리턴 값은 10-100 사이의 값으로 하는 것을 생각해보자.
# 그 외의 알 수 없는 경우 비정상 종료 처리 (exit 255)
exit 255
