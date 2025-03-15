package main

import (
	"fmt"
	"github.com/containers/buildah"
	pbr "github.com/seoyhaein/podbridge5"
	"log"
	"os"
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
	defer func() {
		err := pbr.Shutdown()
		if err != nil {

		}
	}()
	// 일단 에러 발생하니까. 이렇게 만들어줌.
	// ctx := context.Background()

	// 여기서 부터는 컨테이너 나 이제 이미지 파트이다.
	// 컨테이너나 이미지는 여러개 생성 및 삭제가 가능하다라는 사실을 잊으면 안된다.
	/*store, err := pbr.NewStore()
	if err != nil {
		log.Fatalf("Failed to create store: %v\n", err)
	}*/

	// 여기서는 defer 를 해줬지만 컨테이너 레벨에서는 defer 를 해당 컨테이너를 제작하는 메서드에서 해주면 됨.
	// 여기서는 defer 에서 처리하는 메서드의 에러처리를 하지 않는다. 필요없어서.
	//defer pbr.ShutDown(store, false)
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

	_, _, err = pbr.GenerateExecutor(".", "executor.sh", "./scripts/user_script.sh")

	if err != nil {
		log.Printf("Failed to generate executor: %v\n", err)
		os.Exit(1)
	}
	config, err := pbr.NewConfigFromFile("config.json")
	if err != nil {
		log.Printf("Failed to create config: %v\n", err)
		os.Exit(1)
	}
	imageName := "docker.io/library/alpine:latest"
	config.SetSourceImageNameAndImageName(imageName)

	// CreateImage3 는 임시 메서드임.
	buildahBuilder, imageId, err := config.CreateImage3()
	if err != nil {
		log.Printf("Failed to create image: %v\n", err)
		os.Exit(1)
	}
	defer func(buildahBuilder *buildah.Builder) {
		buildahErr := buildahBuilder.Delete()
		if buildahErr != nil {
			log.Printf("Failed to delete builder: %v", buildahErr)
		}
	}(buildahBuilder)

	fmt.Printf("Building image: %s\n", imageId)

}
