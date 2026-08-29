package engine

import (
	"testing"
	"time"

	"github.com/anacrolix/torrent"
)

func TestPriorityBoundaries(t *testing.T) {
	if got := PriorityForPiece(0, 10, 100, 1<<20); got != torrent.PiecePriorityNow {
		t.Fatal(got)
	}
	if got := PriorityForPiece(10, 10, 100, 1<<20); got != torrent.PiecePriorityHigh {
		t.Fatal(got)
	}
	// current+8 is still the urgent buffer; current+9 crosses into Zone 2.
	if got := PriorityForPiece(18, 10, 100, 1<<20); got != torrent.PiecePriorityHigh {
		t.Fatal(got)
	}
	if got := PriorityForPiece(19, 10, 100, 1<<20); got != torrent.PiecePriorityReadahead {
		t.Fatal(got)
	}
	// current+40 is still sequential read-ahead; current+41 falls to swarm health.
	if got := PriorityForPiece(50, 10, 100, 1<<20); got != torrent.PiecePriorityReadahead {
		t.Fatal(got)
	}
	if got := PriorityForPiece(51, 10, 100, 1<<20); got != torrent.PiecePriorityNormal {
		t.Fatal(got)
	}
	if got := PriorityForPiece(99, 10, 100, 1<<20); got != torrent.PiecePriorityNow {
		t.Fatal(got)
	}
}

func TestStalledPieces(t *testing.T) {
	p := &PiecePicker{urgentSince: map[int]time.Time{
		1: time.Now().Add(-500 * time.Millisecond), // well past the 400ms deadline
		2: time.Now(),                              // just entered the urgent window
		5: time.Now().Add(-401 * time.Millisecond), // just over the deadline
	}}
	got := p.StalledPieces()
	want := []int{1, 5}
	if len(got) != len(want) {
		t.Fatalf("StalledPieces() = %v, want %v", got, want)
	}
	for i, idx := range want {
		if got[i] != idx {
			t.Fatalf("StalledPieces() = %v, want %v", got, want)
		}
	}
}
