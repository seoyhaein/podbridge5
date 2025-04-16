package podbridge5

import (
	"context"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	entitiesTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
)

// 주의 사항. 여기서 volumeName 생성이 전제되어 있다고 봐야 한다.

// WithNamedVolume 네임드 볼륨 마운트 옵션
func WithNamedVolume(volumeName, dest, subPath string, options ...string) ContainerOptions {
	return func(spec *specgen.SpecGenerator) error {
		// volumeName 이 비어있으면 익명 볼륨(anonymous volume)으로 처리
		isAnonymous := false
		// TODO 아래 내용 확인 해야함.
		// 익명 보륨의 경우,일반적으로 익명 볼륨은 특정 컨테이너에 종속되어 사용되며, 해당 컨테이너가 삭제되면 함께 정리 되어야함.
		// podman rm -v 이러한 방식으로 볼륨을 지울 수 있음.
		if volumeName == "" {
			isAnonymous = true
		}

		// 새로운 NamedVolume 객체 생성 (SubPath 필드 포함)
		newVol := &specgen.NamedVolume{
			Name:        volumeName,
			Dest:        dest,
			Options:     options,
			IsAnonymous: isAnonymous,
			SubPath:     subPath,
		}

		// 이미 같은 Dest 설정되어 있는지 확인
		for _, vol := range spec.Volumes {
			if vol.Dest == dest {
				// 같은 Dest 이미 설정되어 있으면 덮어쓰지 않고 그냥 반환
				return nil
			}
		}

		// SpecGenerator 의 Volumes 필드에 새 볼륨 추가
		spec.Volumes = append(spec.Volumes, newVol)
		return nil
	}
}

// CreateVolume 주어진 볼륨 이름을 기반으로 볼륨 만들어줌. ignoreIfExists true 이면, 동일한 볼륨이 있으면 에러 리턴하지 않고 그대로 사용.
func CreateVolume(ctx context.Context, volumeName string, ignoreIfExists bool) (*entitiesTypes.VolumeConfigResponse, error) {
	volConfig := entitiesTypes.VolumeCreateOptions{
		Name:           volumeName,
		IgnoreIfExists: ignoreIfExists, // 만약 true 이면, 동일한 이름의 볼륨이 있으면 생성하지 않고 기존 볼륨을 사용
	}

	// CreateOptions 객체, 현재 버전에서는 빈 객체임.
	createOptions := &volumes.CreateOptions{}

	// volumes.Create 함수를 호출하여 볼륨 생성
	volumeResp, err := volumes.Create(ctx, volConfig, createOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	return volumeResp, nil
}

// TODO 추가적으로 만들어줘야 함.
