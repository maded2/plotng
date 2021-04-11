package main

import (
	"flag"
	"fmt"
	"github.com/ricochet2200/go-disk-usage/du"
	"github.com/rivo/tview"
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

	usage := du.NewDiskUsage("/")
	fmt.Println("Free:", usage.Free()/(KB*KB))
	fmt.Println("Available:", usage.Available()/(KB*KB))
	fmt.Println("Size:", usage.Size()/(KB*KB))
	fmt.Println("Used:", usage.Used()/(KB*KB))
	fmt.Println("Usage:", usage.Usage()*100, "%")

	config = &PlotConfig{
		configPath: *configFile,
	}
	config.Init()

	setupUI()
	app.Run()
}

func setupUI() {
	plotTable = tview.NewTable()
	plotTable.SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Active Plots")

	targetTable = tview.NewTable()
	targetTable.SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Target Directories")

	lastTable = tview.NewTable()
	lastTable.SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Last Plots")

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
