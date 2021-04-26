package internal

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type Config struct {
	TargetDirectory       []string
	TempDirectory         []string
	NumberOfParallelPlots int
	Fingerprint           string
	FarmerPublicKey       string
	PoolPublicKey         string
	Threads               int
	Buffers               int
	StaggeringDelay       int
	ShowPlotLog           bool
}

type PlotConfig struct {
	ConfigPath    string
	CurrentConfig *Config
	LastMod       time.Time
	Lock          sync.RWMutex
}

func (pc *PlotConfig) ProcessConfig() {
	if fs, err := os.Lstat(pc.ConfigPath); err != nil {
		log.Printf("Failed to open config file [%s]: %s\n", pc.ConfigPath, err)
	} else {
		if pc.LastMod != fs.ModTime() {
			if f, err := os.Open(pc.ConfigPath); err != nil {
				log.Printf("Failed to open config file [%s]: %s\n", pc.ConfigPath, err)
			} else {
				decoder := json.NewDecoder(f)
				var newConfig Config
				if err := decoder.Decode(&newConfig); err != nil {
					log.Printf("Failed to process config file [%s]: %s\n", pc.ConfigPath, err)
				} else {
					pc.Lock.Lock()
					pc.CurrentConfig = &newConfig
					pc.Lock.Unlock()
					log.Printf("New configuration loaded")
				}
				f.Close()
			}
			pc.LastMod = fs.ModTime()
		}
	}
}
