package main

import (
	"bufio"
	"github.com/ricochet2200/go-disk-usage/du"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	PlotRunning = iota
	PlotError
	PlotFinished
)

type ActivePlot struct {
	startTime   time.Time
	endTime     time.Time
	targetDir   string
	plotDir     string
	fingerprint string

	phase string
	tail  []string
	state int
	lock  sync.RWMutex
	id    string
}

func (ap *ActivePlot) CheckSpace() bool {
	plot := du.NewDiskUsage(ap.plotDir)
	target := du.NewDiskUsage(ap.targetDir)
	if plot.Available() < 360*KB*KB*KB {
		log.Printf("Not enough Plot directory space [%s]: %dGB", ap.plotDir, plot.Available()/(KB*KB*KB))
		return false
	}
	if target.Available() < 360*KB*KB*KB {
		log.Printf("Not enough Target directory space [%s]: %dGB", ap.targetDir, target.Available()/(KB*KB*KB))
		return false
	}
	return true
}

func (ap *ActivePlot) RunPlot() {
	ap.startTime = time.Now()
	defer func() {
		ap.endTime = time.Now()
	}()
	args := []string{
		"plots", "create", "-k32", "-n1", "-b6000", "-u128",
		"-t" + ap.targetDir,
		"-d" + ap.targetDir,
		"-a" + ap.fingerprint,
	}
	cmd := exec.Command("chia", args...)
	ap.state = PlotRunning
	stderr, err := cmd.StderrPipe()
	if err != nil {
		ap.state = PlotError
		log.Printf("Failed to start Plotting: %s", err)
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ap.state = PlotError
		log.Printf("Failed to start Plotting: %s", err)
		return
	}
	go ap.processLogs(stdout)
	go ap.processLogs(stderr)
	if err := cmd.Run(); err != nil {
		ap.state = PlotError
		log.Printf("Plotting Exit with Error: %s", err)
		return
	}
}

func (ap *ActivePlot) processLogs(in io.ReadCloser) {
	reader := bufio.NewReader(in)
	for {
		if s, err := reader.ReadString('\n'); err != nil {
			break
		} else {
			if strings.HasPrefix(s, "Starting phase ") {
				ap.phase = s[15:18]
			}
			if strings.HasPrefix(s, "ID: ") {
				ap.id = s[4:]
			}
			ap.lock.Lock()
			ap.tail = append(ap.tail, s)
			if len(ap.tail) > 10 {
				ap.tail = ap.tail[len(ap.tail)-10:]
			}
			ap.lock.Unlock()
		}
	}
	return
}
