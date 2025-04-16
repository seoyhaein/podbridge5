package podbridge5

import (
	"github.com/containers/podman/v5/pkg/specgen"
	specgo "github.com/opencontainers/runtime-spec/specs-go"
)

// TODO 관련 정보를 가져와서 구체적으로 WithMount 을 사용할 수 있도록 하는 api 가 필요하다.

// 주의사항 source, destination 은 이미 생성되어 있다고 가정한다.

func WithMount(source, destination, mountType string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		// 만약 spec.Mounts 필드가 nil 이면 초기화
		if spec.Mounts == nil {
			spec.Mounts = []specgo.Mount{}
		}
		// mountType 은 보통 "bind"로 설정
		spec.Mounts = append(spec.Mounts, specgo.Mount{
			Type:        mountType,   // 예: "bind"
			Source:      source,      // 호스트 경로
			Destination: destination, // 컨테이너 내부 경로
		})
		return nil
	}
}
