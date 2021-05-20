package internal

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const KB = uint64(1024)
const MB = KB * KB
const GB = KB * KB * KB
const PLOT_SIZE = 105 * GB

const (
	PlotRunning = iota
	PlotError
	PlotFinished
	PlotKilled
)

type ActivePlot struct {
	PlotId          int64
	StartTime       time.Time
	EndTime         time.Time
	TargetDir       string
	PlotDir         string
	Fingerprint     string
	FarmerPublicKey string
	PoolPublicKey   string
	Threads         int
	Buffers         int
	DisableBitField bool

	Phase            string
	Tail             []string
	State            int
	lock             sync.RWMutex
	Id               string
	Progress         string
	Phase1Time       time.Time
	Phase2Time       time.Time
	Phase3Time       time.Time
	Pid              int
	UseTargetForTmp2 bool
	BucketSize       int
	SavePlotLogs     bool
	process          *os.Process
}

func (ap *ActivePlot) Duration(currentTime time.Time) string {
	return DurationString(currentTime.Sub(ap.StartTime))
}

func (ap *ActivePlot) String(showLog bool) string {
	ap.lock.RLock()
	state := "Unknown"
	switch ap.State {
	case PlotRunning:
		state = "Running"
	case PlotError:
		state = "Errored"
	case PlotFinished:
		state = "Finished"
	}
	s := fmt.Sprintf("Plot [%s] - %s, Phase: %s %s, Start Time: %s, Duration: %s, Tmp Dir: %s, Dst Dir: %s\n", ap.Id, state, ap.Phase, ap.Progress, ap.StartTime.Format("2006-01-02 15:04:05"), ap.Duration(time.Now()), ap.PlotDir, ap.TargetDir)
	if showLog {
		for _, l := range ap.Tail {
			s += fmt.Sprintf("\t%s", l)
		}
	}
	ap.lock.RUnlock()
	return s
}

func (ap *ActivePlot) RunPlot() {
	ap.StartTime = time.Now()
	defer func() {
		ap.EndTime = time.Now()
	}()
	args := []string{
		"plots", "create", "-k32", "-n1",
		"-t" + ap.PlotDir,
		"-d" + ap.TargetDir,
	}
	if len(ap.Fingerprint) > 0 {
		args = append(args, "-a"+ap.Fingerprint)
	}
	if len(ap.FarmerPublicKey) > 0 {
		args = append(args, "-f"+ap.FarmerPublicKey)
	}
	if len(ap.PoolPublicKey) > 0 {
		args = append(args, "-p"+ap.PoolPublicKey)
	}
	if ap.Threads > 0 {
		args = append(args, fmt.Sprintf("-r%d", ap.Threads))
	}
	if ap.Buffers > 0 {
		args = append(args, fmt.Sprintf("-b%d", ap.Buffers))
	}
	if ap.DisableBitField {
		args = append(args, "-e")
	}
	if ap.UseTargetForTmp2 {
		args = append(args, "-2"+ap.TargetDir)
	}
	if ap.BucketSize > 0 {
		args = append(args, fmt.Sprintf("-u%d", ap.BucketSize))
	}

	cmd := exec.Command("chia", args...)
	ap.State = PlotRunning
	if stderr, err := cmd.StderrPipe(); err != nil {
		ap.State = PlotError
		log.Printf("Failed to start Plotting: %s", err)
		return
	} else {
		go ap.processLogs(stderr)
	}
	if stdout, err := cmd.StdoutPipe(); err != nil {
		ap.State = PlotError
		log.Printf("Failed to start Plotting: %s", err)
		return
	} else {
		go ap.processLogs(stdout)
	}
	//log.Println(cmd.String())

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start chia command: %s", err)
		ap.State = PlotError
		return
	} else {
		ap.process = cmd.Process
		ap.Pid = cmd.Process.Pid
		if err := cmd.Wait(); err != nil {
			if ap.State != PlotKilled {
				ap.State = PlotError
				log.Printf("Plotting Exit with Error: %s", err)
			} else {
				log.Printf("Plot [%s] Killed", ap.Id)
			}
			ap.cleanup()
			return
		}
	}
	ap.State = PlotFinished
	return
}

func (ap *ActivePlot) processLogs(in io.ReadCloser) {
	reader := bufio.NewReader(in)
	var logFile *os.File
	for {
		if s, err := reader.ReadString('\n'); err != nil {
			break
		} else {
			if strings.HasPrefix(s, "Starting phase ") {
				ap.Phase = s[15:18]
				switch ap.Phase {
				case "2/4":
					ap.Phase1Time = time.Now()
				case "3/4":
					ap.Phase2Time = time.Now()
				case "4/4":
					ap.Phase3Time = time.Now()
				}
			}
			if strings.HasPrefix(s, "ID: ") {
				ap.Id = strings.TrimSuffix(s[4:], "\n")
				if ap.SavePlotLogs {
					userHomeDir, err := os.UserHomeDir()
					if err != nil {
						break
					}
					logFilePath := filepath.Join(userHomeDir, ".chia", "mainnet", "plotter", fmt.Sprintf("plotng_log_%s.txt", ap.Id))
					logFile, err = os.Create(logFilePath)
					if err != nil {
						break
					}
					for _, l := range ap.Tail {
						logFile.Write([]byte(l))
					}
				}
			}
			for phaseStr, progress := range progressTable {
				if strings.Index(s, phaseStr) >= 0 {
					ap.Progress = progress
					break
				}
			}
			ap.lock.Lock()
			if logFile != nil {
				logFile.Write([]byte(s))
			}
			ap.Tail = append(ap.Tail, s)
			if len(ap.Tail) > 20 {
				ap.Tail = ap.Tail[len(ap.Tail)-20:]
			}
			ap.lock.Unlock()
		}
	}
	return
}

func (ap *ActivePlot) cleanup() {
	if len(ap.Id) == 0 {
		return
	}
	if fileList, err := ioutil.ReadDir(ap.PlotDir); err == nil {
		for _, file := range fileList {
			if strings.Index(file.Name(), ap.Id) >= 0 && strings.HasSuffix(file.Name(), ".tmp") {
				fullPath := fmt.Sprintf("%s%c%s", ap.PlotDir, os.PathSeparator, file.Name())

				if err := os.Remove(fullPath); err == nil {
					log.Printf("File: %s deleted\n", fullPath)
				} else {
					log.Printf("Failed to delete file: %s\n", fullPath)
				}
			}
		}
	}
}

/*
Progress from Chia docs
https://github.com/Chia-Network/chia-blockchain/wiki/Beginners-Guide#create-a-plot
*/
var progressTable = map[string]string{
	"Computing table 1":          "1%",
	"Computing table 2":          "6%",
	"Computing table 3":          "12%",
	"Computing table 4":          "20%",
	"Computing table 5":          "28%",
	"Computing table 6":          "36%",
	"Computing table 7":          "42%",
	"Backpropagating on table 7": "43%",
	"Backpropagating on table 6": "48%",
	"Backpropagating on table 5": "51%",
	"Backpropagating on table 4": "55%",
	"Backpropagating on table 3": "58%",
	"Backpropagating on table 2": "61%",
	"Compressing tables 1 and 2": "66%",
	"Compressing tables 2 and 3": "73%",
	"Compressing tables 3 and 4": "79%",
	"Compressing tables 4 and 5": "85%",
	"Compressing tables 5 and 6": "92%",
	"Compressing tables 6 and 7": "98%",
	"Write checkpoint tables":    "100%",
}
