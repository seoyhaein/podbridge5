#!/usr/bin/env bash

# PID 배열 예제 (테스트를 위해 가상의 PID를 넣음)
pids=(1234 5678 91011)  # 이 부분은 실제 실행 중인 PID로 변경하거나, 테스트 값으로 유지

# 반복문을 통해 각 PID의 상태를 확인
for pid in "${pids[@]}"; do
  echo "Process PID: '$pid'"
done