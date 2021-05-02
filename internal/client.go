package internal

import (
	"encoding/gob"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"net/http"
	"sort"
	"time"
)

type Client struct {
	app                 *tview.Application
	plotTable           *tview.Table
	targetTable         *tview.Table
	tmpTable            *tview.Table
	lastTable           *tview.Table
	logTextbox          *tview.TextView
	active              map[int64]*ActivePlot
	host                string
	port                int
	msg                 *Msg
	archivedTableActive bool
}

func (client *Client) ProcessLoop(host string, port int) {
	client.host = host
	client.port = port

	gob.Register(Msg{})
	gob.Register(ActivePlot{})

	client.setupUI()

	go client.app.Run()
	client.checkServer()
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		client.checkServer()
	}
}

func (client *Client) displayActivePlots() {
	idList := []int64{}
	for k, _ := range client.active {
		idList = append(idList, k)
	}
	sort.Slice(idList, func(i, j int) bool {
		return idList[i] < idList[j]
	})

	client.plotTable.Clear()
	client.plotTable.SetCell(0, 0, tview.NewTableCell("Start").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.plotTable.SetCell(0, 1, tview.NewTableCell("Duration").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.plotTable.SetCell(0, 2, tview.NewTableCell("Phase").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.plotTable.SetCell(0, 3, tview.NewTableCell("Temp Dir").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.plotTable.SetCell(0, 4, tview.NewTableCell("Target Dir").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.plotTable.SetCell(0, 5, tview.NewTableCell("Id").SetSelectable(false).SetTextColor(tcell.ColorYellow))

	t := time.Now()
	for i, id := range idList {
		plot := client.active[id]
		client.plotTable.SetCell(i+1, 0, tview.NewTableCell(plot.StartTime.Format("2006-01-02 15:04:05")))
		client.plotTable.SetCell(i+1, 1, tview.NewTableCell(plot.Duration(t)))
		client.plotTable.SetCell(i+1, 2, tview.NewTableCell(plot.Phase))
		client.plotTable.SetCell(i+1, 3, tview.NewTableCell(plot.PlotDir))
		client.plotTable.SetCell(i+1, 4, tview.NewTableCell(plot.TargetDir))
		client.plotTable.SetCell(i+1, 5, tview.NewTableCell(plot.Id))
	}
}

func (client *Client) checkServer() {
	c := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/", client.host, client.port), nil)
	if err != nil {
		client.logTextbox.SetText(err.Error())
	}
	if resp, err := c.Do(req); err == nil {
		defer resp.Body.Close()
		var msg Msg
		decoder := gob.NewDecoder(resp.Body)
		if err := decoder.Decode(&msg); err == nil {
			client.msg = &msg
			sort.Slice(client.msg.Actives, func(i, j int) bool {
				return client.msg.Actives[i].PlotId < client.msg.Actives[j].PlotId
			})
			sort.Slice(client.msg.Archived, func(i, j int) bool {
				return client.msg.Archived[i].PlotId > client.msg.Archived[j].PlotId
			})

		} else {
			client.logTextbox.SetText(fmt.Sprintf("Failed to decode msg: %s", err))
		}
	} else {
		client.logTextbox.SetText(err.Error())
	}

	client.drawTempTable()
	client.drawTargetTable()
	client.drawActivePlots()
	client.app.Draw()
}

func (client *Client) drawTempTable() {
	if client.msg == nil {
		return
	}
	client.tmpTable.Clear()
	client.tmpTable.SetCell(0, 0, tview.NewTableCell("Directory").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 1, tview.NewTableCell("Available Space").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 2, tview.NewTableCell("Avg Phase1").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 3, tview.NewTableCell("Avg Phase2").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 4, tview.NewTableCell("Avg Phase3").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.tmpTable.SetCell(0, 5, tview.NewTableCell("Avg Phase4").SetSelectable(false).SetTextColor(tcell.ColorYellow))

	pathList := []string{}
	for k, _ := range client.msg.TempDirs {
		pathList = append(pathList, k)
	}
	sort.Strings(pathList)
	for i, path := range pathList {
		client.tmpTable.SetCell(i+1, 0, tview.NewTableCell(path))
		availableSpace := client.msg.TempDirs[path] / GB
		client.tmpTable.SetCell(i+1, 1, tview.NewTableCell(fmt.Sprintf("%d GB", availableSpace)).SetAlign(tview.AlignRight))
		client.tmpTable.SetCell(i+1, 2, tview.NewTableCell(client.AvgPhase1()).SetAlign(tview.AlignRight))
		client.tmpTable.SetCell(i+1, 3, tview.NewTableCell(client.AvgPhase2()).SetAlign(tview.AlignRight))
		client.tmpTable.SetCell(i+1, 4, tview.NewTableCell(client.AvgPhase3()).SetAlign(tview.AlignRight))
		client.tmpTable.SetCell(i+1, 5, tview.NewTableCell(client.AvgPhase4()).SetAlign(tview.AlignRight))
	}
}

func (client *Client) drawTargetTable() {
	if client.msg == nil {
		return
	}
	client.targetTable.Clear()
	client.targetTable.SetCell(0, 0, tview.NewTableCell("Directory").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.targetTable.SetCell(0, 1, tview.NewTableCell("Available Space").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	client.targetTable.SetCell(0, 2, tview.NewTableCell("Avg Plot Time").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	pathList := []string{}
	for k, _ := range client.msg.TargetDirs {
		pathList = append(pathList, k)
	}
	sort.Strings(pathList)
	for i, path := range pathList {
		client.targetTable.SetCell(i+1, 0, tview.NewTableCell(path))
		availableSpace := client.msg.TargetDirs[path] / GB
		client.targetTable.SetCell(i+1, 1, tview.NewTableCell(fmt.Sprintf("%d GB", availableSpace)).SetAlign(tview.AlignRight))
		client.targetTable.SetCell(i+1, 2, tview.NewTableCell(client.computeAvgTargetTime(path)).SetAlign(tview.AlignRight))
	}
}

func (client *Client) setupUI() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	client.plotTable = tview.NewTable()
	client.plotTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Active Plots")
	client.plotTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	client.plotTable.SetSelectionChangedFunc(client.selectActivePlot).SetFixed(1, 6)

	client.tmpTable = tview.NewTable()
	client.tmpTable.SetSelectable(false, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Plot Directories")
	client.tmpTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.targetTable = tview.NewTable()
	client.targetTable.SetSelectable(false, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Dest Directories")
	client.targetTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.lastTable = tview.NewTable()
	client.lastTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Archived Plots")
	client.lastTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))
	client.lastTable.SetSelectionChangedFunc(client.selectArchivedPlot).SetFixed(1, 6)

	client.logTextbox = tview.NewTextView()
	client.logTextbox.SetBorder(true).SetTitle("Log").SetTitleAlign(tview.AlignLeft)

	client.app = tview.NewApplication()

	dirPanel := tview.NewFlex()
	dirPanel.SetDirection(tview.FlexColumn)
	dirPanel.AddItem(client.tmpTable, 0, 1, false)
	dirPanel.AddItem(client.targetTable, 0, 1, false)

	mainPanel := tview.NewFlex()
	mainPanel.SetDirection(tview.FlexRow)
	mainPanel.AddItem(client.plotTable, 0, 1, true)
	mainPanel.AddItem(dirPanel, 0, 1, false)
	mainPanel.AddItem(client.lastTable, 0, 1, false)
	mainPanel.AddItem(client.logTextbox, 0, 1, false)

	client.app = tview.NewApplication()
	client.app.SetRoot(mainPanel, true)
	client.app.EnableMouse(true)
}

func (client *Client) drawActivePlots() {
	if client.msg == nil {
		return
	}
	client.plotTable.Clear()
	client.plotTable.SetCell(0, 0, tview.NewTableCell("Plot Id"))
	client.plotTable.SetCell(0, 1, tview.NewTableCell("Status"))
	client.plotTable.SetCell(0, 2, tview.NewTableCell("Phase"))
	client.plotTable.SetCell(0, 3, tview.NewTableCell("Progress"))
	client.plotTable.SetCell(0, 4, tview.NewTableCell("Start Time"))
	client.plotTable.SetCell(0, 5, tview.NewTableCell("Duration"))
	client.plotTable.SetCell(0, 6, tview.NewTableCell("Plot Dir"))
	client.plotTable.SetCell(0, 7, tview.NewTableCell("Dest Dir"))

	t := time.Now()
	for i, plot := range client.msg.Actives {
		state := "Unknown"
		switch plot.State {
		case PlotRunning:
			state = "Running"
		case PlotError:
			state = "Errored"
		case PlotFinished:
			state = "Finished"
		}

		client.plotTable.SetCell(i+1, 0, tview.NewTableCell(plot.Id))
		client.plotTable.SetCell(i+1, 1, tview.NewTableCell(state))
		client.plotTable.SetCell(i+1, 2, tview.NewTableCell(plot.Phase).SetAlign(tview.AlignRight))
		client.plotTable.SetCell(i+1, 3, tview.NewTableCell(plot.Progress).SetAlign(tview.AlignRight))
		client.plotTable.SetCell(i+1, 4, tview.NewTableCell(plot.StartTime.Format("2006-01-02 15:04:05")))
		client.plotTable.SetCell(i+1, 5, tview.NewTableCell(plot.Duration(t)))
		client.plotTable.SetCell(i+1, 6, tview.NewTableCell(plot.PlotDir))
		client.plotTable.SetCell(i+1, 7, tview.NewTableCell(plot.TargetDir))
	}
	client.plotTable.ScrollToBeginning()

	client.lastTable.Clear()
	client.lastTable.SetCell(0, 0, tview.NewTableCell("Plot Id"))
	client.lastTable.SetCell(0, 1, tview.NewTableCell("Status"))
	client.lastTable.SetCell(0, 2, tview.NewTableCell("Phase"))
	client.lastTable.SetCell(0, 3, tview.NewTableCell("Start Time"))
	client.lastTable.SetCell(0, 4, tview.NewTableCell("End Time"))
	client.lastTable.SetCell(0, 5, tview.NewTableCell("Duration"))
	client.lastTable.SetCell(0, 6, tview.NewTableCell("Plot Dir"))
	client.lastTable.SetCell(0, 7, tview.NewTableCell("Dest Dir"))

	for i, plot := range client.msg.Archived {
		state := "Unknown"
		switch plot.State {
		case PlotRunning:
			state = "Running"
		case PlotError:
			state = "Errored"
		case PlotFinished:
			state = "Finished"
		}

		client.lastTable.SetCell(i+1, 0, tview.NewTableCell(plot.Id))
		client.lastTable.SetCell(i+1, 1, tview.NewTableCell(state))
		client.lastTable.SetCell(i+1, 2, tview.NewTableCell(plot.Phase))
		client.lastTable.SetCell(i+1, 3, tview.NewTableCell(plot.StartTime.Format("2006-01-02 15:04:05")))
		client.lastTable.SetCell(i+1, 4, tview.NewTableCell(plot.EndTime.Format("2006-01-02 15:04:05")))
		client.lastTable.SetCell(i+1, 5, tview.NewTableCell(plot.Duration(plot.EndTime)))
		client.lastTable.SetCell(i+1, 6, tview.NewTableCell(plot.PlotDir))
		client.lastTable.SetCell(i+1, 7, tview.NewTableCell(plot.TargetDir))
	}
	client.lastTable.ScrollToBeginning()

}

func (client *Client) selectActivePlot(row int, column int) {
	s := ""
	if client.msg == nil || row <= 0 || row > len(client.msg.Actives) {
		client.logTextbox.SetText(s)
		return
	}
	plot := client.msg.Actives[row-1]
	for _, line := range plot.Tail {
		s += line
	}
	client.logTextbox.SetText(s)
}

func (client *Client) selectArchivedPlot(row int, column int) {
	s := ""
	if client.msg == nil || row <= 0 || row > len(client.msg.Archived) {
		client.logTextbox.SetText(s)
		return
	}
	plot := client.msg.Archived[row-1]
	for _, line := range plot.Tail {
		s += line
	}
	client.logTextbox.SetText(s)
}

func (client *Client) computeAvgTargetTime(path string) string {
	var count int64
	var total int64
	for _, plot := range client.msg.Archived {
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

func (client *Client) AvgPhase1() string {
	var count int64
	var total int64
	for _, plot := range client.msg.Archived {
		if plot.Phase1Time.IsZero() || plot.State != PlotFinished {
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

func (client *Client) AvgPhase2() string {
	var count int64
	var total int64
	for _, plot := range client.msg.Archived {
		if plot.Phase1Time.IsZero() || plot.Phase2Time.IsZero() || plot.State != PlotFinished {
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

func (client *Client) AvgPhase3() string {
	var count int64
	var total int64
	for _, plot := range client.msg.Archived {
		if plot.Phase2Time.IsZero() || plot.Phase3Time.IsZero() || plot.State != PlotFinished {
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

func (client *Client) AvgPhase4() string {
	var count int64
	var total int64
	for _, plot := range client.msg.Archived {
		if plot.Phase3Time.IsZero() || plot.State != PlotFinished {
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
