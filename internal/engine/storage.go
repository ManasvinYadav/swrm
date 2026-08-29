package engine

import (
	"github.com/anacrolix/torrent/storage"
)

// NewStorage builds the file-backed piece storage anacrolix/torrent uses to
// hash, sparse-write, and track completion state for downloaded pieces. This
// is exactly what the library builds internally when only DataDir is set;
// constructing it explicitly makes storage configuration first-class instead
// of an implicit library default.
func NewStorage(dir string) storage.ClientImpl {
	if dir == "" {
		dir = "./downloads"
	}
	return storage.NewFile(dir)
}
