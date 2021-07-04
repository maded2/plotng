package internal

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const KB = uint64(1024)
const MB = KB * KB
const GB = KB * KB * KB
const TB = KB * KB * KB * KB
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
	ContractAddress string
	Threads         int
	PlotSize        int
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
	SavePlotLogDir   string
	process          *os.Process
	useMadmaxPlotter bool
}

// getPhaseTime returns the end time of a phase. phase 0 is the start time
// of the entire plot, and phase 4 is the end time of the entire plot.
// TODO: We can change the ActivePlot structure to be PhaseTime [5]time.Time,
//       but that's a protocol change.
func (ap *ActivePlot) getPhaseTime(phase int) time.Time {
	switch phase {
	case 0:
		return ap.StartTime
	case 1:
		return ap.Phase1Time
	case 2:
		return ap.Phase2Time
	case 3:
		return ap.Phase3Time
	case 4:
		return ap.EndTime
	default:
		panic("request for invalid phase time")
	}
}

// getCurrentPhase returns the current phase, or a negative number to indicate an error.
// TODO: We can also change this to be part of the structure, but that's also a protocol change.
func (ap *ActivePlot) getCurrentPhase() int {
	parts := strings.Split(ap.Phase, "/")
	if len(parts) != 2 {
		return -1
	} else if i, err := strconv.Atoi(parts[0]); err != nil {
		return -2
	} else {
		return i
	}
}

// getProgress returns the current progress, or a negative number to indicate an error
// TODO: We can also change this to be part of the structure, but that's also a protocol change.
func (ap *ActivePlot) getProgress() int {
	if len(ap.Progress) == 0 {
		return -1
	} else if i, err := strconv.Atoi(ap.Progress[:len(ap.Progress)-1]); err != nil {
		return -2
	} else {
		return i
	}
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

func (ap *ActivePlot) RunPlot(config *Config) {
	ap.StartTime = time.Now()
	defer func() {
		ap.EndTime = time.Now()
	}()
	cmdStr, args := ap.createCmd(config)

	cmd := exec.Command(cmdStr, args...)
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

func (ap *ActivePlot) createCmd(config *Config) (cmd string, args []string) {
	if len(config.MadMaxPlotter) > 0 {
		ap.useMadmaxPlotter = true
		cmd = config.MadMaxPlotter
		plotDir := ap.PlotDir
		targetDir := ap.TargetDir
		if strings.HasSuffix(plotDir, "/") == false {
			plotDir += "/"
		}
		if strings.HasSuffix(targetDir, "/") == false {
			targetDir += "/"
		}
		args = []string{
			"-t", plotDir,
			"-2", plotDir,
			"-d", targetDir,
			"-f", ap.FarmerPublicKey,
			"-p", ap.PoolPublicKey,
		}

		if ap.Threads > 0 {
			args = append(args, "-r", fmt.Sprintf("%d", ap.Threads))
		}
		if ap.BucketSize > 0 {
			args = append(args, "-u", fmt.Sprintf("%d", ap.BucketSize))
		}
	} else {
		cmd = path.Join(config.ChiaRoot, "chia")
		args = []string{
			"plots", "create",
			"-n1",
			"-t", ap.PlotDir,
			"-d", ap.TargetDir,
		}
		if len(ap.Fingerprint) > 0 {
			args = append(args, "-a", ap.Fingerprint)
		}
		if len(ap.FarmerPublicKey) > 0 {
			args = append(args, "-f", ap.FarmerPublicKey)
		}
		if len(ap.PoolPublicKey) > 0 {
			args = append(args, "-p", ap.PoolPublicKey)
		}
		if len(ap.ContractAddress) > 0 {
			args = append(args, "-c", ap.PoolPublicKey)
		}
		if ap.Threads > 0 {
			args = append(args, "-r", fmt.Sprintf("%d", ap.Threads))
		}
		if ap.PlotSize > 0 {
			args = append(args, "-k", fmt.Sprintf("%d", ap.PlotSize))
			if ap.PlotSize < 32 {
				args = append(args, "--override-k")
			}
		} else {
			args = append(args, "-k32")
		}

		if ap.Buffers > 0 {
			args = append(args, "-b", fmt.Sprintf("%d", ap.Buffers))
		} else {
			switch ap.PlotSize {
			case 32:
				args = append(args, "-b", fmt.Sprintf("%d", 3390))
				break
			case 33:
				args = append(args, "-b", fmt.Sprintf("%d", 7400))
				break
			case 34:
				args = append(args, "-b", fmt.Sprintf("%d", 14800))
				break
			case 35:
				args = append(args, "-b", fmt.Sprintf("%d", 29600))
				break
			default:
				break

			}
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
	}
	return
}

func (ap *ActivePlot) processLogs(in io.ReadCloser) {
	reader := bufio.NewReader(in)
	var logFile *os.File
	for {
		if s, err := reader.ReadString('\n'); err != nil {
			break
		} else {
			if ap.useMadmaxPlotter {
				if strings.HasPrefix(s, "Plot Name:") {
					ap.Phase = "1/4"
					ap.Progress = "1%"
					ap.Id = strings.TrimSuffix(s[37:], "\n")
					if len(ap.SavePlotLogDir) > 0 {
						logFilePath := filepath.Join(ap.SavePlotLogDir, fmt.Sprintf("plotng_log_%s.txt", ap.Id))
						logFile, err = os.Create(logFilePath)
						if err != nil {
							fmt.Sprintf("Failed to create log file [%s]: %s", logFilePath, err)
						} else {
							for _, l := range ap.Tail {
								logFile.Write([]byte(l))
							}
						}
					}
				}
				if strings.HasPrefix(s, "Phase 1 took") {
					ap.Phase = "2/4"
					ap.Progress = "25%"
					ap.Phase1Time = time.Now()
				}
				if strings.HasPrefix(s, "Phase 2 took") {
					ap.Phase = "3/4"
					ap.Progress = "50%"
					ap.Phase2Time = time.Now()
				}
				if strings.HasPrefix(s, "Phase 3 took") {
					ap.Phase = "4/4"
					ap.Progress = "75%"
					ap.Phase3Time = time.Now()
				}
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
					if len(ap.SavePlotLogDir) > 0 {
						logFilePath := filepath.Join(ap.SavePlotLogDir, fmt.Sprintf("plotng_log_%s.txt", ap.Id))
						logFile, err = os.Create(logFilePath)
						if err != nil {
							fmt.Sprintf("Failed to create log file [%s]: %s", logFilePath, err)
						} else {
							for _, l := range ap.Tail {
								logFile.Write([]byte(l))
							}
						}
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
	if logFile != nil {
		logFile.Close()
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
