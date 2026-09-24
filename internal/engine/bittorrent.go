package engine

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"
)

// TorrentSource records how a torrent entered the engine, for display in the
// UI (e.g. the switcher strip / diagnostics) rather than for any behavioral
// difference between the two.
type TorrentSource int

const (
	SourceMagnet TorrentSource = iota
	SourceFile
)

// TorrentHandle is the engine's tracked record for one torrent. Paused is
// kept here rather than queried from *torrent.Torrent because the library
// exposes no getter for its internal data-download-disallowed flag.
type TorrentHandle struct {
	T       *torrent.Torrent
	Source  TorrentSource
	AddedAt time.Time
	Paused  bool
}

type Engine struct {
	Client     *torrent.Client
	VpnManager *VpnManager
	Storage    storage.ClientImpl

	DLRateLimit     float64
	ULRateLimit     float64
	downloadLimiter *rate.Limiter
	uploadLimiter   *rate.Limiter

	postDownloadCmd string

	mu sync.RWMutex
	// torrents/order/highlightedIdx together replace the old singular
	// ActiveTorrent field: order is insertion order (drives the switcher
	// strip and Prev/Next cycling), highlightedIdx indexes into it, -1 when
	// empty.
	torrents       map[metainfo.Hash]*TorrentHandle
	order          []metainfo.Hash
	highlightedIdx int
}

type Options struct {
	DownloadDir                string
	ListenPort                 int
	DHT                        bool
	DownloadLimit, UploadLimit int
	// PostDownloadCmd is run through the shell once each torrent's selected
	// files have all finished downloading. Empty disables it.
	PostDownloadCmd string
}

func NewEngine(vpnMgr *VpnManager, downloadDir string, options ...Options) (*Engine, error) {
	if vpnMgr == nil || !vpnMgr.IsActive() {
		return nil, fmt.Errorf("VPN interface is not active")
	}
	opts := Options{DownloadDir: downloadDir, ListenPort: 6881, DHT: true}
	if len(options) > 0 {
		opts = options[0]
		if opts.DownloadDir == "" {
			opts.DownloadDir = downloadDir
		}
	}
	cfg := torrent.NewDefaultClientConfig()
	if opts.DownloadDir != "" {
		cfg.DataDir = opts.DownloadDir
	} else {
		cfg.DataDir = "./downloads"
	}

	cfg.ListenPort = opts.ListenPort
	ConfigureDHT(cfg, opts.DHT)
	bound := vpnMgr.InterfaceName != ""
	if bound {
		// torrent may initialize tcp4/udp4 listeners. Bind those sockets only to
		// a real IPv4 address, never an IPv6 (especially fe80:: link-local) one.
		ipv4, err := vpnMgr.IPv4Address()
		if err != nil {
			return nil, fmt.Errorf("resolve VPN listener address: %w", err)
		}
		listenHost := ipv4.String()
		// We intentionally select an IPv4-only VPN address, so prevent the
		// client from attempting a later tcp6/udp6 listener with that address.
		cfg.DisableIPv6 = true
		cfg.ListenHost = func(network string) string {
			// The torrent library supplies tcp/tcp4/udp/udp4 here. A resolved
			// IPv4 address is valid for all of those IPv4-capable networks.
			return listenHost
		}
		// Everything below opens sockets the library would otherwise leave
		// unbound, i.e. routed over the real interface:
		//  - outgoing TCP peer connections use the library's own net.Dialer
		//    (no Control hook, no local address), so replace the listener
		//    sockets as dialers with interface-bound ones after NewClient;
		//  - UDP tracker announces fall back to net.ListenPacket(":0");
		//  - WebRTC (wss:// webtorrent trackers) gathers ICE candidates on
		//    every local interface and asks public STUN servers for the
		//    real public IP;
		//  - UPnP/NAT-PMP talks to the physical LAN's router, which can't
		//    forward ports for the tunnel anyway.
		cfg.DialForPeerConns = false
		cfg.TrackerListenPacket = vpnMgr.ListenPacket
		cfg.DisableWebtorrent = true
		cfg.NoDefaultPortForwarding = true
	}
	// No interface configured: bypass raw socket control and fall back to
	// standard system routing (0.0.0.0 / default gateway), per the optional
	// binding rule.
	storageImpl := NewStorage(opts.DownloadDir)
	cfg.DefaultStorage = storageImpl
	dial := vpnMgr.Dialer()
	cfg.HTTPDialContext = dial.DialContext
	cfg.TrackerDialContext = dial.DialContext
	// Rate limiters are mutable after client construction.
	dl := rate.NewLimiter(rate.Inf, 1<<20)
	ul := rate.NewLimiter(rate.Inf, 1<<20)
	cfg.DownloadRateLimiter, cfg.UploadRateLimiter = dl, ul

	client, err := torrent.NewClient(cfg)
	if err != nil && !cfg.DisableIPv6 && errors.Is(err, syscall.EAFNOSUPPORT) {
		// The host has no IPv6 stack at all (ipv6.disable=1, IPv4-only
		// containers). The library treats a failed tcp6/udp6 listener as
		// fatal rather than skipping it, so retry IPv4-only.
		cfg.DisableIPv6 = true
		client, err = torrent.NewClient(cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create torrent client: %w", err)
	}
	if bound {
		// DialForPeerConns is off, so the client has no peer dialers yet. The
		// uTP sockets are already bound to the VPN address via ListenHost and
		// dial from that same socket, so they're safe to reuse as-is; TCP gets
		// an interface-bound dialer in place of the library's unbound one.
		for _, l := range client.Listeners() {
			if d, ok := l.(torrent.Dialer); ok && strings.HasPrefix(d.DialerNetwork(), "udp") {
				client.AddDialer(d)
			}
		}
		peerDialer := vpnMgr.Dialer()
		peerDialer.KeepAlive = -1 // BitTorrent connections manage their own keep-alives.
		client.AddDialer(torrent.NetworkDialer{Network: "tcp4", Dialer: peerDialer})
	}

	e := &Engine{
		Client:          client,
		VpnManager:      vpnMgr,
		Storage:         storageImpl,
		downloadLimiter: dl,
		uploadLimiter:   ul,
		torrents:        make(map[metainfo.Hash]*TorrentHandle),
		highlightedIdx:  -1,
		postDownloadCmd: opts.PostDownloadCmd,
	}
	e.SetDownloadLimit(float64(opts.DownloadLimit))
	e.SetUploadLimit(float64(opts.UploadLimit))
	return e, nil
}

func (e *Engine) SetDownloadLimit(bytesPerSec float64) {
	e.DLRateLimit = bytesPerSec
	if bytesPerSec <= 0 {
		e.downloadLimiter.SetLimit(rate.Inf)
	} else {
		e.downloadLimiter.SetLimit(rate.Limit(bytesPerSec))
		e.downloadLimiter.SetBurst(max(16<<10, int(bytesPerSec)))
	}
}

func (e *Engine) SetUploadLimit(bytesPerSec float64) {
	e.ULRateLimit = bytesPerSec
	if bytesPerSec <= 0 {
		e.uploadLimiter.SetLimit(rate.Inf)
	} else {
		e.uploadLimiter.SetLimit(rate.Limit(bytesPerSec))
		e.uploadLimiter.SetBurst(max(16<<10, int(bytesPerSec)))
	}
}

// AddMagnet adds a torrent by magnet URI. It becomes the newly highlighted
// torrent.
func (e *Engine) AddMagnet(uri string) (*torrent.Torrent, error) {
	return e.addAndTrack(func() (*torrent.Torrent, error) { return e.Client.AddMagnet(uri) }, SourceMagnet)
}

// AddTorrentFile adds a torrent from a local .torrent file. It becomes the
// newly highlighted torrent.
func (e *Engine) AddTorrentFile(path string) (*torrent.Torrent, error) {
	return e.addAndTrack(func() (*torrent.Torrent, error) { return e.Client.AddTorrentFromFile(path) }, SourceFile)
}

// addAndTrack centralizes the VPN guard and dedupe/highlight bookkeeping
// both AddMagnet and AddTorrentFile need, so neither duplicates it.
func (e *Engine) addAndTrack(addFn func() (*torrent.Torrent, error), src TorrentSource) (*torrent.Torrent, error) {
	if !e.VpnManager.IsActive() {
		return nil, fmt.Errorf("VPN interface dropped; leak prevention active")
	}
	t, err := addFn()
	if err != nil {
		return nil, err
	}
	hash := t.InfoHash()
	e.mu.Lock()
	_, exists := e.torrents[hash]
	if !exists {
		e.torrents[hash] = &TorrentHandle{T: t, Source: src, AddedAt: time.Now()}
		e.order = append(e.order, hash)
	}
	e.highlightLocked(hash)
	e.mu.Unlock()
	if !exists && e.postDownloadCmd != "" {
		go e.runPostDownloadWhenComplete(t)
	}
	return t, nil
}

// runPostDownloadWhenComplete runs the configured post-download command once
// every file the user selected for t has finished downloading. It gives up
// if the torrent is closed first (engine shutdown or a VPN halt).
func (e *Engine) runPostDownloadWhenComplete(t *torrent.Torrent) {
	select {
	case <-t.GotInfo():
	case <-t.Closed():
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.Closed():
			return
		case <-ticker.C:
			// Nothing selected yet (the file-selection modal is still open,
			// or was dismissed) never counts as complete.
			if done, total := selectedProgress(t); total > 0 && done >= total {
				RunPostDownload(e.postDownloadCmd)
				return
			}
		}
	}
}

// selectedProgress sums completed and total bytes across t's selected files,
// i.e. those with a priority other than PiecePriorityNone. Requires info.
func selectedProgress(t *torrent.Torrent) (done, total int64) {
	for _, f := range t.Files() {
		if f.Priority() != torrent.PiecePriorityNone {
			total += f.Length()
			done += f.BytesCompleted()
		}
	}
	return done, total
}

// highlightLocked sets highlightedIdx to hash's position in order. Caller
// must hold mu.
func (e *Engine) highlightLocked(hash metainfo.Hash) {
	for i, h := range e.order {
		if h == hash {
			e.highlightedIdx = i
			return
		}
	}
}

// HighlightHash re-highlights an already-tracked torrent, e.g. once its
// metadata resolves. Reports whether hash was found.
func (e *Engine) HighlightHash(hash metainfo.Hash) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	before := e.highlightedIdx
	e.highlightLocked(hash)
	return e.highlightedIdx != before || (len(e.order) > 0 && e.order[e.highlightedIdx] == hash)
}

// Pause stops a torrent's data download and upload without removing it from
// the engine.
func (e *Engine) Pause(hash metainfo.Hash) error {
	e.mu.Lock()
	h, ok := e.torrents[hash]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown torrent")
	}
	h.T.DisallowDataDownload()
	h.T.DisallowDataUpload()
	e.mu.Lock()
	h.Paused = true
	e.mu.Unlock()
	return nil
}

// Resume re-allows data download and upload for a paused torrent.
func (e *Engine) Resume(hash metainfo.Hash) error {
	e.mu.Lock()
	h, ok := e.torrents[hash]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown torrent")
	}
	h.T.AllowDataDownload()
	h.T.AllowDataUpload()
	e.mu.Lock()
	h.Paused = false
	e.mu.Unlock()
	return nil
}

// TogglePauseHighlighted is what the UI's Space key calls.
func (e *Engine) TogglePauseHighlighted() error {
	h, ok := e.Highlighted()
	if !ok {
		return fmt.Errorf("no highlighted torrent")
	}
	if h.Paused {
		return e.Resume(h.T.InfoHash())
	}
	return e.Pause(h.T.InfoHash())
}

// Torrents returns every tracked torrent handle, in the order they were
// added.
func (e *Engine) Torrents() []*TorrentHandle {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*TorrentHandle, len(e.order))
	for i, hash := range e.order {
		out[i] = e.torrents[hash]
	}
	return out
}

func (e *Engine) Get(hash metainfo.Hash) (*TorrentHandle, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	h, ok := e.torrents[hash]
	return h, ok
}

// Highlighted returns the currently highlighted torrent, if any.
func (e *Engine) Highlighted() (*TorrentHandle, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.highlightedIdx < 0 || e.highlightedIdx >= len(e.order) {
		return nil, false
	}
	return e.torrents[e.order[e.highlightedIdx]], true
}

// HighlightNext/HighlightPrev cycle the highlighted torrent for the
// switcher-strip's Left/Right navigation.
func (e *Engine) HighlightNext() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.order) == 0 {
		return
	}
	e.highlightedIdx = (e.highlightedIdx + 1 + len(e.order)) % len(e.order)
}

func (e *Engine) HighlightPrev() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.order) == 0 {
		return
	}
	e.highlightedIdx = (e.highlightedIdx - 1 + len(e.order)) % len(e.order)
}

func (e *Engine) Close() {
	if e.Client != nil {
		e.Client.Close()
	}
}

// Halt stops all live network activity as soon as the VPN watchdog detects
// loss. It deliberately doesn't hold mu: closing the client waits on every
// torrent's teardown, and the UI's once-a-second Snapshot would otherwise
// block on mu, freezing the dashboard exactly when it should show the drop.
func (e *Engine) Halt() {
	e.Close()
}
