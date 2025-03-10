package podbridge5

import (
	"github.com/containers/buildah"
	"github.com/containers/storage/pkg/unshare"
)

// InitForBuildah initializes buildah for rootless mode. 사용하지 않음.
// TODO 이렇게 하면 에러 남. 그냥 메서드만 빠져나감. 다시 시작 되지 않음. 경고를 위해서 남겨둠.
func InitForBuildah() {
	Log.Info("Initializing buildah for rootless mode")
	if buildah.InitReexec() {
		Log.Info("Reexec initiated")
		return
	}
	Log.Info("Proceeding with MaybeReexecUsingUserNamespace")
	unshare.MaybeReexecUsingUserNamespace(false)
	Log.Info("Initialization complete")
}
