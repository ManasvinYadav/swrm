//go:build !linux && !darwin

package engine

import "fmt"

// bindToInterface fails loudly on platforms swrm doesn't support raw
// interface binding on, rather than silently skipping the VPN kill-switch.
func bindToInterface(_ uintptr, ifaceName string, _ int) error {
	return fmt.Errorf("interface binding to %q is not supported on this platform", ifaceName)
}
