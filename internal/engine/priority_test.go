package engine

import (
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// TestFilePriorityIsolatesDeselectedFiles is a regression test for a bug
// where deselecting a file in the file-selection modal had no effect on
// whether it actually downloaded. The old streaming PiecePicker (removed
// along with streaming) called Piece.SetPriority directly for every piece
// the moment a torrent's metadata resolved, before the user ever chose which
// files to keep. anacrolix/torrent's Piece.purePriority() takes the MAX of a
// piece's own explicit priority and every overlapping file's priority, so
// once that piece-level floor was set, no later File.SetPriority(PiecePriorityNone)
// call could ever lower it back down.
//
// This drives a real *torrent.Torrent the same way the file-selection modal
// does today (File.SetPriority only, no direct Piece.SetPriority) and
// asserts a deselected file's exclusive pieces actually end up not-wanted.
func TestFilePriorityIsolatesDeselectedFiles(t *testing.T) {
	const pieceLength = 16 << 10 // 16 KiB
	const piecesPerFile = 4
	fileLength := int64(pieceLength * piecesPerFile)

	info := metainfo.Info{
		PieceLength: pieceLength,
		Name:        "test",
		Files: []metainfo.FileInfo{
			{Length: fileLength, Path: []string{"keep.bin"}},
			{Length: fileLength, Path: []string{"skip.bin"}},
		},
		// Piece length/file boundaries are what this test exercises, not
		// actual data — the hashes never get checked because nothing here
		// reads or verifies piece data.
		Pieces: make([]byte, 2*piecesPerFile*20),
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatalf("marshal info: %v", err)
	}

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = t.TempDir()
	cfg.NoDHT = true
	cfg.DisableTrackers = true
	cfg.DisablePEX = true
	cfg.DisableTCP = true
	cfg.DisableUTP = true
	cfg.NoDefaultPortForwarding = true
	cl, err := torrent.NewClient(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer cl.Close()

	tr, err := cl.AddTorrent(&metainfo.MetaInfo{InfoBytes: infoBytes})
	if err != nil {
		t.Fatalf("add torrent: %v", err)
	}

	files := tr.Files()
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	keep, skip := files[0], files[1]

	// Exactly what RootModel.Update's FileTree-apply step does today.
	keep.SetPriority(torrent.PiecePriorityHigh)
	skip.SetPriority(torrent.PiecePriorityNone)

	// The client resolves each piece's local-storage completion status
	// asynchronously after AddTorrent returns; effectivePriority() reports
	// PiecePriorityNone for any piece until that resolves (see
	// Piece.ignoreForRequests), which is a startup-timing detail unrelated
	// to file-priority selection. Wait for it to settle so the assertions
	// below are actually exercising priority behavior, not this race.
	deadline := time.Now().Add(3 * time.Second)
	for tr.Piece(keep.BeginPieceIndex()).State().Priority == torrent.PiecePriorityNone {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for piece completion status to resolve")
		}
		time.Sleep(5 * time.Millisecond)
	}

	for i := skip.BeginPieceIndex(); i < skip.EndPieceIndex(); i++ {
		if got := tr.Piece(i).State().Priority; got != torrent.PiecePriorityNone {
			t.Errorf("piece %d belongs only to the deselected file, want PiecePriorityNone, got %v", i, got)
		}
	}
	for i := keep.BeginPieceIndex(); i < keep.EndPieceIndex(); i++ {
		if got := tr.Piece(i).State().Priority; got != torrent.PiecePriorityHigh {
			t.Errorf("piece %d belongs only to the kept file, want PiecePriorityHigh, got %v", i, got)
		}
	}
}
