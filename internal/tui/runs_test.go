package tui

import "testing"

func TestBuildRunsTableColumnsFillWidth(t *testing.T) {
	const tableWidth = 120

	cols := buildRunsTableColumns(tableWidth)
	if len(cols) != len(runsTableColumnSpecs) {
		t.Fatalf("column count = %d, want %d", len(cols), len(runsTableColumnSpecs))
	}

	total := len(cols) * runsTableColumnPadding
	for _, col := range cols {
		total += col.Width
	}

	if total != tableWidth {
		t.Fatalf("total rendered width = %d, want %d", total, tableWidth)
	}
}

func TestBuildRunsTableColumnsShrinkToMinimums(t *testing.T) {
	const tableWidth = 80

	cols := buildRunsTableColumns(tableWidth)

	total := len(cols) * runsTableColumnPadding
	for i, col := range cols {
		total += col.Width
		if col.Width < runsTableColumnSpecs[i].minWidth {
			t.Fatalf("column %q width = %d, want >= %d", col.Title, col.Width, runsTableColumnSpecs[i].minWidth)
		}
	}

	if total != tableWidth {
		t.Fatalf("total rendered width = %d, want %d", total, tableWidth)
	}
}
