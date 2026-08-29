package engine

import (
	"github.com/anacrolix/torrent"
)

// DHT setup happens automatically in anacrolix/torrent NewDefaultClientConfig.
// This is to explicitly define DHT configuration if needed.
func ConfigureDHT(cfg *torrent.ClientConfig, enable bool) {
	if !enable {
		cfg.NoDHT = true
	}
}
