package podbridge5

import (
	"context"
	"fmt"
	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/common/pkg/config"
	imageTypes "github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/storage/pkg/unshare"
	"os"
	"testing"
	"time"
)

/*
buildah 의 메서드를 사용할 경우 먼저 알아야 할 것이 rootless 사용해서 테스트를 진행하는 것은 상당한 오류를 발생할 수 있다
rootless 모드로 테스트를 진행할 경우는 main.go 에서 테스트를 진행해야 하는 것이 시간 절약을 할 수 있고, 여러가지 문제를 경험하지 않을 수 있다.
이것때문에 많은 시간을 허비하였다. 따라서 image_test.go 같은 경우 buildah 메서드를 사용하는 경우는 test 코드를 사용하지 않는다.

또한 main.go 에서 테스트를 진행한다고 했을 경우도 break point 가 적용되지 않는다. 그 이유는 rootless 모드 이기 때문에 재시작 할 경우 debugger 가 break point 를 찾아가지 못하는 문제가 있다.
따라서, 이때는 로그를 통한 디버깅을 진행해야 한다.

이러한 부분은 상당히 거슬리고 습관적으로 디버깅을 하다가 시간을 허비하는 경우가 많다. 항상 주의 해야한다.

물론 root 모드로 할 경우는 위의 문제들은 발생하지 않는다. 이때 만약 goland 를 사용할 경우 root 로 실행하면 된다.

*/

func TestNewStore(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Errorf("NewStore() failed")
	}
	if store == nil {
		t.Errorf("store is nil")

	}
}

func TestBuildDockerfile01(t *testing.T) {

	//InitForBuildah()

	// Initialize the context
	ctx := context.Background()

	ctx, err := NewConnectionLinux5(ctx)
	if err != nil {
		t.Fatalf("connection error: %v", err)
	}

	// Create a temporary storage store
	//store, err := NewStore()

	//defer store.Shutdown(false)
	/*if err != nil {
		t.Fatalf("error creating store: %v", err)
	}*/

	// Define build options
	pullPolicy := define.PullIfMissing
	systemContext := &imageTypes.SystemContext{}
	isolation := define.IsolationOCIRootless

	// 로그 파일 설정
	logFile, err := os.OpenFile("build.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("error opening log file: %v", err)
	}
	defer logFile.Close()

	timestamp := time.Now().UTC()
	jobs := 2
	// 새로운 BuildOptions 구조체를 생성
	options := types.BuildOptions{
		BuildOptions: define.BuildOptions{
			AdditionalTags:          []string{"v1.0", "latest"},
			AllPlatforms:            false,
			Annotations:             []string{"author=developer"},
			Architecture:            "amd64",
			Args:                    map[string]string{"MY_ENV_VAR": "value"},
			BlobDirectory:           "/tmp/blob-cache",
			Compression:             define.Gzip,
			ContextDirectory:        ".",
			DefaultMountsFilePath:   "/etc/containers/mounts.conf",
			Devices:                 []string{},
			Err:                     os.Stderr,
			ForceRmIntermediateCtrs: true,
			From:                    "docker.io/library/alpine:latest",
			IDMappingOptions:        nil,
			IIDFile:                 "image-id-file",
			In:                      os.Stdin,
			Isolation:               isolation,
			IgnoreFile:              ".dockerignore",
			Labels:                  []string{"version=1.0"},
			Layers:                  true,
			LogRusage:               true,
			Manifest:                "",
			MaxPullPushRetries:      3,
			NoCache:                 false,
			OS:                      "linux",
			Out:                     os.Stdout,
			Output:                  "myimage:latest",
			BuildOutput:             "tarball.tar",
			OutputFormat:            define.Dockerv2ImageManifest,
			PullPolicy:              pullPolicy,
			PullPushRetryDelay:      2 * time.Second,
			Quiet:                   false,
			RemoveIntermediateCtrs:  true,
			ReportWriter:            logFile,
			Runtime:                 "runc",
			RuntimeArgs:             []string{"--log", "/var/log/runc.log"},
			RusageLogFile:           "rusage.log",
			SignBy:                  "",
			SignaturePolicyPath:     "",
			Squash:                  true,
			SystemContext:           systemContext,
			OciDecryptConfig:        nil,
			Jobs:                    &jobs,
			Excludes:                []string{"build/*"},
			Timestamp:               &timestamp,
			Platforms:               []struct{ OS, Arch, Variant string }{{OS: "linux", Arch: "amd64", Variant: ""}},
			UnsetEnvs:               []string{"UNUSED_ENV"},
			Envs:                    []string{"MY_ENV=prod"},
			OSFeatures:              []string{"seccomp"},
			OSVersion:               "10.0",
		},
		ContainerFiles:   []string{"Dockerfile"},   // Dockerfile 경로를 추가
		FarmBuildOptions: types.FarmBuildOptions{}, // FarmBuildOptions 초기화
		LogFileToClose:   logFile,                  // 로그 파일
		TmpDirToClose:    "",                       // 임시 디렉토리 설정 (필요 시)
	}

	// Path to the Dockerfile
	dockerfilePath := "Dockerfile"
	// Create or append to containerFiles slice
	containerFiles := []string{dockerfilePath}

	// Build the Dockerfile
	r, err := images.Build(ctx, containerFiles, options)
	//id, ref, err := imagebuildah.BuildDockerfiles(ctx, store, options, dockerfilePath)
	if err != nil {
		t.Fatalf("error building Dockerfile: %v", err)
	}

	// Output the image ID and reference
	fmt.Printf("Built image ID: %s\n", r.ID)

}

func TestBabo(t *testing.T) {
	if buildah.InitReexec() {
		return
	}
	// Check if the test is already running in a user namespace
	/*if unshare.IsRootless() {
		t.Skip("Skipping test in rootless mode to avoid os.Exit() during testing")
	}*/

	unshare.MaybeReexecUsingUserNamespace(false)
	ctx := context.Background()
	ctx, _ = NewConnectionLinux5(ctx)

	/*builderOption := buildah.BuilderOptions{
		FromImage: "35a88802559d",
		Isolation: buildah.IsolationOCI,
	}*/

	store, err := NewStore()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	defer store.Shutdown(false)

	conf, err := config.Default()
	if err != nil {
		panic(err)
	}
	capabilitiesForRoot, err := conf.Capabilities("root", nil, nil)
	if err != nil {
		panic(err)
	}

	builderOpts := buildah.BuilderOptions{
		FromImage:    "docker.io/library/ubuntu:latest",
		Capabilities: capabilitiesForRoot,
	}

	builder, err := buildah.NewBuilder(context.TODO(), store, builderOpts)
	if err != nil {
		panic(err)
	}
	defer builder.Delete()

	isolation, err := parse.IsolationOption("")
	if err != nil {
		panic(err)
	}

	/*runOptions := buildah.RunOptions{
		Isolation: isolation,
		Terminal:  buildah.WithoutTerminal,
	}*/

	err = builder.Run([]string{"echo", "helloworld"}, buildah.RunOptions{
		Isolation: isolation,
		Terminal:  buildah.WithoutTerminal,
	})
	if err != nil {
		panic(err)
	}

	/*builder, err := buildah.NewBuilder(ctx, store, builderOption)
	defer builder.Delete()*/
	/*runOptions := buildah.RunOptions{}
	var cmd = []string{"apt-get", "update", "-y"}*/
	/*err = builder.Run(cmd, runOptions)
	if err != nil {
		t.Fatalf("error: %v", err)
	}*/

	/*if err == nil {
		fmt.Println("success")
	}*/
}
