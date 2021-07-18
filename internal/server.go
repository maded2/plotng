package internal

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ricochet2200/go-disk-usage/du"
)

type Server struct {
	config               *PlotConfig
	active               map[int64]*ActivePlot
	archive              []*ActivePlot
	currentTemp          int
	currentTarget        int
	targetDelayStartTime time.Time
	lastStatus           string
	lock                 sync.RWMutex
}

func (server *Server) ProcessLoop(configPath string, host string, port int) {
	gob.Register(Msg{})
	gob.Register(ActivePlot{})
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), server); err != nil {
			log.Fatalf("Failed to start webserver: %s", err)
		}
	}()

	server.config = &PlotConfig{
		ConfigPath: configPath,
	}
	server.active = map[int64]*ActivePlot{}
	server.createPlot(time.Now())
	ticker := time.NewTicker(time.Minute)
	for t := range ticker.C {
		server.createPlot(t)
	}
}

func (server *Server) createPlot(t time.Time) {
	if server.config.ProcessConfig() {
		server.targetDelayStartTime = time.Time{} // reset delay if new config was loaded
	}
	if server.config.CurrentConfig != nil {
		server.config.Lock.RLock()
		server.lock.Lock()
		if targetDir, plotDir, err := server.canCreateNewPlot(server.config.CurrentConfig, time.Now()); err == nil {
			server.createNewPlot(server.config.CurrentConfig, targetDir, plotDir)
			server.lastStatus = "Creating plot"
		} else {
			log.Printf("Skipping new plot: %v", err)
			server.lastStatus = err.Error()
		}

		server.lock.Unlock()
		server.config.Lock.RUnlock()
	}
	fmt.Printf("%s, %d Active Plots\n", t.Format("2006-01-02 15:04:05"), len(server.active))
	for _, plot := range server.active {
		fmt.Print(plot.String(server.config.CurrentConfig.ShowPlotLog))
		if plot.State == PlotFinished || plot.State == PlotError {
			server.archive = append(server.archive, plot)
			delete(server.active, plot.PlotId)
		}
	}
	fmt.Println(" ")
}

func (server *Server) canCreateNewPlot(config *Config, now time.Time) (string, string, error) {
	if len(config.TempDirectory) == 0 || len(config.TargetDirectory) == 0 {
		return "", "", errors.New("configuration lacks TempDirectory or TargetDirectory")
	}
	if now.Before(server.targetDelayStartTime) {
		return "", "", fmt.Errorf("waiting until %s", server.targetDelayStartTime.Format("2006-01-02 15:04:05"))
	}
	if server.countActivePlots() >= server.config.CurrentConfig.NumberOfParallelPlots {
		return "", "", fmt.Errorf("running %d/%d plots", server.countActivePlots(), server.config.CurrentConfig.NumberOfParallelPlots)
	}

	if server.currentTarget >= len(config.TargetDirectory) {
		server.currentTarget = 0
		server.targetDelayStartTime = now.Add(time.Duration(config.StaggeringDelay) * time.Minute)
		return "", "", fmt.Errorf("staggering start until %s", server.targetDelayStartTime.Format("2006-01-02 15:04:05"))
	}
	if server.currentTemp >= len(config.TempDirectory) {
		server.currentTemp = 0
	}
	if config.MaxActivePlotPerPhase1 > 0 {
		var sum int
		for _, plot := range server.active {
			if plot.State == PlotRunning && plot.getCurrentPhase() <= 1 {
				sum++
			}
		}

		if config.MaxActivePlotPerPhase1 <= sum {
			return "", "", fmt.Errorf("too many active plots in phase 1: %d", sum)
		}
	}
	plotDir := config.TempDirectory[server.currentTemp]
	server.currentTemp++
	if server.currentTemp >= len(config.TempDirectory) {
		server.currentTemp = 0
	}
	if config.MaxActivePlotPerTemp > 0 && server.countActiveTemp(plotDir) >= config.MaxActivePlotPerTemp {
		return "", "", fmt.Errorf("skipping [%s], too many temp plots: %d", plotDir, server.countActiveTemp(plotDir))
	}
	targetDir := config.TargetDirectory[server.currentTarget]
	server.currentTarget++

	activeTargets := server.countActiveTarget(targetDir)
	if config.MaxActivePlotPerTarget > 0 && activeTargets >= config.MaxActivePlotPerTarget {
		return "", "", fmt.Errorf("skipping [%s], too many active plots: %d", targetDir, activeTargets)
	}

	server.targetDelayStartTime = now.Add(time.Duration(config.DelaysBetweenPlot) * time.Minute)

	targetDirSpace := server.getDiskSpaceAvailable(targetDir)
	if config.DiskSpaceCheck && uint64(activeTargets+1)*PLOT_SIZE > targetDirSpace {
		return "", "", fmt.Errorf("skipping [%s], not enough space: %s", targetDir, SpaceString(targetDirSpace))
	}

	return targetDir, plotDir, nil
}

func (server *Server) createNewPlot(config *Config, targetDir string, plotDir string) {
	t := time.Now()
	plot := &ActivePlot{
		PlotId:           t.Unix(),
		TargetDir:        targetDir,
		PlotDir:          plotDir,
		Fingerprint:      config.Fingerprint,
		FarmerPublicKey:  config.FarmerPublicKey,
		PoolPublicKey:    config.PoolPublicKey,
		ContractAddress:  config.ContractAddress,
		Threads:          config.Threads,
		Buffers:          config.Buffers,
		PlotSize:         config.PlotSize,
		DisableBitField:  config.DisableBitField,
		UseTargetForTmp2: config.UseTargetForTmp2,
		BucketSize:       config.BucketSize,
		SavePlotLogDir:   config.SavePlotLogDir,
		Tmp2Dir:          config.Tmp2,
		Phase:            "NA",
		Tail:             nil,
		State:            PlotRunning,
	}
	server.active[plot.PlotId] = plot
	go plot.RunPlot(config)
}

func (server *Server) countActiveTarget(path string) (count int) {
	for _, plot := range server.active {
		if plot.TargetDir == path {
			count++
		}
	}
	return
}

func (server *Server) countActivePlots() (count int) {
	if server.config.CurrentConfig.AsyncCopying {
		for _, plot := range server.active {
			if plot.getCurrentPhase() < 4 {
				count++
			}
		}
	}  else {
		count = len(server.active)
	}

	return
}

func (server *Server) countActiveTemp(path string) (count int) {
	for _, plot := range server.active {
		if plot.PlotDir == path {
			count++
		}
	}
	return
}

func (server *Server) getDiskSpaceAvailable(path string) uint64 {
	d := du.NewDiskUsage(path)
	return d.Available()
}

func (server *Server) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	log.Printf("New query: %s -  %s", req.Method, req.URL.String())
	defer server.lock.RUnlock()
	server.lock.RLock()

	switch req.Method {
	case "GET":
		var msg Msg
		msg.TargetDirs = map[string]uint64{}
		msg.TempDirs = map[string]uint64{}
		for _, v := range server.active {
			msg.Actives = append(msg.Actives, v)
		}
		for _, v := range server.archive {
			msg.Archived = append(msg.Archived, v)
		}
		if server.config.CurrentConfig != nil {
			for _, dir := range server.config.CurrentConfig.TargetDirectory {
				msg.TargetDirs[dir] = server.getDiskSpaceAvailable(dir)
			}
			for _, dir := range server.config.CurrentConfig.TempDirectory {
				msg.TempDirs[dir] = server.getDiskSpaceAvailable(dir)
			}
		}
		msg.Status = server.lastStatus
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(msg); err == nil {
			resp.WriteHeader(http.StatusOK)
			resp.Write(buf.Bytes())
		} else {
			resp.WriteHeader(http.StatusInternalServerError)
			log.Printf("Failed to encode message: %s", err)
		}
	case "DELETE":
		for _, v := range server.active {
			if v.Id == req.RequestURI {
				v.process.Kill()
				v.State = PlotKilled
			}
		}
	}
}

type Msg struct {
	Actives    []*ActivePlot
	Archived   []*ActivePlot
	TempDirs   map[string]uint64
	TargetDirs map[string]uint64
	Status     string
}
