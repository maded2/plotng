package internal

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/ricochet2200/go-disk-usage/du"
	"github.com/rivo/tview"
	"sort"
	"time"
)

type Client struct {
	app             *tview.Application
	plotTable       *tview.Table
	targetTable     *tview.Table
	tmpTable        *tview.Table
	lastTable       *tview.Table
	logTextbox      *tview.TextView
	active          map[int64]*ActivePlot
	currentTemp     int
	currentTarget   int
	TargetDirectory []string
	TempDirectory   []string
}

func (client *Client) ProcessLoop() {
	client.setupUI()

	go client.displayDiskSpaces()
	client.app.Run()
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
		client.plotTable.SetCell(i+1, 1, tview.NewTableCell(t.Sub(plot.StartTime).String()))
		client.plotTable.SetCell(i+1, 2, tview.NewTableCell(plot.Phase))
		client.plotTable.SetCell(i+1, 3, tview.NewTableCell(plot.PlotDir))
		client.plotTable.SetCell(i+1, 4, tview.NewTableCell(plot.TargetDir))
		client.plotTable.SetCell(i+1, 5, tview.NewTableCell(plot.Id))
	}
}

func (client *Client) displayDiskSpaces() {
	time.Sleep(5 * time.Second)
	client.drawTargetTable(client.targetTable, true)
	client.drawTargetTable(client.tmpTable, false)
	client.app.Draw()
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		client.drawTargetTable(client.targetTable, true)
		client.drawTargetTable(client.tmpTable, false)
		client.app.Draw()
	}
}

func (client *Client) drawTargetTable(table *tview.Table, drawTarget bool) {
	table.Clear()
	table.SetCell(0, 0, tview.NewTableCell("Directory").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	table.SetCell(0, 1, tview.NewTableCell("Available Space").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	paths := client.TargetDirectory
	if !drawTarget {
		paths = client.TempDirectory
	}
	for i, path := range paths {
		table.SetCell(i+1, 0, tview.NewTableCell(path))
		availableSpace := client.getDiskSpaceAvailable(path) / (KB * KB * KB)
		table.SetCell(i+1, 1, tview.NewTableCell(fmt.Sprintf("%d GB", availableSpace)).SetAlign(tview.AlignRight))
	}
}

func (client *Client) getDiskSpaceAvailable(path string) uint64 {
	d := du.NewDiskUsage(path)
	return d.Available()
}

func (client *Client) setupUI() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	client.plotTable = tview.NewTable()
	client.plotTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Active Plots")
	client.plotTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.tmpTable = tview.NewTable()
	client.tmpTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Temp Directories")
	client.tmpTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.targetTable = tview.NewTable()
	client.targetTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Target Directories")
	client.targetTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	client.lastTable = tview.NewTable()
	client.lastTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Last Plots")

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
