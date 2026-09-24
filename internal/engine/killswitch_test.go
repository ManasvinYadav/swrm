//go:build linux

package engine

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
)

// tunnelStandIn picks an up, non-loopback interface with a usable IPv4
// address to play the part of the VPN interface, or skips the test.
func tunnelStandIn(t *testing.T) *VpnManager {
	t.Helper()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Skipf("list interfaces: %v", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		vm, err := NewVpnManager(iface.Name, nil)
		if err != nil {
			continue
		}
		if _, err := vm.IPv4Address(); err == nil {
			return vm
		}
	}
	t.Skip("no up, non-loopback interface with an IPv4 address to bind to")
	return nil
}

// TestKillSwitchBindsPeerAndUDPTrackerTraffic is a regression test for two
// kill-switch leaks: outgoing TCP peer connections went through the torrent
// library's own unbound net.Dialer, and UDP tracker announces through an
// unbound net.ListenPacket(":0"), so both left over whatever route the OS
// picked rather than the configured interface.
//
// Loopback addresses are only reachable via lo, so with every socket bound to
// another interface nothing may ever arrive at the 127.0.0.1 peer/tracker.
// The same pair on the bound interface's own address must still be reached;
// that also tells us when the client has had its chance to leak.
func TestKillSwitchBindsPeerAndUDPTrackerTraffic(t *testing.T) {
	vm := tunnelStandIn(t)
	tunIP, _ := vm.IPv4Address()

	// SO_BINDTODEVICE needs CAP_NET_RAW on kernels before 5.7.
	probe, err := net.Listen("tcp4", net.JoinHostPort(tunIP.String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	if c, err := vm.Dialer().Dial("tcp4", probe.Addr().String()); err != nil {
		t.Skipf("interface binding not permitted here: %v", err)
	} else {
		c.Close()
	}

	type target struct {
		peer    net.Listener
		tracker net.PacketConn
		hits    chan string
	}
	listen := func(ip string) target {
		peer, err := net.Listen("tcp4", net.JoinHostPort(ip, "0"))
		if err != nil {
			t.Fatal(err)
		}
		tracker, err := net.ListenPacket("udp4", net.JoinHostPort(ip, "0"))
		if err != nil {
			t.Fatal(err)
		}
		tg := target{peer, tracker, make(chan string, 2)}
		go func() {
			if c, err := peer.Accept(); err == nil {
				tg.hits <- "TCP peer connection"
				c.Close()
			}
		}()
		go func() {
			if _, _, err := tracker.ReadFrom(make([]byte, 2048)); err == nil {
				tg.hits <- "UDP tracker announce"
			}
		}()
		t.Cleanup(func() { peer.Close(); tracker.Close() })
		return tg
	}
	inTunnel := listen(tunIP.String())
	offTunnel := listen("127.0.0.1")

	eng, err := NewEngine(vm, t.TempDir(), Options{DownloadDir: t.TempDir(), ListenPort: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	magnet := fmt.Sprintf("magnet:?xt=urn:btih:%040x&tr=udp://%s/announce&tr=udp://%s/announce",
		0x5eed, inTunnel.tracker.LocalAddr(), offTunnel.tracker.LocalAddr())
	tr, err := eng.AddMagnet(magnet)
	if err != nil {
		t.Fatal(err)
	}
	tr.AddPeers([]torrent.PeerInfo{{Addr: inTunnel.peer.Addr()}, {Addr: offTunnel.peer.Addr()}})

	for i := 0; i < 2; i++ {
		select {
		case <-inTunnel.hits:
		case leak := <-offTunnel.hits:
			t.Fatalf("%s reached a loopback address outside the bound interface", leak)
		case <-time.After(10 * time.Second):
			t.Fatal("no TCP peer connection and UDP tracker announce over the bound interface")
		}
	}
	select {
	case leak := <-offTunnel.hits:
		t.Fatalf("%s reached a loopback address outside the bound interface", leak)
	case <-time.After(time.Second):
	}
}
