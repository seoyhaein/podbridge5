#!/bin/sh

# 이미 bash에서 실행 중인 경우 반복하지 않도록 함
if [ -z "$BASH_EXECUTED" ]; then
    if ! command -v bash >/dev/null 2>&1; then
        echo "Bash is not installed. Installing bash..."

        if command -v apk >/dev/null 2>&1; then
            apk add bash  # Alpine
        elif command -v apt >/dev/null 2>&1; then
            apt update && apt install -y bash  # Ubuntu
        elif command -v yum >/dev/null 2>&1; then
            yum install -y bash  # RHEL/CentOS
        else
            echo "Unknown package manager. Please install bash manually."
            exit 1
        fi
    fi

    # 환경 변수 설정하고 Bash로 다시 실행
    export BASH_EXECUTED=1
    exec /usr/bin/env bash "$0" "$@"
fi

#!/usr/bin/env bash

# healthcheck.log 파일과 사용자 app의 출력을 기록할 로그 파일
healthcheck_log="./healthcheck.log"
result_log="./result.log"

# 로그 파일 초기화
> "$healthcheck_log"
> "$result_log"

# ISO 8601 형식으로 시작 시간 기록
start_time=$(date --iso-8601=seconds)
echo "start: $start_time" | tee -a "$healthcheck_log"

# 상태 업데이트 함수 (동시 쓰기 방지)
status_update() {
    local status="$1"
    (
        flock -e 200
        sed -i "s/status:running/status:$status/" "$healthcheck_log"
    ) 200>"$healthcheck_log.lock"
}

# long_task 함수
long_task() {
    echo "status:running" | tee -a "$healthcheck_log"

    # 사용자 app 실행 후 stdout, stderr을 로그에 기록
    # 문법 오류가 발생하는지 미리 감지하고 실행
    if ! bash -n scripts/user_script.sh; then
        echo "Syntax error in shelltester" | tee -a "$result_log"
        return 2  # 문법 오류를 의미하는 반환 코드
    fi

    # 문법 오류가 없으면 실제 실행
    bash scripts/user_script.sh 2>&1 | tee -a "$result_log"

    # shelltester의 종료 코드를 반환
    return ${PIPESTATUS[0]}
}

# long_task 백그라운드에서 실행
long_task &
wait $!  # 백그라운드 작업이 완료될 때까지 대기

task_exit_code=$?  # 종료 코드 저장

# 상태 업데이트 및 종료 코드 기록
if [ "$task_exit_code" -eq 0 ]; then
    status_update "completed"
elif [ "$task_exit_code" -eq 2 ]; then
    status_update "syntax_error"
else
    status_update "failed"
fi

echo "exit:$task_exit_code" | tee -a "$healthcheck_log"
