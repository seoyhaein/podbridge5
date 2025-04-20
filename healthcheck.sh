#!/usr/bin/env bash
set -eo pipefail

LOG_DIR="/app"
STATUS_LOG_FILE="$LOG_DIR/internal_status.log"
EXIT_CODE_LOG_FILE="$LOG_DIR/exit_code.log"
LOCK_FILE="$EXIT_CODE_LOG_FILE.lock"

# 간단 로그 함수
log() {
  echo "$(date +'%Y-%m-%dT%H:%M:%S%z') - $*" >> "$STATUS_LOG_FILE"
}

# 1) 프로세스 검색
mapfile -t pids < <(pgrep -f "[b]ash /app/executor.sh")
if (( ${#pids[@]} == 0 )); then
  log "Healthcheck: executor.sh 프로세스가 없음"
  exit 1
fi

# 2) 상태 확인
all_healthy=true
for pid in "${pids[@]}"; do
  st=$(ps -p "$pid" -o stat= | tr -d ' ')
  log "PID $pid status=$st"
  if [[ ! $st =~ ^[RS] ]]; then
    log "Healthcheck: PID $pid 비정상 상태($st)"
    all_healthy=false
  else
    log "Healthcheck: PID $pid 정상 상태"
  fi
done

# 3) 종료 코드 파일 읽기 (공유 잠금)
exit_code=""
if [ -s "$EXIT_CODE_LOG_FILE" ]; then
  # 잠금 파일이 없다면 생성
  touch "$LOCK_FILE"

  {
    flock -s 200
    while IFS= read -r line; do
      log "읽은 줄: $line"
      if [[ $line == exit_code:* ]]; then
        exit_code=${line#exit_code:}
        log "추출된 exit_code=$exit_code"
        break
      fi
    done < "$EXIT_CODE_LOG_FILE"
  } 200>"$LOCK_FILE"
fi

# 4) 최종 헬스 체크
if [[ -z "$exit_code" ]]; then
  # exit_code 파일이 없거나 비어 있음
  if $all_healthy; then
    log "Healthcheck: 실행 중(정상), exit_code 없음"
    exit 0
  else
    log "Healthcheck: 실행 중(비정상), exit_code 없음"
    exit 1
  fi
else
  # exit_code 가 존재할 때
  if [[ "$exit_code" -eq 0 && $all_healthy == true ]]; then
    log "Healthcheck: 정상 종료(exit_code=0)"
    exit 0
  else
    log "Healthcheck: 종료 코드($exit_code) 또는 프로세스 비정상"
    exit 1
  fi
fi
