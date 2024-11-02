#!/usr/bin/env bash

result_log="/app/result.log"
temp_status_log="/app/exit_code_temp.log"  # 임시 로그 파일
status_log="/app/exit_code.log"  # 종료 코드 기록용 로그 파일
> "$result_log"
> "$status_log"
> "$temp_status_log"

# long_task 함수
long_task() {
    if ! bash -n ./scripts/user_script.sh; then
        echo "Syntax error in user_script.sh" | tee -a "$result_log"
        return 1
    fi

    bash ./scripts/user_script.sh 2>&1 | tee -a "$result_log"
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

exit $task_exit_code