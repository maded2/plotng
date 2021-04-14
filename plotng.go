package main

import (
	"flag"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/ricochet2200/go-disk-usage/du"
	"github.com/rivo/tview"
	"sort"
	"time"
)

const KB = uint64(1024)

var (
	app           *tview.Application
	plotTable     *tview.Table
	targetTable   *tview.Table
	tmpTable      *tview.Table
	lastTable     *tview.Table
	logTextbox    *tview.TextView
	config        *PlotConfig
	active        map[int64]*ActivePlot
	currentTemp   int
	currentTarget int
)

func main() {
	configFile := flag.String("config", "", "configuration file")
	flag.Parse()
	if flag.Parsed() == false || len(*configFile) == 0 {
		flag.Usage()
		return
	}
	active = map[int64]*ActivePlot{}

	config = &PlotConfig{
		configPath: *configFile,
	}
	config.Init()

	setupUI()

	go createPlot()
	go displayDiskSpaces()
	app.Run()
}

func createPlot() {
	displayActivePlots()
	app.Draw()
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		if config.currentConfig != nil {
			config.lock.RLock()
			if len(active) < config.currentConfig.NumberOfPlots {
				createNewPlot(config.currentConfig)
			}
			config.lock.RUnlock()
		}
		displayActivePlots()
		app.Draw()
	}
}

func displayActivePlots() {
	idList := []int64{}
	for k, _ := range active {
		idList = append(idList, k)
	}
	sort.Slice(idList, func(i, j int) bool {
		return idList[i] < idList[j]
	})

	plotTable.Clear()
	plotTable.SetCell(0, 0, tview.NewTableCell("Start").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	plotTable.SetCell(0, 1, tview.NewTableCell("Duration").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	plotTable.SetCell(0, 2, tview.NewTableCell("Phase").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	plotTable.SetCell(0, 3, tview.NewTableCell("Temp Dir").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	plotTable.SetCell(0, 4, tview.NewTableCell("Target Dir").SetSelectable(false).SetTextColor(tcell.ColorYellow))
	plotTable.SetCell(0, 5, tview.NewTableCell("Id").SetSelectable(false).SetTextColor(tcell.ColorYellow))

	t := time.Now()
	for i, id := range idList {
		plot := active[id]
		plotTable.SetCell(i+1, 0, tview.NewTableCell(plot.startTime.Format("2006-01-02 15:04:05")))
		plotTable.SetCell(i+1, 1, tview.NewTableCell(t.Sub(plot.startTime).String()))
		plotTable.SetCell(i+1, 2, tview.NewTableCell(plot.phase))
		plotTable.SetCell(i+1, 3, tview.NewTableCell(plot.plotDir))
		plotTable.SetCell(i+1, 4, tview.NewTableCell(plot.targetDir))
		plotTable.SetCell(i+1, 5, tview.NewTableCell(plot.id))
	}
}

func createNewPlot(config *Config) {
	if len(config.TempDirectory) == 0 || len(config.TargetDirectory) == 0 {
		return
	}
	if currentTemp >= len(config.TempDirectory) {
		currentTemp = 0
	}
	plotDir := config.TempDirectory[currentTemp]
	currentTemp++

	if currentTarget >= len(config.TargetDirectory) {
		currentTarget = 0
	}
	targetDir := config.TargetDirectory[currentTarget]
	currentTarget++
	t := time.Now()
	plot := &ActivePlot{
		plotId:      t.Unix(),
		startTime:   t,
		targetDir:   targetDir,
		plotDir:     plotDir,
		fingerprint: config.Fingerprint,
		phase:       "",
		tail:        nil,
		state:       PlotFinished,
	}
	active[plot.plotId] = plot
}

func displayDiskSpaces() {
	time.Sleep(5 * time.Second)
	drawTargetTable(targetTable, true)
	drawTargetTable(tmpTable, false)
	app.Draw()
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		drawTargetTable(targetTable, true)
		drawTargetTable(tmpTable, false)
		app.Draw()
	}
}

func drawTargetTable(table *tview.Table, drawTarget bool) {
	if config.currentConfig != nil {
		table.Clear()
		table.SetCell(0, 0, tview.NewTableCell("Directory").SetSelectable(false).SetTextColor(tcell.ColorYellow))
		table.SetCell(0, 1, tview.NewTableCell("Available Space").SetSelectable(false).SetTextColor(tcell.ColorYellow))
		config.lock.RLock()
		paths := config.currentConfig.TargetDirectory
		if !drawTarget {
			paths = config.currentConfig.TempDirectory
		}
		for i, path := range paths {
			table.SetCell(i+1, 0, tview.NewTableCell(path))
			availableSpace := getDiskSpaceAvailable(path) / (KB * KB * KB)
			table.SetCell(i+1, 1, tview.NewTableCell(fmt.Sprintf("%d GB", availableSpace)).SetAlign(tview.AlignRight))
		}
		config.lock.RUnlock()
	}
}

func getDiskSpaceAvailable(path string) uint64 {
	d := du.NewDiskUsage(path)
	return d.Available()
}

func setupUI() {
	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	plotTable = tview.NewTable()
	plotTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Active Plots")
	plotTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	tmpTable = tview.NewTable()
	tmpTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Temp Directories")
	tmpTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	targetTable = tview.NewTable()
	targetTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Target Directories")
	targetTable.SetSelectedStyle(tcell.StyleDefault.Attributes(tcell.AttrReverse))

	lastTable = tview.NewTable()
	lastTable.SetSelectable(true, false).SetBorder(true).SetTitleAlign(tview.AlignLeft).SetTitle("Last Plots")

	logTextbox = tview.NewTextView()
	logTextbox.SetBorder(true).SetTitle("Log").SetTitleAlign(tview.AlignLeft)

	app = tview.NewApplication()

	dirPanel := tview.NewFlex()
	dirPanel.SetDirection(tview.FlexColumn)
	dirPanel.AddItem(tmpTable, 0, 1, false)
	dirPanel.AddItem(targetTable, 0, 1, false)

	mainPanel := tview.NewFlex()
	mainPanel.SetDirection(tview.FlexRow)
	mainPanel.AddItem(plotTable, 0, 1, true)
	mainPanel.AddItem(dirPanel, 0, 1, false)
	mainPanel.AddItem(lastTable, 0, 1, false)
	mainPanel.AddItem(logTextbox, 0, 1, false)

	app = tview.NewApplication()
	app.SetRoot(mainPanel, true)
	app.EnableMouse(true)
}
