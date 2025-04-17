package podbridge5

import (
	"fmt"
	"github.com/containers/podman/v5/pkg/specgen"
	specgo "github.com/opencontainers/runtime-spec/specs-go"
	"os"
)

// WithMount 컨테이너에서 bind mount 같은 경우 마운트할 디렉토리가 없으면 자동으로 만들어줌. 또한 stop 이나 remove 하면 자동으로 unmount 됨.
func WithMount(source, destination, mountType string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		// 1) 호스트 경로가 실제로 있는지 검사
		if fi, err := os.Stat(source); err != nil {
			return fmt.Errorf("host path %q does not exist: %w", source, err)
		} else if !fi.IsDir() {
			return fmt.Errorf("host path %q exists but is not a directory", source)
		}
		// 2) spec.Mounts 초기화
		if spec.Mounts == nil {
			spec.Mounts = []specgo.Mount{}
		}
		// 3) mountType("bind" 등)과 source, destination 설정
		spec.Mounts = append(spec.Mounts, specgo.Mount{
			Type:        mountType,   // "bind"
			Source:      source,      // 호스트 경로
			Destination: destination, // 컨테이너 내부 경로
		})
		return nil
	}
}
