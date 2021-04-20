package main

import (
	"flag"
	"github.com/ricochet2200/go-disk-usage/du"
	"plotng/plotng"
	"time"
)

var (
	config        *plotng.PlotConfig
	active        map[int64]*plotng.ActivePlot
	currentTemp   int
	currentTarget int
)

func main() {
	configFile := flag.String("config", "", "configuration file")
	flag.Parse()
	if flag.Parsed() == false || len(*configFile) == 0 {
		flag.Usage()
		return
	}
	active = map[int64]*plotng.ActivePlot{}

	config = &plotng.PlotConfig{
		ConfigPath: *configFile,
	}
	config.Init()

	createPlot()
}

func createPlot() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		if config.CurrentConfig != nil {
			config.Lock.RLock()
			if len(active) < config.CurrentConfig.NumberOfPlots {
				createNewPlot(config.CurrentConfig)
			}
			config.Lock.RUnlock()
		}
	}
}

func createNewPlot(config *plotng.Config) {
	if len(config.TempDirectory) == 0 || len(config.TargetDirectory) == 0 {
		return
	}
	if currentTemp >= len(config.TempDirectory) {
		currentTemp = 0
	}
	plotDir := config.TempDirectory[currentTemp]
	currentTemp++

	if currentTarget >= len(config.TargetDirectory) {
		currentTarget = 0
	}
	targetDir := config.TargetDirectory[currentTarget]
	currentTarget++
	t := time.Now()
	plot := &plotng.ActivePlot{
		PlotId:      t.Unix(),
		StartTime:   t,
		TargetDir:   targetDir,
		PlotDir:     plotDir,
		Fingerprint: config.Fingerprint,
		Phase:       "",
		Tail:        nil,
		State:       plotng.PlotRunning,
	}
	active[plot.PlotId] = plot
}

func getDiskSpaceAvailable(path string) uint64 {
	d := du.NewDiskUsage(path)
	return d.Available()
}
