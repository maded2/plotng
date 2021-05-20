package widget

import (
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SortableRow interface {
	Strings() []string
	LessThan(Other SortableRow, column int) bool
}

type tableRow struct {
	key  string
	data SortableRow
}

// SortedTable is a wrapper around tview.Table which provides sortable column headers.  Rows are
// identified by a key rather than by index.
type SortedTable struct {
	table       *tview.Table
	values      []tableRow
	curRow      int
	curKey      string
	sortColumn  int
	sortReverse bool

	columnAlign map[int]int

	selectionChangedFunc func(key string)
}

// Mostly implement tview.Primitive with proxies
func (st *SortedTable) Draw(screen tcell.Screen) {
	st.Redraw()
	st.table.Draw(screen)
}

func (st *SortedTable) GetRect() (int, int, int, int) {
	return st.table.GetRect()
}

func (st *SortedTable) SetRect(x, y, width, height int) {
	st.table.SetRect(x, y, width, height)
}

func (st *SortedTable) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return st.table.InputHandler()
}

func (st *SortedTable) Focus(delegate func(p tview.Primitive)) {
	st.table.Focus(delegate)
}

func (st *SortedTable) HasFocus() bool {
	return st.table.HasFocus()
}

func (st *SortedTable) Blur() {
	st.table.Blur()
}

func (st *SortedTable) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		fn := st.table.MouseHandler()
		consumed, capture = fn(action, event, func(p tview.Primitive) {
			if p == st.table {
				p = st
			}
			setFocus(p)
		})
		if capture == st.table {
			capture = st
		}
		return consumed, capture
	}
}

func NewSortedTable() *SortedTable {
	st := &SortedTable{
		table:       tview.NewTable(),
		columnAlign: make(map[int]int),
	}
	st.table.SetFixed(1, 0)
	st.table.InsertRow(0)
	st.table.SetSelectionChangedFunc(st.selectionChanged)
	return st
}

func (st *SortedTable) SetSelectable(selectable bool) *SortedTable {
	st.table.SetSelectable(selectable, false)
	return st
}

func (st *SortedTable) SetBorder(show bool) *SortedTable {
	st.table.SetBorder(show)
	return st
}

func (st *SortedTable) SetTitleAlign(align int) *SortedTable {
	st.table.SetTitleAlign(align)
	return st
}

func (st *SortedTable) SetTitle(title string) *SortedTable {
	st.table.SetTitle(title)
	return st
}

func (st *SortedTable) SetSelectedStyle(style tcell.Style) *SortedTable {
	st.table.SetSelectedStyle(style)
	return st
}

func (st *SortedTable) selectionChanged(row, column int) {
	if row <= 0 {
		if st.curRow > 0 {
			st.table.Select(st.curRow, 0)
		}
	} else {
		st.curRow = row
		if st.curKey != st.values[row-1].key {
			st.curKey = st.values[row-1].key
			if st.selectionChangedFunc != nil {
				st.selectionChangedFunc(st.curKey)
			}
		}
	}
}

func (st *SortedTable) SetSelectionChangedFunc(handler func(key string)) *SortedTable {
	st.selectionChangedFunc = handler
	return st
}

func (st *SortedTable) SetHeaders(headers ...string) *SortedTable {
	for colIndex := len(headers); colIndex < st.table.GetColumnCount(); colIndex++ {
		cell := st.table.GetCell(0, colIndex)
		cell.Text = ""
		cell.Clicked = nil
	}
	for c, h := range headers {
		cell := tview.NewTableCell(h)
		cell.NotSelectable = true
		cell.Clicked = st.setSortColumn(c)
		st.table.SetCell(0, c, cell)
	}
	return st
}

func (st *SortedTable) Clear() *SortedTable {
	st.values = nil
	return st
}

func (st *SortedTable) Keys() []string {
	keys := make([]string, 0, len(st.values))
	for _, row := range st.values {
		keys = append(keys, row.key)
	}
	return keys
}

func (st *SortedTable) SetRowData(key string, data SortableRow) *SortedTable {
	found := false
	for idx, dr := range st.values {
		if dr.key == key {
			st.values[idx].data = data
			found = true
			break
		}
	}
	if !found {
		st.values = append(st.values, tableRow{key, data})
	}
	return st
}

func (st *SortedTable) ClearRowData(key string) *SortedTable {
	newValues := st.values[:0]
	for _, row := range st.values {
		if row.key != key {
			newValues = append(newValues, row)
		}
	}
	for i := len(newValues); i < len(st.values); i++ {
		st.values[i] = tableRow{}
	}
	st.values = newValues
	return st
}

func (st *SortedTable) setSortColumn(col int) func() bool {
	return func() bool {
		if st.sortColumn == col {
			st.sortReverse = !st.sortReverse
		} else {
			st.sortColumn = col
			st.sortReverse = false
		}
		return true
	}
}

func (st *SortedTable) redrawHeaders() {
	for c := 0; c < st.table.GetColumnCount(); c++ {
		if c == st.sortColumn {
			if !st.sortReverse {
				st.table.GetCell(0, c).SetTextColor(tcell.ColorGreen)
			} else {
				st.table.GetCell(0, c).SetTextColor(tcell.ColorRed)
			}
		} else {
			st.table.GetCell(0, c).SetTextColor(tcell.ColorYellow)
		}
	}
}

func (st *SortedTable) GetSelection() string {
	if st.curRow > 0 {
		return st.values[st.curRow-1].key
	}
	return ""
}

func (st *SortedTable) Select(key string) *SortedTable {
	for row, value := range st.values {
		if value.key == key {
			st.table.Select(row+1, 0)
			break
		}
	}
	return st
}

func (st *SortedTable) sortData() {
	sort.SliceStable(st.values, func(row1, row2 int) bool {
		row1Value := st.values[row1].data
		row2Value := st.values[row2].data
		if row2Value == nil {
			return true
		} else if row1Value == nil {
			return false
		} else {
			return row1Value.LessThan(row2Value, st.sortColumn) != st.sortReverse
		}
	})
}

func (st *SortedTable) SetColumnAlign(col int, align int) *SortedTable {
	st.columnAlign[col] = align
	return st
}

func (st *SortedTable) updateData() {
	for rowIndex, rowData := range st.values {
		strData := rowData.data.Strings()
		colIndex := 0
		for ; colIndex < len(strData); colIndex++ {
			cellText := strData[colIndex]
			cell := tview.NewTableCell(cellText)
			if align, ok := st.columnAlign[colIndex]; ok {
				cell.Align = align
			}
			st.table.SetCell(rowIndex+1, colIndex, cell)
		}
		for ; colIndex < st.table.GetColumnCount(); colIndex++ {
			cell := tview.NewTableCell("")
			st.table.SetCell(rowIndex+1, colIndex, cell)
		}
	}
	for st.table.GetRowCount() > len(st.values)+1 {
		st.table.RemoveRow(st.table.GetRowCount() - 1)
	}
}

func (st *SortedTable) Redraw() {
	st.redrawHeaders()
	selectedKey := st.GetSelection()
	st.sortData()
	st.updateData()
	st.Select(selectedKey)
}
