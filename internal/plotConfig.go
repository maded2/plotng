package internal

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type config struct {
	TargetDirectory        []string
	TempDirectory          []string
	NumberOfParallelPlots  int
	Fingerprint            string
	FarmerPublicKey        string
	PoolPublicKey          string
	Threads                int
	PlotSize               int
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

type plotConfig struct {
	ConfigPath    string
	CurrentConfig *config
	LastMod       time.Time
	Lock          sync.RWMutex
}

func (pc *PlotConfig) ProcessConfig() (newConfigLoaded bool) {
	fs, err := os.Lstat(pc.ConfigPath)
	if err != nil {
		log.Printf("Failed to open config file [%s]: %s\n", pc.ConfigPath, err)
		return
	}
	if pc.LastMod == fs.ModTime() {
		return
	}
	f, err := os.Open(pc.ConfigPath)
	if err != nil {
		log.Printf("Failed to open config file [%s]: %s\n", pc.ConfigPath, err)
		return
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	var newConfig config
	if err := decoder.Decode(&newConfig); err != nil {
		log.Printf("Failed to process config file [%s], check your config file for mistake: %s\n", pc.ConfigPath, err)
		return
	}
	pc.Lock.Lock()
	pc.CurrentConfig = &newConfig
	pc.Lock.Unlock()
	log.Printf("New configuration loaded")
	pc.LastMod = fs.ModTime()
	return true
}
