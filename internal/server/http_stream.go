package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/anacrolix/torrent"
)

type OffsetUpdater interface{ UpdateOffset(int64) }

type StreamServer struct {
	T        *torrent.Torrent
	FileIdx  int
	picker   OffsetUpdater
	listener net.Listener
	server   *http.Server
	mu       sync.RWMutex
}

func NewStreamServer(t *torrent.Torrent, fileIdx int, picker ...OffsetUpdater) *StreamServer {
	s := &StreamServer{T: t, FileIdx: fileIdx}
	if len(picker) > 0 {
		s.picker = picker[0]
	}
	return s
}
func (s *StreamServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return fmt.Errorf("stream server already started")
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	s.listener = l
	s.server = &http.Server{Handler: http.HandlerFunc(s.handleStream)}
	srv := s.server
	go func() { _ = srv.Serve(l) }()
	return nil
}
func (s *StreamServer) Close(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
func (s *StreamServer) URL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil || s.T == nil {
		return ""
	}
	return fmt.Sprintf("http://%s/stream/%s/%d", s.listener.Addr(), s.T.InfoHash().HexString(), s.FileIdx)
}

func parseRange(value string, size int64) (int64, int64, error) {
	if size <= 0 {
		return 0, 0, fmt.Errorf("empty resource")
	}
	if value == "" {
		return 0, size - 1, nil
	}
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return 0, 0, fmt.Errorf("invalid range")
	}
	p := strings.Split(strings.TrimPrefix(value, "bytes="), "-")
	if len(p) != 2 {
		return 0, 0, fmt.Errorf("invalid range")
	}
	if p[0] == "" {
		n, e := strconv.ParseInt(p[1], 10, 64)
		if e != nil || n <= 0 {
			return 0, 0, fmt.Errorf("invalid range")
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, nil
	}
	start, e := strconv.ParseInt(p[0], 10, 64)
	if e != nil || start < 0 || start >= size {
		return 0, 0, fmt.Errorf("invalid range")
	}
	end := size - 1
	if p[1] != "" {
		end, e = strconv.ParseInt(p[1], 10, 64)
		if e != nil || end < start {
			return 0, 0, fmt.Errorf("invalid range")
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, nil
}
func (s *StreamServer) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", 405)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "stream" || s.T == nil || parts[1] != s.T.InfoHash().HexString() {
		http.NotFound(w, r)
		return
	}
	idx, e := strconv.Atoi(parts[2])
	if e != nil || idx != s.FileIdx {
		http.NotFound(w, r)
		return
	}
	select {
	case <-s.T.GotInfo():
	case <-r.Context().Done():
		return
	}
	files := s.T.Files()
	if idx < 0 || idx >= len(files) {
		http.NotFound(w, r)
		return
	}
	file := files[idx]
	size := file.Length()
	start, end, e := parseRange(r.Header.Get("Range"), size)
	if e != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		http.Error(w, "range not satisfiable", 416)
		return
	}
	if s.picker != nil {
		s.picker.UpdateOffset(file.Offset() + start)
	}
	file.Download()
	reader := file.NewReader()
	defer reader.Close()
	reader.SetContext(r.Context())
	reader.SetReadahead(40 << 20)
	if _, e = reader.Seek(start, io.SeekStart); e != nil {
		http.Error(w, "seek failed", 500)
		return
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("Content-Type", "application/octet-stream")
	if r.Header.Get("Range") != "" {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.WriteHeader(206)
	}
	if r.Method != "HEAD" {
		_, _ = io.CopyN(w, reader, end-start+1)
	}
}
