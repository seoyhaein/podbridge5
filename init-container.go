package podbridge5

import (
	"context"
	"fmt"
)

// TODO 일단 간단하게 overlayfs 가 잘 생성이 되고 잘 전파가 되는지만 먼저 확인하고, 다시 구현해야 함.
// TODO Pod 생성한 다음에, 제일 처음에는 init container 를 실행해서 overlayfs 를 생성하는 작업을 해야 한다.

// CreateInitContainer sets up an overlay mount in the pod's mount namespace.
// It launches a privileged container that executes the mount command, then sleeps indefinitely.
func CreateInitContainer(ctx context.Context, podID, lowerdir, upperdir, workdir, target string) (string, error) {
	spec, err := NewSpec(
		WithPod(podID),
		WithName("init-container"),
		WithImageName("docker.io/library/alpine:latest"), // Use a lightweight image for the init container
		WithSysAdmin(),
		WithUnconfinedSeccomp(),
		WithCommand([]string{"sh", "-c",
			fmt.Sprintf(
				"mount -t overlay lowerdir=%s,upperdir=%s,workdir=%s %s && sleep infinity",
				lowerdir, upperdir, workdir, target),
		}),
	)
	if err != nil {
		return "", fmt.Errorf("failed to build mount-init spec: %w", err)
	}
	return StartContainer(ctx, spec)
}
