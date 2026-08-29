package engine

import "testing"

func TestNewVpnManagerOptionalInterface(t *testing.T) {
	ch := make(chan VpnState, 1)
	v, err := NewVpnManager("", ch)
	if err != nil {
		t.Fatalf("expected no error for an empty interface, got %v", err)
	}
	if !v.IsActive() {
		t.Fatal("expected a manager with no interface to stay active: there's nothing to watch or lose")
	}
	if v.InterfaceName != "" {
		t.Fatalf("expected empty InterfaceName, got %q", v.InterfaceName)
	}
	dialer := v.Dialer()
	if dialer.Control == nil {
		t.Fatal("expected a non-nil Control func even with no interface configured")
	}
}
