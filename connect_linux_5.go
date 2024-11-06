//This file now only builds on Linux.
//go:build linux
// +build linux

package podbridge5

import (
	"context"
	"errors"
	"fmt"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/storage/pkg/unshare"
	"github.com/seoyhaein/utils"
	"os"
)

func NewConnection5(ctx context.Context, ipcName string) (context.Context, error) {

	if utils.IsEmptyString(ipcName) {
		Log.Error("ipcName cannot be an empty string")
		return nil, errors.New("ipcName cannot be an empty string")
	}
	ctx, err := bindings.NewConnection(ctx, ipcName)

	return ctx, err
}

func NewConnectionLinux5(ctx context.Context) (context.Context, error) {

	socket := defaultLinuxSockDir5()
	ctx, err := bindings.NewConnection(ctx, socket)

	return ctx, err
}

func defaultLinuxSockDir5() (socket string) {
	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		if unshare.IsRootless() {
			// Non-root user
			sockDir = fmt.Sprintf("/run/user/%d", os.Getuid())
		} else {
			// Root user
			sockDir = "/run"
		}
	}
	socket = "unix:" + sockDir + "/podman/podman.sock"
	return
}
