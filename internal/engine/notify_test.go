package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// TestPostDownloadCmdRunsOnceSelectionCompletes covers post_download_cmd,
// which used to be parsed from the config and documented but never run.
// The torrent's data is already on disk, so it completes as soon as its
// pieces are checked, but only once a file selection has been applied.
func TestPostDownloadCmdRunsOnceSelectionCompletes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the command below is POSIX shell")
	}
	dataDir := t.TempDir()
	payload := filepath.Join(dataDir, "payload.bin")
	if err := os.WriteFile(payload, bytes.Repeat([]byte("swrm"), 16<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	info := metainfo.Info{PieceLength: 16 << 10}
	if err := info.BuildFromFilePath(payload); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	torrentPath := filepath.Join(t.TempDir(), "payload.torrent")
	f, err := os.Create(torrentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&metainfo.MetaInfo{InfoBytes: infoBytes}).Write(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	marker := filepath.Join(t.TempDir(), "done")
	vm, err := NewVpnManager("", nil)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := NewEngine(vm, dataDir, Options{DownloadDir: dataDir, PostDownloadCmd: "touch '" + marker + "'"})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	tr, err := eng.AddTorrentFile(torrentPath)
	if err != nil {
		t.Fatal(err)
	}
	<-tr.GotInfo()

	// Nothing is selected until the file-selection modal is confirmed, and
	// "all zero selected files are done" mustn't count as complete.
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("post_download_cmd ran before any file was selected")
	}

	for _, file := range tr.Files() {
		file.SetPriority(torrent.PiecePriorityNormal)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("post_download_cmd never ran after the selected files completed")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
