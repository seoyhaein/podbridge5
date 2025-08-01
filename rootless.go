package podbridge5

import (
	"context"
	"fmt"
	"github.com/containers/buildah"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
	"sync"
)

// 일단 초안 부터 시작하자.

var (
	once    sync.Once
	pbStore storage.Store
	pbCtx   context.Context
)

// 전체적인 methods
// Init, InitWithContext 반드시 둘중 하나만 사용해야함.

func Init() error {
	var err error
	once.Do(func() {
		// podman 연결 초기화
		pbCtx, err = NewConnectionLinux5(context.Background())
		if err != nil {
			err = fmt.Errorf("failed to initialize podman connection: %v", err)
			return
		}
		// 내부 전역 변수에 할당한다.
		pbStore, err = NewStore()
		if err != nil {
			err = fmt.Errorf("failed to create store: %v", err)
			return
		}
	})
	return err
}

// InitWithContext 필요 없을 수 있음.
func InitWithContext(ctx context.Context) (context.Context, error) {
	var err error
	once.Do(func() {
		ctx, err = NewConnectionLinux5(ctx)
		if err != nil {
			err = fmt.Errorf("failed to initialize podman connection: %v", err)
			return
		}
		pbStore, err = NewStore()
		if err != nil {
			err = fmt.Errorf("failed to create store: %v", err)
			return
		}
	})
	return ctx, err
}

// Shutdown app main 에서 defer 로 처리해야함. TODO 이거 낭비인거 같은데 확인하자.
func Shutdown() error {
	if pbStore == nil {
		return fmt.Errorf("pbStore is nil")
	}
	return shutdown(pbStore, false)
}

// 추가적인 수정도 생각해볼 수 있음.
// 컨테이너 하나 생성하는 것을 생각해봐야 함.
// 컨테이너 methods
// TODO 여기서 부터 시작. 컨테이너 및 이미지 외부 노출 api 최대한 단순하게 메서드 하나로 끝낼 수 있는 방향으로.

// ReexecIfNeeded 는 rootless 모드라면 reexec 를 수행합
// 만약 reexec 가 필요하면, 이 함수는 true 를 반환하고, 프로세스는 재실행되며 현재 프로세스는 종료
// 반드시 init 메서드에서 실행하거나 main 메서드에서 실행해야해?
func ReexecIfNeeded() bool {
	if buildah.InitReexec() {
		return true
	}
	unshare.MaybeReexecUsingUserNamespace(false)
	return false
}
