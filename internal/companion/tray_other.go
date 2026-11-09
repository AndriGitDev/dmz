//go:build !windows

package companion

import (
	"context"
	"errors"
)

func RunStatusIcon(context.Context, string) error { return errors.New("tray_unavailable") }
