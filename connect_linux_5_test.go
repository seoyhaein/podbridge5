package podbridge5

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestNewConnection5(t *testing.T) {
	sockDir := fmt.Sprintf("/run/user/%d", os.Getuid())
	fmt.Println(sockDir)
}

func TestConnection(t *testing.T) {
	_, err := NewConnectionLinux5(context.Background())
	if err != nil {
		fmt.Println("Error: ", err)
	}
}
