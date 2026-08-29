package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"
)

type VpnState int

const (
	StateActive VpnState = iota
	StateHaltedLeakPrevention
)

type VpnManager struct {
	InterfaceName string
	InterfaceIdx  int
	State         VpnState
	mu            sync.RWMutex
	cancel        context.CancelFunc
	stateChan     chan VpnState
}

func NewVpnManager(ifaceName string, stateChan chan VpnState) (*VpnManager, error) {
	if ifaceName == "" {
		// Optional binding rule: no interface means bypass raw socket control
		// and use standard system routing. The manager stays permanently
		// active since there's no interface to watch or lose.
		return &VpnManager{State: StateActive, stateChan: stateChan}, nil
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}
	if iface.Flags&net.FlagUp == 0 {
		return nil, fmt.Errorf("interface %s is down", ifaceName)
	}

	return &VpnManager{
		InterfaceName: ifaceName,
		InterfaceIdx:  iface.Index,
		State:         StateActive,
		stateChan:     stateChan,
	}, nil
}

func (v *VpnManager) StartHeartbeat(ctx context.Context) {
	if v.InterfaceName == "" {
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.checkInterface()
		}
	}
}

func (v *VpnManager) checkInterface() {
	iface, err := net.InterfaceByName(v.InterfaceName)
	dropped := false
	if err != nil {
		dropped = true
	} else if (iface.Flags & net.FlagUp) == 0 {
		dropped = true
	} else if _, err := v.IPv4Address(); err != nil {
		dropped = true
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if dropped && v.State == StateActive {
		v.State = StateHaltedLeakPrevention
		if v.stateChan != nil {
			select {
			case v.stateChan <- v.State:
			default:
			}
		}
	}
	// A lost interface is terminal for this engine instance: its client and peer
	// sockets have already been closed. Restart explicitly after VPN recovery.
}

func (v *VpnManager) IsActive() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.State == StateActive
}

// IPv4Address returns a routable IPv4 address assigned to the selected VPN
// interface. Link-local and IPv6 addresses cannot be used for tcp4 listeners.
func (v *VpnManager) IPv4Address() (net.IP, error) {
	iface, err := net.InterfaceByName(v.InterfaceName)
	if err != nil {
		return nil, fmt.Errorf("interface %s not found: %w", v.InterfaceName, err)
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("list addresses for %s: %w", v.InterfaceName, err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP.To4()
		if ip == nil || ip.IsLinkLocalUnicast() {
			continue
		}
		return ip, nil
	}
	return nil, fmt.Errorf("interface %s has no usable IPv4 address", v.InterfaceName)
}

func (v *VpnManager) Dialer() *net.Dialer {
	return &net.Dialer{
		Control: func(network, address string, c syscall.RawConn) error {
			if v.InterfaceName == "" {
				return nil
			}

			v.mu.RLock()
			state := v.State
			v.mu.RUnlock()

			if state == StateHaltedLeakPrevention {
				return errors.New("VPN interface dropped; leak prevention active")
			}

			var sockErr error
			err := c.Control(func(fd uintptr) {
				sockErr = bindToInterface(fd, v.InterfaceName, v.InterfaceIdx)
			})
			if err != nil {
				return err
			}
			return sockErr
		},
	}
}
