//go:build !windows

package devicedisk

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

func ejectVolume(ctx context.Context, root string) error {
	name, args := "eject", []string{root}
	if runtime.GOOS == "darwin" {
		name, args = "diskutil", []string{"eject", root}
	}
	output, err := execCommand(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}
