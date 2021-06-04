package internal

import (
	"strings"
	"testing"
	"time"
)

const (
	msgStagger = "staggering start until"
)

func checkFailure(t *testing.T, svr *Server, now time.Time, expectedErr string) {
	t.Helper()
	checkResults(t, svr, now, expectedErr, "", "")
}

func checkSuccess(t *testing.T, svr *Server, now time.Time, expectedTargetDir, expectedPlotDir string) {
	t.Helper()
	checkResults(t, svr, now, "", expectedTargetDir, expectedPlotDir)
}

func checkResults(t *testing.T, svr *Server, now time.Time, expectedErrString string, expectedTargetDir, expectedPlotDir string) {
	t.Helper()
	actualTargetDir, actualPlotDir, actualErr := svr.canCreateNewPlot(svr.config.CurrentConfig, now)
	failed := false
	if actualErr != nil {
		if expectedErrString == "" || !strings.Contains(actualErr.Error(), expectedErrString) {
			t.Error("err mismatch")
			failed = true
		}
	} else if expectedErrString != "" {
		t.Error("err mismatch")
		failed = true
	}
	if expectedTargetDir != actualTargetDir {
		t.Error("target mismatch")
		failed = true
	}
	if expectedPlotDir != actualPlotDir {
		t.Error("plot mismatch")
		failed = true
	}
	if failed {
		t.Logf("expected={%s,'%s','%s'}, actual={%v,'%s','%s'}", expectedErrString, expectedTargetDir, expectedPlotDir, actualErr, actualTargetDir, actualPlotDir)
		t.FailNow()
	}
}

var initialTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func TestCanCreateNewPlotBasic(t *testing.T) {
	svr := &Server{
		config: &PlotConfig{
			CurrentConfig: &Config{},
		},
	}

	now := initialTime

	checkFailure(t, svr, now, "configuration lacks")
	svr.config.CurrentConfig.TargetDirectory = []string{"target"}
	checkFailure(t, svr, now, "configuration lacks")
	svr.config.CurrentConfig.TargetDirectory = nil
	svr.config.CurrentConfig.TempDirectory = []string{"plot"}
	checkFailure(t, svr, now, "configuration lacks")
}

func TestCanCreateNewPlotObeysStartTime(t *testing.T) {
	svr := &Server{
		config: &PlotConfig{
			CurrentConfig: &Config{
				TargetDirectory: []string{"target1", "target2"},
				TempDirectory:   []string{"plot"},
			},
		},
		targetDelayStartTime: initialTime,
	}

	now := initialTime
	checkSuccess(t, svr, now, "target1", "plot")

	svr.targetDelayStartTime = initialTime.Add(time.Minute)
	checkFailure(t, svr, now, "waiting until")
}

func TestCanCreateNewPlotCyclesTargets(t *testing.T) {
	svr := &Server{
		config: &PlotConfig{
			CurrentConfig: &Config{
				TargetDirectory: []string{"target1", "target2"},
				TempDirectory:   []string{"plot"},
			},
		},
	}

	now := initialTime
	checkSuccess(t, svr, now, "target1", "plot")
	checkSuccess(t, svr, now, "target2", "plot")

	// After cycling targets, it's always a reject
	checkFailure(t, svr, now, msgStagger)

	checkSuccess(t, svr, now, "target1", "plot")
	checkSuccess(t, svr, now, "target2", "plot")
}

func TestCanCreateNewPlotHandlesTempChange(t *testing.T) {
	svr := &Server{
		config: &PlotConfig{
			ConfigPath: "",
			CurrentConfig: &Config{
				TargetDirectory: []string{"target"},
				TempDirectory:   []string{"plot1", "plot2", "plot3", "plot4"},
			},
		},
	}

	now := initialTime
	checkSuccess(t, svr, now, "target", "plot1")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject

	checkSuccess(t, svr, now, "target", "plot2")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject

	checkSuccess(t, svr, now, "target", "plot3")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject

	// We've cycled through 3 plot directories, now we "reload" the config with less than 3 items
	svr.config.CurrentConfig.TempDirectory = []string{"plot1", "plot2"}

	checkSuccess(t, svr, now, "target", "plot1")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	checkSuccess(t, svr, now, "target", "plot2")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
}

func TestCanCreateNewPlotLimitsPhase1(t *testing.T) {
	const msgTooManyPlot1 = "too many active plots in phase 1"

	svr := &Server{
		config: &PlotConfig{
			CurrentConfig: &Config{
				TargetDirectory:        []string{"target"},
				TempDirectory:          []string{"plot"},
				MaxActivePlotPerPhase1: 2,
			},
		},
		active: make(map[int64]*ActivePlot),
	}

	now := initialTime
	checkSuccess(t, svr, now, "target", "plot")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	svr.active[1] = &ActivePlot{State: PlotRunning, Phase: "1/4"}

	checkSuccess(t, svr, now, "target", "plot")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	svr.active[2] = &ActivePlot{State: PlotRunning, Phase: "1/4"}
	checkFailure(t, svr, now, msgTooManyPlot1) // We're busy

	svr.active[1].Phase = "2/4"
	checkSuccess(t, svr, now, "target", "plot")
	svr.active[3] = &ActivePlot{State: PlotRunning, Phase: "1/4"}
	checkFailure(t, svr, now, msgStagger)      // After cycling targets, it's always a reject
	checkFailure(t, svr, now, msgTooManyPlot1) // We're busy

}

func TestCanCreateNewPlotCyclesPlots(t *testing.T) {
	svr := &Server{
		config: &PlotConfig{
			CurrentConfig: &Config{
				TargetDirectory: []string{"target"},
				TempDirectory:   []string{"plot1", "plot2"},
			},
		},
	}

	now := initialTime
	checkSuccess(t, svr, now, "target", "plot1")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	checkSuccess(t, svr, now, "target", "plot2")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	checkSuccess(t, svr, now, "target", "plot1")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	checkSuccess(t, svr, now, "target", "plot2")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
}

func TestCanCreateNewPlotLimitsPlots(t *testing.T) {
	const msgTooManyTemp = "too many temp plots"

	svr := &Server{
		config: &PlotConfig{
			CurrentConfig: &Config{
				TargetDirectory:      []string{"target"},
				TempDirectory:        []string{"plot1", "plot2"},
				MaxActivePlotPerTemp: 2,
			},
		},
		active: make(map[int64]*ActivePlot),
	}

	now := initialTime
	checkSuccess(t, svr, now, "target", "plot1")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	svr.active[1] = &ActivePlot{State: PlotRunning, Phase: "1/4", PlotDir: "plot1"}

	checkSuccess(t, svr, now, "target", "plot2")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	svr.active[2] = &ActivePlot{State: PlotRunning, Phase: "1/4", PlotDir: "plot2"}

	checkSuccess(t, svr, now, "target", "plot1")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	svr.active[3] = &ActivePlot{State: PlotRunning, Phase: "1/4", PlotDir: "plot1"}

	checkSuccess(t, svr, now, "target", "plot2")
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject
	svr.active[4] = &ActivePlot{State: PlotRunning, Phase: "1/4", PlotDir: "plot2"}

	checkFailure(t, svr, now, msgTooManyTemp) // We're busy
	checkFailure(t, svr, now, msgTooManyTemp) // We're busy
}

func TestCanCreateNewPlotLimitsTargets(t *testing.T) {
	const msgTooManyActive = "too many active plots"

	svr := &Server{
		config: &PlotConfig{
			CurrentConfig: &Config{
				TargetDirectory:        []string{"target1", "target2"},
				TempDirectory:          []string{"plot"},
				MaxActivePlotPerTarget: 2,
			},
		},
		active: make(map[int64]*ActivePlot),
	}

	now := initialTime
	checkSuccess(t, svr, now, "target1", "plot")
	svr.active[1] = &ActivePlot{State: PlotRunning, Phase: "1/4", TargetDir: "target1"}
	checkSuccess(t, svr, now, "target2", "plot")
	svr.active[2] = &ActivePlot{State: PlotRunning, Phase: "1/4", TargetDir: "target2"}
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject

	checkSuccess(t, svr, now, "target1", "plot")
	svr.active[3] = &ActivePlot{State: PlotRunning, Phase: "1/4", TargetDir: "target1"}
	checkSuccess(t, svr, now, "target2", "plot")
	svr.active[4] = &ActivePlot{State: PlotRunning, Phase: "1/4", TargetDir: "target2"}
	checkFailure(t, svr, now, msgStagger) // After cycling targets, it's always a reject

	checkFailure(t, svr, now, msgTooManyActive) // We're busy
}
