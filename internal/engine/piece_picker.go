package engine

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

// PriorityForPiece documents the playback scheduling policy independently of
// the torrent library, making boundary behaviour straightforward to test.
//
// Four zones, in descending urgency:
//   - Zone 0 (critical headers, first/last ~4MB): PiecePriorityNow — matches
//     the library's own "a reader is reading in this piece" semantics.
//   - Zone 1 (urgent buffer, current..current+8): PiecePriorityHigh.
//   - Zone 2 (sequential read-ahead, current+9..current+40):
//     PiecePriorityReadahead — the library's comparator breaks ties within
//     this tier by raw piece index, giving strict sequential ordering.
//   - Zone 3 (swarm health, everything beyond current+40): PiecePriorityNormal
//     — the library's native rarest-first request order applies here.
func PriorityForPiece(piece, current, total int, pieceLength int64) torrent.PiecePriority {
	if total <= 0 || piece < 0 || piece >= total {
		return torrent.PiecePriorityNone
	}
	header := int((4*1024*1024 + pieceLength - 1) / pieceLength)
	if piece < header || piece >= total-header {
		return torrent.PiecePriorityNow
	}
	if piece >= current && piece <= current+8 {
		return torrent.PiecePriorityHigh
	}
	if piece <= current+40 {
		return torrent.PiecePriorityReadahead
	}
	return torrent.PiecePriorityNormal
}

type PiecePicker struct {
	mu            sync.Mutex
	t             *torrent.Torrent
	currentOffset int64
	pieceLength   int64
	totalPieces   int
	urgentSince   map[int]time.Time
}

func NewPiecePicker(t *torrent.Torrent) *PiecePicker {
	info := t.Info()
	var pieceLen int64
	var totalPieces int
	if info != nil {
		pieceLen = info.PieceLength
		totalPieces = info.NumPieces()
	}

	return &PiecePicker{
		t:           t,
		pieceLength: pieceLen,
		totalPieces: totalPieces,
		urgentSince: make(map[int]time.Time),
	}
}

func (p *PiecePicker) UpdateOffset(offset int64) {
	p.mu.Lock()
	p.currentOffset = offset
	p.mu.Unlock()
	p.Recalculate()
}

func (p *PiecePicker) Recalculate() {
	if p.pieceLength == 0 || p.totalPieces == 0 {
		info := p.t.Info()
		if info == nil {
			return // Not resolved yet
		}
		p.pieceLength = info.PieceLength
		p.totalPieces = info.NumPieces()
	}

	p.mu.Lock()
	offset := p.currentOffset
	pCurrent := int(math.Floor(float64(offset) / float64(p.pieceLength)))
	now := time.Now()
	// Track when each piece first entered the urgent [current, current+8]
	// window, so the endgame monitor can tell how long it's been waiting.
	// Pieces that fall out of the window (e.g. a seek) drop their clock, so
	// re-entry starts fresh rather than looking falsely stale.
	for idx := range p.urgentSince {
		if idx < pCurrent || idx > pCurrent+8 {
			delete(p.urgentSince, idx)
		}
	}
	for i := pCurrent; i <= pCurrent+8 && i < p.totalPieces; i++ {
		if i < 0 {
			continue
		}
		if _, ok := p.urgentSince[i]; !ok {
			p.urgentSince[i] = now
		}
	}
	p.mu.Unlock()

	for i := 0; i < p.totalPieces; i++ {
		p.t.Piece(i).SetPriority(PriorityForPiece(i, pCurrent, p.totalPieces, p.pieceLength))
	}
}

// StalledPieces returns urgent-window pieces that have been waiting to
// complete for at least 400ms, per the streaming spec's latency deadline.
func (p *PiecePicker) StalledPieces() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	var out []int
	for idx, since := range p.urgentSince {
		if now.Sub(since) >= 400*time.Millisecond {
			out = append(out, idx)
		}
	}
	sort.Ints(out)
	return out
}

// StartEndgameMonitor watches the urgent playback buffer and escalates any
// piece that's been stalled past the 400ms deadline to PiecePriorityNow, so
// anacrolix/torrent requests it as aggressively as possible from every
// available peer. The spec describes duplicating requests to the top 3
// lowest-RTT peers specifically; anacrolix/torrent's public API doesn't
// expose per-peer request duplication, so priority escalation is the
// closest real equivalent it supports.
func (p *PiecePicker) StartEndgameMonitor(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.escalateStalledPieces()
		}
	}
}

func (p *PiecePicker) escalateStalledPieces() {
	for _, idx := range p.StalledPieces() {
		piece := p.t.Piece(idx)
		if !piece.State().Complete {
			piece.SetPriority(torrent.PiecePriorityNow)
		}
	}
}
