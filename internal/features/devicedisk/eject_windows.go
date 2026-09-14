//go:build windows

package devicedisk

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Windows Explorer owns non-elevated removable-device eject. Invoke its
// canonical Eject verb rather than opening the volume with elevated access.
func ejectVolume(ctx context.Context, root string) error {
	drive := filepath.VolumeName(root)
	if len(drive) != 2 || drive[1] != ':' {
		return fmt.Errorf("invalid drive %q", drive)
	}
	script := fmt.Sprintf(`$item=(New-Object -ComObject Shell.Application).Namespace(17).ParseName('%s'); if ($null -eq $item) { exit 2 }; $item.InvokeVerb('Eject')`, drive)
	output, err := execCommand(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("request Explorer eject for %s: %w: %s", drive, err, strings.TrimSpace(string(output)))
	}
	return nil
}
