package internal

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type Config struct {
	TargetDirectory        []string
	TempDirectory          []string
	NumberOfParallelPlots  int
	Fingerprint            string
	FarmerPublicKey        string
	PoolPublicKey          string
	Threads                int
	Buffers                int
	DisableBitField        bool
	StaggeringDelay        int
	ShowPlotLog            bool
	DiskSpaceCheck         bool
	DelaysBetweenPlot      int
	MaxActivePlotPerTarget int
	MaxActivePlotPerTemp   int
	MaxActivePlotPerPhase1 int
	UseTargetForTmp2       bool
	BucketSize             int
	SavePlotLogDir         string
}

type PlotConfig struct {
	ConfigPath    string
	CurrentConfig *Config
	LastMod       time.Time
	Lock          sync.RWMutex
}

func (pc *PlotConfig) ProcessConfig() (newConfigLoaded bool) {
	if fs, err := os.Lstat(pc.ConfigPath); err != nil {
		log.Printf("Failed to open config file [%s]: %s\n", pc.ConfigPath, err)
	} else {
		if pc.LastMod != fs.ModTime() {
			if f, err := os.Open(pc.ConfigPath); err != nil {
				log.Printf("Failed to open config file [%s]: %s\n", pc.ConfigPath, err)
			} else {
				defer f.Close()
				decoder := json.NewDecoder(f)
				var newConfig Config
				if err := decoder.Decode(&newConfig); err != nil {
					log.Printf("Failed to process config file [%s], check your config file for mistake: %s\n", pc.ConfigPath, err)
				} else {
					pc.Lock.Lock()
					pc.CurrentConfig = &newConfig
					pc.Lock.Unlock()
					log.Printf("New configuration loaded")
					newConfigLoaded = true
				}
			}
			pc.LastMod = fs.ModTime()
		}
	}
	return
}
