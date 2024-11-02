#!/usr/bin/env bash

# 10초 정도 걸리는 계산 작업 수행
echo "Calculating..."

# 시작 시간 기록
start_time=$(date +%s)

# 큰 수까지 소수를 계산하여 작업 시간 확보
count=0
for ((i=2; i<50000; i++)); do
    is_prime=1
    for ((j=2; j*j<=i; j++)); do
        if ((i % j == 0)); then
            is_prime=0
            break
        fi
    done
    if ((is_prime)); then
        ((count++))
    fi
    # 10초가 지나면 루프 종료
    current_time=$(date +%s)
    elapsed_time=$((current_time - start_time))
    if ((elapsed_time >= 10)); then
        break
    fi
done

echo "Found $count prime numbers in $elapsed_time seconds."

exit 0