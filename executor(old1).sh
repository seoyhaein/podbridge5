#!/usr/bin/env bash

# 사용자 app의 출력을 기록할 로그 파일
result_log="./result.log"

# 로그 파일 초기화
> "$result_log"

# long_task 함수
long_task() {
    # 사용자 app 실행 후 stdout, stderr을 로그에 기록
    # 문법 오류가 발생하는지 미리 감지하고 실행 
    if ! bash -n ./scripts/user_script.sh; then
        echo "Syntax error in user_script.sh" | tee -a "$result_log"
        return 1  # 문법 오류를 의미하는 반환 코드
    fi

    # 문법 오류가 없으면 실제 실행
    bash ./scripts/user_script.sh 2>&1 | tee -a "$result_log"

    # 종료 코드를 반환
    return ${PIPESTATUS[0]}
}

# long_task 백그라운드에서 실행
long_task &
wait $!  # 백그라운드 작업이 완료될 때까지 대기

task_exit_code=$?  # 종료 코드 저장

# 종료 코드 확인 및 에러 처리
if [ "$task_exit_code" -ne 0 ]; then
    echo "Task failed with exit code $task_exit_code" | tee -a "$result_log"
else
    echo "Task completed successfully" | tee -a "$result_log"
fi

# 종료 코드 반환
exit $task_exit_code