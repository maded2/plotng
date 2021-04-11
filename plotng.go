package main

import (
	"flag"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/ricochet2200/go-disk-usage/du"
	"github.com/rivo/tview"
	"time"
)

const KB = uint64(1024)

var (
	app         *tview.Application
	plotTable   *tview.Table
	targetTable *tview.Table
	lastTable   *tview.Table
	logTextbox  *tview.TextView
	config      *PlotConfig
)

func main() {
	configFile := flag.String("config", "", "configuration file")
	flag.Parse()
	if flag.Parsed() == false || len(*configFile) == 0 {
		flag.Usage()
		return
	}

	config = &PlotConfig{
		configPath: *configFile,
	}
	config.Init()

	setupUI()

	go displayDiskSpaces()
	app.Run()
}

func displayDiskSpaces() {
	time.Sleep(5 * time.Second)
	drawTargetTable()
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		drawTargetTable()
	}
}

func drawTargetTable() {
	if config.currentConfig != nil {
		targetTable.Clear()
		targetTable.SetCell(0, 0, tview.NewTableCell("Directory").SetSelectable(false).SetTextColor(tcell.ColorYellow))
		targetTable.SetCell(0, 1, tview.NewTableCell("Available Space").SetSelectable(false).SetTextColor(tcell.ColorYellow))
		config.lock.RLock()
		for i, path := range config.currentConfig.TargetDirectory {
			targetTable.SetCell(i+1, 0, tview.NewTableCell(path))
			d := du.NewDiskUsage(path)
			availableSpace := d.Available() / (KB * KB * KB)
			targetTable.SetCell(i+1, 1, tview.NewTableCell(fmt.Sprintf("%d GB", availableSpace)).SetAlign(tview.AlignRight))
		}
		config.lock.RUnlock()
		app.Draw()
	}
}

func setupUI() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	plotTable = tview.NewTable()
	plotTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Active Plots")

	targetTable = tview.NewTable()
	targetTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Target Directories")
	targetTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	lastTable = tview.NewTable()
	lastTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Last Plots")

	logTextbox = tview.NewTextView()
	logTextbox.SetBorder(true).SetTitle("Log").SetTitleAlign(tview.AlignLeft)

	app = tview.NewApplication()
	mainPanel := tview.NewFlex()
	mainPanel.SetDirection(tview.FlexRow)
	mainPanel.AddItem(plotTable, 0, 1, true)
	mainPanel.AddItem(targetTable, 0, 1, false)
	mainPanel.AddItem(lastTable, 0, 1, false)
	mainPanel.AddItem(logTextbox, 0, 1, false)

	app = tview.NewApplication()
	app.SetRoot(mainPanel, true)
	app.EnableMouse(true)
}
