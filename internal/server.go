package internal

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/ricochet2200/go-disk-usage/du"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Server struct {
	config               *PlotConfig
	active               map[int64]*ActivePlot
	archive              []*ActivePlot
	currentTemp          int
	currentTarget        int
	targetDelayStartTime time.Time
	lock                 sync.RWMutex
}

func (server *Server) ProcessLoop(configPath string, port int) {
	gob.Register(Msg{})
	gob.Register(ActivePlot{})
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), server); err != nil {
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
		if len(server.active) < server.config.CurrentConfig.NumberOfParallelPlots {
			server.createNewPlot(server.config.CurrentConfig)
		}
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

func (server *Server) createNewPlot(config *Config) {
	defer server.lock.Unlock()
	server.lock.Lock()
	if len(config.TempDirectory) == 0 || len(config.TargetDirectory) == 0 {
		return
	}
	if time.Now().Before(server.targetDelayStartTime) {
		log.Printf("Waiting until %s", server.targetDelayStartTime.Format("2006-01-02 15:04:05"))
		return
	}

	if server.currentTarget >= len(config.TargetDirectory) {
		server.currentTarget = 0
		server.targetDelayStartTime = time.Now().Add(time.Duration(config.StaggeringDelay) * time.Minute)
		return
	}
	if server.currentTemp >= len(config.TempDirectory) {
		server.currentTemp = 0
	}
	if config.MaxActivePlotPerPhase1 > 0 {
		getPhase1 := func(plot *ActivePlot) bool {
			if strings.HasPrefix(plot.Phase, "1/4") {
				return true
			}
			return false
		}

		var sum int

		for _, plot := range server.active {
			if getPhase1(plot) {
				sum++
			}
		}

		if config.MaxActivePlotPerPhase1 <= sum {
			return
		}
	}
	plotDir := config.TempDirectory[server.currentTemp]
	server.currentTemp++
	if server.currentTemp >= len(config.TempDirectory) {
		server.currentTemp = 0
	}
	if config.MaxActivePlotPerTemp > 0 && int(server.countActiveTemp(plotDir)) >= config.MaxActivePlotPerTemp {
		log.Printf("Skipping [%s], too many active plots: %d", plotDir, int(server.countActiveTemp(plotDir)))
		return
	}
	targetDir := config.TargetDirectory[server.currentTarget]
	server.currentTarget++

	if config.MaxActivePlotPerTarget > 0 && int(server.countActiveTarget(targetDir)) >= config.MaxActivePlotPerTarget {
		log.Printf("Skipping [%s], too many active plots: %d", targetDir, int(server.countActiveTarget(targetDir)))
		return
	}

	server.targetDelayStartTime = time.Now().Add(time.Duration(config.DelaysBetweenPlot) * time.Minute)

	targetDirSpace := server.getDiskSpaceAvailable(targetDir)
	if config.DiskSpaceCheck && (server.countActiveTarget(targetDir)+1)*PLOT_SIZE > targetDirSpace {
		log.Printf("Skipping [%s], Not enough space: %d", targetDir, targetDirSpace/GB)
		return
	}

	t := time.Now()
	plot := &ActivePlot{
		PlotId:           t.Unix(),
		TargetDir:        targetDir,
		PlotDir:          plotDir,
		Fingerprint:      config.Fingerprint,
		FarmerPublicKey:  config.FarmerPublicKey,
		PoolPublicKey:    config.PoolPublicKey,
		Threads:          config.Threads,
		Buffers:          config.Buffers,
		DisableBitField:  config.DisableBitField,
		UseTargetForTmp2: config.UseTargetForTmp2,
		BucketSize:       config.BucketSize,
		SavePlotLogDir:   config.SavePlotLogDir,
		Phase:            "NA",
		Tail:             nil,
		State:            PlotRunning,
	}
	server.active[plot.PlotId] = plot
	go plot.RunPlot()
}

func (server *Server) countActiveTarget(path string) (count uint64) {
	for _, plot := range server.active {
		if plot.TargetDir == path {
			count++
		}
	}
	return
}

func (server *Server) countActiveTemp(path string) (count uint64) {
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
}
