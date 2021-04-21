package main

import (
	"flag"
	"fmt"
	"github.com/ricochet2200/go-disk-usage/du"
	"plotng/plotng"
	"time"
)

var (
	config               *plotng.PlotConfig
	active               map[int64]*plotng.ActivePlot
	archive              []*plotng.ActivePlot
	currentTemp          int
	currentTarget        int
	targetDelayStartTime time.Time
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
	for t := range ticker.C {
		if config.CurrentConfig != nil {
			config.Lock.RLock()
			if len(active) < config.CurrentConfig.NumberOfParallelPlots {
				createNewPlot(config.CurrentConfig)
			}
			config.Lock.RUnlock()
		}
		fmt.Printf("%s\n", t.Format("2006-01-02 15:04:05"))
		for _, plot := range active {
			fmt.Print(plot.String())
			if plot.State == plotng.PlotFinished || plot.State == plotng.PlotError {
				archive = append(archive, plot)
				delete(active, plot.PlotId)
			}
		}
		fmt.Println(" ")
	}
}

func createNewPlot(config *plotng.Config) {
	if len(config.TempDirectory) == 0 || len(config.TargetDirectory) == 0 {
		return
	}
	if time.Now().Before(targetDelayStartTime) {
		return
	}
	if currentTemp >= len(config.TempDirectory) {
		currentTemp = 0
	}
	plotDir := config.TempDirectory[currentTemp]
	currentTemp++

	if currentTarget >= len(config.TargetDirectory) {
		currentTarget = 0
		targetDelayStartTime = time.Now().Add(time.Duration(config.StaggeringDelay) * time.Minute)
		return
	}
	targetDir := config.TargetDirectory[currentTarget]
	currentTarget++
	t := time.Now()
	plot := &plotng.ActivePlot{
		PlotId:      t.Unix(),
		TargetDir:   targetDir,
		PlotDir:     plotDir,
		Fingerprint: config.Fingerprint,
		Phase:       "NA",
		Tail:        nil,
		State:       plotng.PlotRunning,
	}
	active[plot.PlotId] = plot
	go plot.RunPlot()
}

func getDiskSpaceAvailable(path string) uint64 {
	d := du.NewDiskUsage(path)
	return d.Available()
}
