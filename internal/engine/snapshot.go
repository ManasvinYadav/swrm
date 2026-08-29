package engine

import (
	"time"

	"github.com/anacrolix/torrent"
)

// Snapshot is the stable, UI-facing view of the highlighted transfer. UI code
// can consume this value without retaining or mutating torrent-library
// internals.
type Snapshot struct {
	Active               bool
	Paused               bool
	Hash                 string
	Name                 string
	Length, Completed    int64
	PieceCount           int
	Pieces               []PieceSnapshot
	Peers                []PeerSnapshot
	Downloaded, Uploaded int64
	// DownloadRate/UploadRate are the sum of each connected peer's own
	// real-time rate (anacrolix/torrent's Peer.Stats()), the closest
	// approximation of "current transfer speed" its public API exposes.
	DownloadRate, UploadRate float64
	SampledAt                time.Time
	VPNActive                bool
}
type PieceSnapshot struct {
	Complete bool
	Priority torrent.PiecePriority
}
type PeerSnapshot struct {
	Address, Client          string
	DownloadRate, UploadRate float64
}

// Snapshot samples the currently highlighted torrent. Only the highlighted
// torrent pays the cost of walking every piece and peer connection — see
// Summaries for the cheap multi-torrent view.
func (e *Engine) Snapshot() Snapshot {
	s := Snapshot{SampledAt: time.Now(), VPNActive: e.VpnManager != nil && e.VpnManager.IsActive()}
	h, ok := e.Highlighted()
	if !ok {
		return s
	}
	s.Active = true
	s.Paused = h.Paused
	s.Hash = h.T.InfoHash().HexString()
	if h.T.Info() == nil {
		return s
	}
	t := h.T
	s.Name = t.Name()
	s.Length = t.Length()
	s.Completed = t.BytesCompleted()
	s.PieceCount = t.NumPieces()
	s.Pieces = make([]PieceSnapshot, s.PieceCount)
	for i := 0; i < s.PieceCount; i++ {
		p := t.Piece(i).State()
		s.Pieces[i] = PieceSnapshot{p.Complete, p.Priority}
	}
	stats := t.Stats()
	s.Downloaded = stats.BytesReadData.Int64()
	s.Uploaded = stats.BytesWrittenData.Int64()
	for _, p := range t.PeerConns() {
		client := ""
		if v := p.PeerClientName.Load(); v != nil {
			client, _ = v.(string)
		}
		ps := p.Stats()
		s.DownloadRate += ps.DownloadRate
		s.UploadRate += ps.LastWriteUploadRate
		s.Peers = append(s.Peers, PeerSnapshot{
			Address:      p.RemoteAddr.String(),
			Client:       client,
			DownloadRate: ps.DownloadRate,
			UploadRate:   ps.LastWriteUploadRate,
		})
	}
	return s
}

// TorrentSummary is the lightweight per-torrent view for the switcher strip
// — deliberately not a full Snapshot, which walks every piece and peer.
type TorrentSummary struct {
	Hash          string
	Name          string
	Progress      float64 // 0..100, 0 if metadata not yet resolved
	MetadataReady bool
	Paused        bool
	Highlighted   bool
}

// Summaries returns a lightweight view of every tracked torrent, in
// insertion order, for rendering the switcher strip.
func (e *Engine) Summaries() []TorrentSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]TorrentSummary, len(e.order))
	for i, hash := range e.order {
		h := e.torrents[hash]
		sum := TorrentSummary{Hash: hash.HexString(), Paused: h.Paused, Highlighted: i == e.highlightedIdx}
		if info := h.T.Info(); info != nil {
			sum.MetadataReady = true
			sum.Name = h.T.Name()
			if length := h.T.Length(); length > 0 {
				sum.Progress = float64(h.T.BytesCompleted()) / float64(length) * 100
			}
		}
		out[i] = sum
	}
	return out
}
