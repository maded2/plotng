package internal

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"plotng/internal/widget"
)

type Client struct {
	app                 *tview.Application
	activePlotsTable    *widget.SortedTable
	targetTable         *tview.Table
	tmpTable            *tview.Table
	lastTable           *tview.Table
	logTextbox          *tview.TextView
	hosts               []string
	msg                 map[string]*Msg
	archivedTableActive bool
	activeLogs          map[string][]string
	archivedLogs        map[int][]string
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
	client.activeLogs = make(map[string][]string)
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
			msg := &msg
			sort.Slice(msg.Archived, func(i, j int) bool {
				return msg.Archived[i].PlotId > msg.Archived[j].PlotId
			})
			return msg, nil
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
			client.logTextbox.SetText(err.Error())
			return
		}
		client.msg[host] = msg
		client.drawTempTable()
		client.drawTargetTable()
		client.drawActivePlots()
		client.drawArchivedPlots()
		if log, ok := client.activeLogs[client.logPlotId]; ok {
			client.logTextbox.SetText(strings.Join(log, ""))
			client.logTextbox.ScrollToEnd()
		}
	})
}

func (client *Client) drawTempTable() {
	client.tmpTable.Clear()
	client.tmpTable.SetCell(0, 0, tview.NewTableCell("Host").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 1, tview.NewTableCell("Directory").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 2, tview.NewTableCell("Available Space").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 3, tview.NewTableCell("Avg Phase1").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 4, tview.NewTableCell("Avg Phase2").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 5, tview.NewTableCell("Avg Phase3").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 6, tview.NewTableCell("Avg Phase4").SetSelectable(false).SetTextColor(tcell.ColorYellow))

	row := 1
	for _, host := range client.hosts {
		msg, found := client.msg[host]
		if !found {
			continue
		}
		pathList := []string{}
		for k, _ := range msg.TempDirs {
			pathList = append(pathList, k)
		}
		sort.Strings(pathList)
		for _, path := range pathList {
			client.tmpTable.SetCell(row, 0, tview.NewTableCell(host))
			client.tmpTable.SetCell(row, 1, tview.NewTableCell(path))
			availableSpace := msg.TempDirs[path] / GB
			client.tmpTable.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d GB", availableSpace)).SetAlign(tview.AlignRight))
			client.tmpTable.SetCell(row, 3, tview.NewTableCell(client.AvgPhase1(host, path)).SetAlign(tview.AlignRight))
			client.tmpTable.SetCell(row, 4, tview.NewTableCell(client.AvgPhase2(host, path)).SetAlign(tview.AlignRight))
			client.tmpTable.SetCell(row, 5, tview.NewTableCell(client.AvgPhase3(host, path)).SetAlign(tview.AlignRight))
			client.tmpTable.SetCell(row, 6, tview.NewTableCell(client.AvgPhase4(host, path)).SetAlign(tview.AlignRight))
			row++
		}
	}
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
	client.activePlotsTable.SetHeaders(activePlotsHeaders...)
	client.activePlotsTable.SetColumnAlign(3, tview.AlignRight)
	client.activePlotsTable.SetColumnAlign(4, tview.AlignRight)

	client.tmpTable = tview.NewTable()
	client.tmpTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Plot Directories")
	client.tmpTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.targetTable = tview.NewTable()
	client.targetTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Dest Directories")
	client.targetTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.lastTable = tview.NewTable()
	client.lastTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Archived Plots")
	client.lastTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	client.lastTable.SetSelectionChangedFunc(client.selectArchivedPlot).SetFixed(1, 6)

	client.logTextbox = tview.NewTextView()
	client.logTextbox.SetBorder(true).SetTitle("Log").SetTitleAlign(tview.AlignLeft)

	client.logTextbox.ScrollToEnd()

	client.app = tview.NewApplication()

	dirPanel := tview.NewFlex()
	dirPanel.SetDirection(tview.FlexColumn)
	dirPanel.AddItem(client.tmpTable, 0, 1, false)
	dirPanel.AddItem(client.targetTable, 0, 1, false)

	mainPanel := tview.NewFlex()
	mainPanel.SetDirection(tview.FlexRow)
	mainPanel.AddItem(client.activePlotsTable, 0, 1, true)
	mainPanel.AddItem(dirPanel, 0, 1, false)
	mainPanel.AddItem(client.lastTable, 0, 1, false)
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
	host      string
	plotId    string
	status    int
	phase     int
	progress  int
	startTime time.Time
	plotDir   string
	destDir   string
}

var activePlotsHeaders = []string{
	"Host",
	"Plot ID",
	"Status",
	"Phase",
	"Progress",
	"Start Time",
	"Duration",
	"Plot Dir",
	"Dest Dir",
}

func (apd *activePlotsData) Strings() []string {
	status := "Unknown"
	switch apd.status {
	case PlotRunning:
		status = "Running"
	case PlotError:
		status = "Errored"
	case PlotFinished:
		status = "Finished"
	}
	return []string{
		apd.host,
		shortenPlotId(apd.plotId),
		status,
		fmt.Sprintf("%d/4", apd.phase),
		fmt.Sprintf("%d%%", apd.progress),
		apd.startTime.Format("2006-01-02 15:04:05"),
		DurationString(time.Now().Sub(apd.startTime)),
		apd.plotDir,
		apd.destDir,
	}
}

func (apd *activePlotsData) LessThan(Other widget.SortableRow, column int) bool {
	other := Other.(*activePlotsData)
	switch column {
	case 0:
		return apd.host < other.host
	case 1:
		return apd.plotId < other.plotId
	case 2:
		return apd.status < other.status
	case 3:
		return apd.phase < other.phase
	case 4:
		return apd.progress < other.progress
	case 5, 6:
		return apd.startTime.Before(other.startTime)
	case 7:
		return apd.plotDir < other.plotDir
	case 8:
		return apd.destDir < other.destDir
	default:
		panic("unknown column")
	}
}

func (client *Client) makeActivePlotsData(host string, p *ActivePlot) *activePlotsData {
	apd := &activePlotsData{}
	apd.host = host
	apd.plotId = p.Id
	apd.status = p.State
	apd.phase = p.getCurrentPhase()
	apd.progress = p.getProgress()
	apd.startTime = p.getPhaseTime(0)
	apd.plotDir = p.PlotDir
	apd.destDir = p.TargetDir
	return apd
}

func (client *Client) drawActivePlots() {
	activePlotsCount := 0

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

////////////////////////////////////////////

func (client *Client) drawArchivedPlots() {
	client.lastTable.Clear()
	client.lastTable.SetCell(0, 0, tview.NewTableCell("Host"))
	client.lastTable.SetCell(0, 1, tview.NewTableCell("Plot Id"))
	client.lastTable.SetCell(0, 2, tview.NewTableCell("Status"))
	client.lastTable.SetCell(0, 3, tview.NewTableCell("Phase"))
	client.lastTable.SetCell(0, 4, tview.NewTableCell("Start Time"))
	client.lastTable.SetCell(0, 5, tview.NewTableCell("End Time"))
	client.lastTable.SetCell(0, 6, tview.NewTableCell("Duration"))
	client.lastTable.SetCell(0, 7, tview.NewTableCell("Plot Dir"))
	client.lastTable.SetCell(0, 8, tview.NewTableCell("Dest Dir"))

	count := 0
	client.archivedLogs = map[int][]string{}
	for _, host := range client.hosts {
		msg, found := client.msg[host]
		if !found {
			continue
		}
		for _, plot := range msg.Archived {
			state := "Unknown"
			switch plot.State {
			case PlotRunning:
				state = "Running"
			case PlotError:
				state = "Errored"
			case PlotFinished:
				state = "Finished"
			}

			client.lastTable.SetCell(count+1, 0, tview.NewTableCell(host))
			client.lastTable.SetCell(count+1, 1, tview.NewTableCell(shortenPlotId(plot.Id)))
			client.lastTable.SetCell(count+1, 2, tview.NewTableCell(state))
			client.lastTable.SetCell(count+1, 3, tview.NewTableCell(plot.Phase))
			client.lastTable.SetCell(count+1, 4, tview.NewTableCell(plot.StartTime.Format("2006-01-02 15:04:05")))
			client.lastTable.SetCell(count+1, 5, tview.NewTableCell(plot.EndTime.Format("2006-01-02 15:04:05")))
			client.lastTable.SetCell(count+1, 6, tview.NewTableCell(plot.Duration(plot.EndTime)))
			client.lastTable.SetCell(count+1, 7, tview.NewTableCell(plot.PlotDir))
			client.lastTable.SetCell(count+1, 8, tview.NewTableCell(plot.TargetDir))
			client.archivedLogs[count+1] = plot.Tail
			count++
		}
	}
	client.lastTable.SetTitle(fmt.Sprintf(" Archived Plots [%d] ", count))
	client.lastTable.ScrollToBeginning()
}

func (client *Client) selectActivePlot(key string) {
	client.logPlotId = key
	client.lastTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrDim))
	client.activePlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrBold))
	if log, found := client.activeLogs[key]; found {
		client.logTextbox.SetText(strings.Join(log, ""))
		client.logTextbox.ScrollToEnd()

	} else {
		client.logTextbox.SetText("")
	}
}

func (client *Client) selectArchivedPlot(row int, column int) {
	s := ""

	client.logPlotId = ""
	client.activePlotsTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrDim))
	client.lastTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse | tcell.AttrBold))
	if log, found := client.archivedLogs[row]; found {
		for _, line := range log {
			s += line
		}
	}
	client.logTextbox.SetText(strings.TrimSpace(s))
}

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
