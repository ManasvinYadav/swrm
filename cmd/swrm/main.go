package main

import (
	"context"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"swrm/internal/config"
	"swrm/internal/engine"
	"swrm/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	stateChan := make(chan engine.VpnState, 10)
	vpnMgr, err := engine.NewVpnManager(cfg.Interface, stateChan)
	if err != nil {
		log.Fatalf("Failed to initialize VPN manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go vpnMgr.StartHeartbeat(ctx)

	eng, err := engine.NewEngine(vpnMgr, cfg.DownloadDir, engine.Options{DownloadDir: cfg.DownloadDir, ListenPort: cfg.ListenPort, DHT: cfg.DHT, DownloadLimit: cfg.DownloadLimit, UploadLimit: cfg.UploadLimit})
	if err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}
	defer eng.Close()
	go func() {
		for state := range stateChan {
			if state == engine.StateHaltedLeakPrevention {
				eng.Halt()
			}
		}
	}()

	rootModel := ui.NewRootModel(eng)
	p := tea.NewProgram(rootModel, tea.WithAltScreen())

	_, err = p.Run()
	if err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
