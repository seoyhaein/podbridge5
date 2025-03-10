package podbridge5

import (
	"github.com/containers/buildah"
	"github.com/containers/storage/pkg/unshare"
)

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
