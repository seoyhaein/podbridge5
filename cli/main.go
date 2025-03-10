package main

import (
	"context"
	"fmt"
	pbr "github.com/seoyhaein/podbridge5"
	"log"
	"os"
	"time"
)

// for testing
// TODO: 이거 이렇게 테스트 하는 것에 대한 문서화 하자. 향후 책으로 활용할 예정 임.
// TODO: 이거 디버깅 안되서 아래 코딩 디버깅 할려면 root 모드에서 해야함.
// rootless 에서 디버깅 할려면, root 모드에서 진행해야 하는데, 이것 또한 불편하다. 일단 신중히 해야함.
// 아래 내용 문서화 해놓자.
// 사용예시를 보여주고 있음. podman 과의 접속이 끊어지면 빠르게 종료하게 하는 전략을 삼음.
// Fail-fast 전략:
// pbr.Init() 호출 후 오류가 발생하면 os.Exit(1)로 프로세스를 종료하여, 잘못된 환경에서 실행되지 않도록함.
// 그리고 내부적으로 전역 변수로 잡아놓아서 리턴을 사용하지 않아도 됨.
/*
var (
	pbStore storage.Store
	pbCtx   context.Context
	once    sync.Once
)
*/
// 코드 지저분해저도 주석 지우지 않는다. 참고 사항.

func init() {
	if pbr.ReexecIfNeeded() {
		os.Exit(0)
	}
	/*_, err := pbr.Init()
	if err != nil {
		os.Exit(1)
	}*/

	// context 를 살라지 않는 이유는 어차피 에러나면 종료되서 podman 설정을 확인하는 것이 더 중요하기 때문이다.
	// 비정상적으로 연결되어서 생각지 못한 side effect 방지를 위해서.
	if err := pbr.Init(); err != nil {
		log.Fatalf("Podman connection initialization failed, exiting: %v", err)
	}
}

func main() {

	// 메서드로 만들면 위험 함.
	/*if buildah.InitReexec() {
		return
	}
	unshare.MaybeReexecUsingUserNamespace(false)*/

	/*ctx, err := pbr.NewConnectionLinux5(context.Background())
	if err != nil {
		log.Fatalf("Failed to establish connection: %v\n", err)
	}*/

	// 일단 에러 발생하니까. 이렇게 만들어줌.
	ctx := context.Background()

	// 여기서 부터는 컨테이너 나 이제 이미지 파트이다.
	// 컨테이너나 이미지는 여러개 생성 및 삭제가 가능하다라는 사실을 잊으면 안된다.
	store, err := pbr.NewStore()
	if err != nil {
		log.Fatalf("Failed to create store: %v\n", err)
	}

	// 여기서는 defer 를 해줬지만 컨테이너 레벨에서는 defer 를 해당 컨테이너를 제작하는 메서드에서 해주면 됨.
	// 여기서는 defer 에서 처리하는 메서드의 에러처리를 하지 않는다. 필요없어서.
	defer pbr.ShutDown(store, false)
	//defer store.Shutdown(false)

	// TODO 따로 깔끔하게 정리할 필요가 있음.
	testScript := `#!/usr/bin/env bash

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

exit 0`

	// user_script 만들어줌.
	filepath, err := pbr.ProcessScript(testScript, "./scripts/")
	if err != nil {
		log.Fatalf("Failed to process script: %v\n", err)
	}
	// 파일 경로 출력
	fmt.Println("Script generated at:", filepath)

	pbr.GenerateExecutor(".", "executor.sh", "./scripts/user_script.sh")

	// TODO: 문서화 하고 docker.io/library 나 localhost 등의 설정등도 담자.
	// TODO: 이름을 UserSettings 라고 잡아두자. 고정되는 값들도 있다. 고정되는 값들은 외부에서 접근 안되도록 하는 것을 생각하자.
	// TODO: (중요)경로의 문제가 있다. 지금 경로는 소스 위치에 따른 경로로 잡힌다. 다만 향후 main 내용이 합쳐질 것이기 때문에 그때 생각해보자
	config := &pbr.BuildConfig{
		SourceImageName:  "docker.io/library/alpine:latest",
		HealthcheckDir:   "/app/healthcheck",
		ImageSavePath:    "/opt/images",
		HealthcheckShell: "./healthcheck.sh",
		DockerfilePath:   "./Dockerfile",
		ImageName:        "tester",
		ExecutorShell:    "./executor.sh",
		UserScriptShell:  "./scripts/user_script.sh",
		InstallShell:     "./install.sh",
	}

	//builder, imageId, err := config.CreateImageWithDockerfile(ctx, store)
	builder, imageId, err := config.CreateImage(ctx, store)
	if err != nil {
		log.Fatalf("Failed to create image: %v\n", err)
	}

	containerId, err := pbr.RunContainer(ctx, imageId, "testContainer", true)
	if err != nil {
		log.Fatalf("Failed to create container: %v\n", err)
	}

	defer builder.Delete()

	fmt.Println(containerId)

	// 로그 파일 생성
	logFile, err := os.Create("container_status.log")
	if err != nil {
		log.Fatalf("Failed to create log file: %v\n", err)
	}
	defer logFile.Close()

	// 컨테이너 상태 모니터링 루프
	for {
		fmt.Println("start")
		containerData, err := pbr.InspectContainer(ctx, containerId)
		if err != nil {
			log.Printf("Error getting container info: %v\n", err)
			break
		}

		// 컨테이너 상태 로그 기록
		status := containerData.State.Status
		logLine := fmt.Sprintf("Time: %s, Status: %s\n", time.Now().Format(time.RFC3339), status)
		fmt.Println(logLine)
		logFile.WriteString(logLine)
		// HealthCheckResults 값을 로그에 출력
		if status == "exited" || status == "stopped" {
			logFile.WriteString("---Container has exited or stopped---")
			if containerData.State.Health != nil {
				health := containerData.State.Health
				healthLine := fmt.Sprintf("Health Status: %s, FailingStreak: %d\n", health.Status, health.FailingStreak)
				fmt.Println(healthLine)
				logFile.WriteString(healthLine)

				// HealthCheckLog 출력
				for _, logEntry := range health.Log {
					/*healthLine = fmt.Sprintf("Health Status: %s, FailingStreak: %d\n", health.Status, health.FailingStreak)
					fmt.Println(healthLine)
					logFile.WriteString(healthLine)*/

					logEntryLine := fmt.Sprintf("Log - Start: %s, End: %s, ExitCode: %d, Output: %s\n",
						logEntry.Start,
						logEntry.End,
						logEntry.ExitCode,
						logEntry.Output)
					fmt.Println(logEntryLine)
					logFile.WriteString(logEntryLine)
				}

			}
			break
		} else {
			logFile.WriteString("---Container is running or sleeping---")
			if containerData.State.Health != nil {
				health := containerData.State.Health
				healthLine := fmt.Sprintf("Health Status: %s, FailingStreak: %d\n", health.Status, health.FailingStreak)
				fmt.Println(healthLine)
				logFile.WriteString(healthLine)

				// HealthCheckLog 출력
				for _, logEntry := range health.Log {
					/*healthLine = fmt.Sprintf("Health Status: %s, FailingStreak: %d\n", health.Status, health.FailingStreak)
					fmt.Println(healthLine)
					logFile.WriteString(healthLine)*/

					logEntryLine := fmt.Sprintf("Log - Start: %s, End: %s, ExitCode: %d, Output: %s\n",
						logEntry.Start,
						logEntry.End,
						logEntry.ExitCode,
						logEntry.Output)
					fmt.Println(logEntryLine)
					logFile.WriteString(logEntryLine)
				}
			}

			// 컨테이너가 종료되었는지 확인
			/*if status == "exited" || status == "stopped" {
				log.Println("Container has exited.")

				status := containerData.State.Status
				logLine := fmt.Sprintf("Time: %s, Status: %s\n", time.Now().Format(time.RFC3339), status)
				fmt.Println(logLine)
				logFile.WriteString(logLine)
				break
			}*/

			// 일정 시간 대기 후 재확인
			time.Sleep(1 * time.Second)
		}
	}
}
