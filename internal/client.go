package internal

import (
	"encoding/gob"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"plotng/internal/widget"
)

type Client struct {
	app                *tview.Application
	activePlotsTable   *widget.SortedTable
	plotDirsTable      *widget.SortedTable
	targetTable        *tview.Table
	archivedPlotsTable *widget.SortedTable

	logTextbox          *tview.TextView
	hosts               []string
	msg                 map[string]*Msg
	archivedTableActive bool
	activeLogs          map[string][]string
	archivedLogs        map[string][]string
	logPlotId           string
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second, // This covers the entire request
}

func (client *Client) ProcessLoop(hostList string) {
	for _, host := range strings.Split(hostList, ",") {
		host = strings.TrimSpace(host)
		if strings.Index(host, ":") < 0 {
			host += ":8484"
		}
		client.hosts = append(client.hosts, host)
	}
	client.msg = map[string]*Msg{}

	gob.Register(Msg{})
	gob.Register(ActivePlot{})

	client.setupUI()

	go client.processLoop()
	client.app.Run()
}

func (client *Client) processLoop() {
	client.checkServers()
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		client.checkServers()
	}
}

func (client *Client) checkServers() {
	for _, host := range client.hosts {
		client.checkServer(host)
	}
}

func (client *Client) getServerData(host string) (*Msg, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/", host), nil)
	if err != nil {
		return nil, err
	}

	if resp, err := httpClient.Do(req); err == nil {
		defer resp.Body.Close()
		var msg Msg
		decoder := gob.NewDecoder(resp.Body)
		if err := decoder.Decode(&msg); err == nil {
			return &msg, nil
		} else {
			return nil, fmt.Errorf("Failed to decode message: %w", err)
		}
	} else {
		return nil, err
	}
}

func (client *Client) checkServer(host string) {
	// Retrieve data on the goroutine thread
	msg, err := client.getServerData(host)

	// Modify UI state on the tview thread.
	client.app.QueueUpdateDraw(func() {
		if err != nil {
			client.logTextbox.SetText("Log (error) ")
			client.logTextbox.SetText(err.Error())
			return
		}
		client.msg[host] = msg
		client.drawActivePlotsTable()
		client.drawPlotDirsTable()
		client.drawTargetTable()
		client.drawArchivedPlotsTable()

		log, ok := client.activeLogs[client.logPlotId]
		if !ok {
			log, ok = client.archivedLogs[client.logPlotId]
		}
		if ok {
			client.logTextbox.SetTitle(fmt.Sprintf(" Log (%s) ", shortenPlotId(client.logPlotId)))
			client.logTextbox.SetText(strings.Join(log, ""))
			client.logTextbox.ScrollToEnd()
		}
	})
}

func (client *Client) drawTargetTable() {
	client.targetTable.Clear()
	client.targetTable.SetCell(0, 0, tview.NewTableCell("Host").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.targetTable.SetCell(0, 1, tview.NewTableCell("Directory").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.targetTable.SetCell(0, 2, tview.NewTableCell("Available Space").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.targetTable.SetCell(0, 3, tview.NewTableCell("Avg Plot Time").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	row := 1
	for _, host := range client.hosts {
		msg, found := client.msg[host]
		if !found {
			continue
		}
		pathList := []string{}
		for k, _ := range msg.TargetDirs {
			pathList = append(pathList, k)
		}
		sort.Strings(pathList)
		for _, path := range pathList {
			client.targetTable.SetCell(row, 0, tview.NewTableCell(host))
			client.targetTable.SetCell(row, 1, tview.NewTableCell(path))
			availableSpace := msg.TargetDirs[path] / GB
			client.targetTable.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d GB", availableSpace)).SetAlign(tview.AlignRight))
			client.targetTable.SetCell(row, 3, tview.NewTableCell(client.computeAvgTargetTime(host, path)).SetAlign(tview.AlignRight))
			row++
		}
	}
}

func (client *Client) setupUI() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	client.activePlotsTable = widget.NewSortedTable()
	client.activePlotsTable.SetSelectable(true)
	client.activePlotsTable.SetBorder(true)
	client.activePlotsTable.SetTitleAlign(tview.AlignLeft)
	client.activePlotsTable.SetTitle(" Active Plots ")
	client.activePlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	client.activePlotsTable.SetSelectionChangedFunc(client.selectActivePlot)
	client.activePlotsTable.SetupFromType(activePlotsData{})

	client.plotDirsTable = widget.NewSortedTable()
	client.plotDirsTable.SetSelectable(true)
	client.plotDirsTable.SetBorder(true)
	client.plotDirsTable.SetTitleAlign(tview.AlignLeft)
	client.plotDirsTable.SetTitle(" Plot Directories ")
	client.plotDirsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	client.plotDirsTable.SetupFromType(plotDirData{})

	client.targetTable = tview.NewTable()
	client.targetTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Dest Directories")
	client.targetTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.archivedPlotsTable = widget.NewSortedTable()
	client.archivedPlotsTable.SetSelectable(true)
	client.archivedPlotsTable.SetBorder(true)
	client.archivedPlotsTable.SetTitleAlign(tview.AlignLeft)
	client.archivedPlotsTable.SetTitle(" Archived Plots ")
	client.archivedPlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	client.archivedPlotsTable.SetSelectionChangedFunc(client.selectArchivedPlot)
	client.archivedPlotsTable.SetupFromType(archivedPlotData{})

	client.logTextbox = tview.NewTextView()
	client.logTextbox.SetBorder(true).SetTitle(" Log ").SetTitleAlign(tview.AlignLeft)

	client.logTextbox.ScrollToEnd()

	client.app = tview.NewApplication()

	dirPanel := tview.NewFlex()
	dirPanel.SetDirection(tview.FlexColumn)
	dirPanel.AddItem(client.plotDirsTable, 0, 1, false)
	dirPanel.AddItem(client.targetTable, 0, 1, false)

	mainPanel := tview.NewFlex()
	mainPanel.SetDirection(tview.FlexRow)
	mainPanel.AddItem(client.activePlotsTable, 0, 1, true)
	mainPanel.AddItem(dirPanel, 0, 1, false)
	mainPanel.AddItem(client.archivedPlotsTable, 0, 1, false)
	mainPanel.AddItem(client.logTextbox, 0, 1, false)

	client.app = tview.NewApplication()
	client.app.SetRoot(mainPanel, true)
	client.app.EnableMouse(true)
}

func shortenPlotId(id string) string {
	if len(id) < 20 {
		return ""
	}
	return fmt.Sprintf("%s...%s", id[:10], id[len(id)-10:])
}

// Active plots

type activePlotsData struct {
	Host      string        `header:"Host"`
	PlotId    string        `header:"Plot ID"`
	Status    int           `header:"Status"`
	Phase     int           `header:"Phase"    data-align:"right"`
	Progress  int           `header:"Progress" data-align:"right"`
	StartTime time.Time     `header:"Start Time"`
	Duration  time.Duration `header:"Duration"`
	PlotDir   string        `header:"Plot Dir"`
	DestDir   string        `header:"Dest Dir"`
}

func (apd *activePlotsData) Strings() []string {
	status := "Unknown"
	switch apd.Status {
	case PlotRunning:
		status = "Running"
	case PlotError:
		status = "Errored"
	case PlotFinished:
		status = "Finished"
	}
	return []string{
		apd.Host,
		shortenPlotId(apd.PlotId),
		status,
		fmt.Sprintf("%d/4", apd.Phase),
		fmt.Sprintf("%d%%", apd.Progress),
		apd.StartTime.Format("2006-01-02 15:04:05"),
		DurationString(apd.Duration),
		apd.PlotDir,
		apd.DestDir,
	}
}

func (client *Client) makeActivePlotsData(host string, p *ActivePlot) *activePlotsData {
	apd := &activePlotsData{}
	apd.Host = host
	apd.PlotId = p.Id
	apd.Status = p.State
	apd.Phase = p.getCurrentPhase()
	apd.Progress = p.getProgress()
	apd.StartTime = p.getPhaseTime(0)
	apd.Duration = time.Since(apd.StartTime)
	apd.PlotDir = p.PlotDir
	apd.DestDir = p.TargetDir
	return apd
}

func (client *Client) drawActivePlotsTable() {
	activePlotsCount := 0
	client.activeLogs = make(map[string][]string)

	keysToRemove := make(map[string]struct{})
	for _, key := range client.activePlotsTable.Keys() {
		keysToRemove[key] = struct{}{}
	}

	for host, msg := range client.msg {
		for _, plot := range msg.Actives {
			delete(keysToRemove, plot.Id)
			client.activeLogs[plot.Id] = plot.Tail
			client.activePlotsTable.SetRowData(plot.Id, client.makeActivePlotsData(host, plot))
			activePlotsCount++
		}
	}

	for key, _ := range keysToRemove {
		client.activePlotsTable.ClearRowData(key)
	}

	client.activePlotsTable.SetTitle(fmt.Sprintf(" Active Plots [%d] ", activePlotsCount))
}

func (client *Client) selectActivePlot(key string) {
	client.logPlotId = key
	client.archivedPlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrDim))
	client.activePlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrBold))
	client.logTextbox.SetTitle(fmt.Sprintf(" Log (%s) ", shortenPlotId(client.logPlotId)))
	if log, found := client.activeLogs[key]; found {
		client.logTextbox.SetText(strings.Join(log, ""))
		client.logTextbox.ScrollToEnd()
	} else {
		client.logTextbox.SetText("")
	}
}

// Plot directories

type plotDirData struct {
	Host           string        `header:"Host"`
	PlotDir        string        `header:"Directory"`
	AvailableBytes uint64        `header:"Available Space" data-align:"right"`
	AvgPhase1      time.Duration `header:"Avg Phase 1" data-align:"right"`
	AvgPhase2      time.Duration `header:"Avg Phase 2" data-align:"right"`
	AvgPhase3      time.Duration `header:"Avg Phase 3" data-align:"right"`
	AvgPhase4      time.Duration `header:"Avg Phase 4" data-align:"right"`
	Count          int           `header:"Count" data-align:"right"`
	Failed         int           `header:"Failed" data-align:"right"`
}

func (pdd *plotDirData) Strings() []string {
	return []string{
		pdd.Host,
		pdd.PlotDir,
		fmt.Sprintf("%d GB", pdd.AvailableBytes/GB),
		DurationString(pdd.AvgPhase1),
		DurationString(pdd.AvgPhase2),
		DurationString(pdd.AvgPhase3),
		DurationString(pdd.AvgPhase4),
		fmt.Sprintf("%d", pdd.Count),
		fmt.Sprintf("%d", pdd.Failed),
	}
}

func (client *Client) makePlotDirsData() map[string]*plotDirData {
	plotDirs := make(map[string]*plotDirData)

	for host, msg := range client.msg {
		for plotDir, plotSpace := range msg.TempDirs {
			plotDirs[host+"||"+plotDir] = &plotDirData{
				Host:           host,
				PlotDir:        plotDir,
				AvailableBytes: plotSpace,
			}
		}

		for _, plot := range msg.Archived {
			pdd, ok := plotDirs[host+"||"+plot.PlotDir]
			if !ok {
				// There's data from a completed plot, but we're no longer using it
				pdd = &plotDirData{
					Host:           host,
					PlotDir:        plot.PlotDir,
					AvailableBytes: math.MaxUint64,
				}
				plotDirs[host+"||"+plot.PlotDir] = pdd
			}
			switch plot.State {
			case PlotFinished:
				pdd.AvgPhase1 += plot.getPhaseTime(1).Sub(plot.getPhaseTime(0))
				pdd.AvgPhase2 += plot.getPhaseTime(2).Sub(plot.getPhaseTime(1))
				pdd.AvgPhase3 += plot.getPhaseTime(3).Sub(plot.getPhaseTime(2))
				pdd.AvgPhase4 += plot.getPhaseTime(4).Sub(plot.getPhaseTime(3))
				pdd.Count++
			case PlotError, PlotKilled:
				pdd.Failed++
			}
		}
	}

	for _, pdd := range plotDirs {
		if pdd.Count > 0 {
			pdd.AvgPhase1 /= time.Duration(pdd.Count)
			pdd.AvgPhase2 /= time.Duration(pdd.Count)
			pdd.AvgPhase3 /= time.Duration(pdd.Count)
			pdd.AvgPhase4 /= time.Duration(pdd.Count)
		}
	}
	return plotDirs
}

func (client *Client) drawPlotDirsTable() {
	plotDirs := client.makePlotDirsData()

	keysToRemove := make(map[string]struct{})
	for _, key := range client.plotDirsTable.Keys() {
		keysToRemove[key] = struct{}{}
	}

	for key, pdd := range plotDirs {
		delete(keysToRemove, key)
		client.plotDirsTable.SetRowData(key, pdd)
	}

	for key, _ := range keysToRemove {
		client.plotDirsTable.ClearRowData(key)
	}

	client.plotDirsTable.SetTitle(fmt.Sprintf(" Plot Directories [%d] ", len(plotDirs)))
}

// Archived plots

type archivedPlotData struct {
	Host      string        `header:"Host"`
	PlotId    string        `header:"Plot Id"`
	Status    int           `header:"Status"`
	Phase     int           `header:"Phase" data-align:"right"`
	StartTime time.Time     `header:"Start Time"`
	EndTime   time.Time     `header:"End Time"`
	Duration  time.Duration `header:"Duration"`
	PlotDir   string        `header:"Plot Dir"`
	DestDir   string        `header:"Dest Dir"`
}

func (apd *archivedPlotData) Strings() []string {
	status := "Unknown"
	switch apd.Status {
	case PlotRunning:
		status = "Running"
	case PlotError:
		status = "Errored"
	case PlotFinished:
		status = "Finished"
	}
	return []string{
		apd.Host,
		shortenPlotId(apd.PlotId),
		status,
		fmt.Sprintf("%d/4", apd.Phase),
		apd.StartTime.Format("2006-01-02 15:04:05"),
		apd.EndTime.Format("2006-01-02 15:04:05"),
		DurationString(apd.Duration),
		apd.PlotDir,
		apd.DestDir,
	}
}

func (client *Client) makeArchivedPlotData(host string, p *ActivePlot) *archivedPlotData {
	apd := &archivedPlotData{}
	apd.Host = host
	apd.PlotId = p.Id
	apd.Status = p.State
	apd.Phase = p.getCurrentPhase()
	apd.StartTime = p.getPhaseTime(0)
	apd.EndTime = p.getPhaseTime(4)
	apd.Duration = apd.EndTime.Sub(apd.StartTime)
	apd.PlotDir = p.PlotDir
	apd.DestDir = p.TargetDir
	return apd
}

func (client *Client) drawArchivedPlotsTable() {
	archivedPlotsSuccess := 0
	archivedPlotsFailed := 0
	client.archivedLogs = make(map[string][]string)

	keysToRemove := make(map[string]struct{})
	for _, key := range client.archivedPlotsTable.Keys() {
		keysToRemove[key] = struct{}{}
	}

	for host, msg := range client.msg {
		for _, plot := range msg.Archived {
			delete(keysToRemove, plot.Id)
			client.archivedLogs[plot.Id] = plot.Tail
			client.archivedPlotsTable.SetRowData(plot.Id, client.makeArchivedPlotData(host, plot))
			switch plot.State {
			case PlotFinished:
				archivedPlotsSuccess++
			case PlotKilled:
				archivedPlotsFailed++
			case PlotError:
				archivedPlotsFailed++
			}
		}
	}

	for key, _ := range keysToRemove {
		client.archivedPlotsTable.ClearRowData(key)
	}

	if archivedPlotsFailed > 0 {
		client.archivedPlotsTable.SetTitle(fmt.Sprintf(" Archived Plots [%d (%d failed)] ", archivedPlotsSuccess, archivedPlotsFailed))
	} else {
		client.archivedPlotsTable.SetTitle(fmt.Sprintf(" Archived Plots [%d] ", archivedPlotsSuccess))
	}
}

func (client *Client) selectArchivedPlot(key string) {
	client.logPlotId = key
	client.activePlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrDim))
	client.archivedPlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrBold))
	client.logTextbox.SetTitle(fmt.Sprintf(" Log (%s) ", shortenPlotId(client.logPlotId)))
	if log, found := client.archivedLogs[key]; found {
		client.logTextbox.SetText(strings.Join(log, ""))
		client.logTextbox.ScrollToEnd()
	} else {
		client.logTextbox.SetText("")
	}
}

////////////////////

func (client *Client) computeAvgTargetTime(host, path string) string {
	msg, found := client.msg[host]
	if !found {
		return ""
	}
	var count int64
	var total int64
	for _, plot := range msg.Archived {
		if plot.TargetDir != path || plot.State != PlotFinished {
			continue
		}
		count++
		total = total + int64(plot.EndTime.Sub(plot.StartTime))
	}
	if count == 0 {
		return ""
	} else {
		return DurationString(time.Duration(total / count))
	}
}

func (client *Client) AvgPhase1(host, path string) string {
	msg, found := client.msg[host]
	if !found {
		return ""
	}
	var count int64
	var total int64
	for _, plot := range msg.Archived {
		if plot.PlotDir != path || plot.Phase1Time.IsZero() || plot.State != PlotFinished {
			continue
		}
		count++
		total = total + int64(plot.Phase1Time.Sub(plot.StartTime))
	}
	if count == 0 {
		return ""
	} else {
		return DurationString(time.Duration(total / count))
	}
}

func (client *Client) AvgPhase2(host, path string) string {
	msg, found := client.msg[host]
	if !found {
		return ""
	}
	var count int64
	var total int64
	for _, plot := range msg.Archived {
		if plot.PlotDir != path || plot.Phase1Time.IsZero() || plot.Phase2Time.IsZero() || plot.State != PlotFinished {
			continue
		}
		count++
		total = total + int64(plot.Phase2Time.Sub(plot.Phase1Time))
	}
	if count == 0 {
		return ""
	} else {
		return DurationString(time.Duration(total / count))
	}
}

func (client *Client) AvgPhase3(host, path string) string {
	msg, found := client.msg[host]
	if !found {
		return ""
	}
	var count int64
	var total int64
	for _, plot := range msg.Archived {
		if plot.PlotDir != path || plot.Phase2Time.IsZero() || plot.Phase3Time.IsZero() || plot.State != PlotFinished {
			continue
		}
		count++
		total = total + int64(plot.Phase3Time.Sub(plot.Phase2Time))
	}
	if count == 0 {
		return ""
	} else {
		return DurationString(time.Duration(total / count))
	}
}

func (client *Client) AvgPhase4(host, path string) string {
	msg, found := client.msg[host]
	if !found {
		return ""
	}
	var count int64
	var total int64
	for _, plot := range msg.Archived {
		if plot.PlotDir != path || plot.Phase3Time.IsZero() || plot.State != PlotFinished {
			continue
		}
		count++
		total = total + int64(plot.EndTime.Sub(plot.Phase3Time))
	}
	if count == 0 {
		return ""
	} else {
		return DurationString(time.Duration(total / count))
	}
}
