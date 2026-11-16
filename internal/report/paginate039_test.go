package report

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// segments returns the number of rows per page for a pagination result.
func segments(breaks []bool) []int {
	if len(breaks) == 0 {
		return nil
	}
	out := []int{0}
	for i, b := range breaks {
		if b && i > 0 {
			out = append(out, 0)
		}
		out[len(out)-1]++
	}
	return out
}

// checkPagination verifies fit, orphan (QA-17) and widow (QA-18) rules.
func checkPagination(t *testing.T, label string, y float64, rows []float64, fresh bool, breaks []bool) {
	t.Helper()
	if len(breaks) != len(rows) {
		t.Fatalf("%s: %d breaks for %d rows", label, len(breaks), len(rows))
	}
	if len(rows) > 0 && breaks[0] {
		t.Fatalf("%s: first row must not carry a break", label)
	}
	cur := y
	if fresh {
		cur = bodyTop
	}
	cur += tableFirstHeader
	for i, h := range rows {
		if breaks[i] {
			cur = bodyTop + tableContHeader
		}
		cur += h
		if cur > bodyBottom+0.001 {
			t.Fatalf("%s: row %d overflows page (%.1f > %.1f)", label, i, cur, float64(bodyBottom))
		}
	}
	segs := segments(breaks)
	if len(rows) >= 2 {
		for p, n := range segs {
			if n < 2 {
				t.Fatalf("%s: page %d of table holds a single row (segments %v)", label, p, segs)
			}
		}
	}
}

func TestPaginateOrphanHeaderMovesToNextPage(t *testing.T) {
	rows := []float64{40, 40, 40, 40}
	// Header and one row fit, the second row does not.
	y := bodyBottom - tableFirstHeader - 40 - 10
	fresh, breaks := paginateRows(y, rows)
	if !fresh {
		t.Fatal("QA-17: header with a single row left at page end")
	}
	checkPagination(t, "orphan", y, rows, fresh, breaks)
	// With room for two rows the table starts in place.
	y = bodyBottom - tableFirstHeader - 80
	fresh, breaks = paginateRows(y, rows)
	if fresh || breaks[2] != true {
		t.Fatalf("expected start in place and break before row 3, got fresh=%v breaks=%v", fresh, breaks)
	}
	checkPagination(t, "two rows", y, rows, fresh, breaks)
}

func TestPaginateWidowPullsPreviousRow(t *testing.T) {
	// Fresh page: exactly 10 rows of 60 fit, the eleventh would stand alone.
	avail := bodyBottom - bodyTop - tableFirstHeader
	n := int(avail / 60)
	rows := make([]float64, n+1)
	for i := range rows {
		rows[i] = 60
	}
	fresh, breaks := paginateRows(bodyTop, rows)
	if !breaks[n-1] || breaks[n] {
		t.Fatalf("QA-18: last two rows must break together, got %v", segments(breaks))
	}
	checkPagination(t, "widow", bodyTop, rows, fresh, breaks)
}

func TestPaginateShortTables(t *testing.T) {
	for _, rows := range [][]float64{{40}, {40, 40}} {
		y := bodyBottom - tableFirstHeader - 45
		fresh, breaks := paginateRows(y, rows)
		if len(rows) == 1 && fresh {
			t.Fatal("single-row table must start in place when it fits")
		}
		if len(rows) == 2 && !fresh {
			t.Fatal("two-row table must start on a new page when only one row fits")
		}
		checkPagination(t, "short", y, rows, fresh, breaks)
	}
}

func TestPaginateNoOrphansOrWidowsAcrossLengths(t *testing.T) {
	heights := []float64{32, 42, 54, 64, 76, 96, 120}
	for n := 10; n <= 40; n++ {
		for y := float64(bodyTop); y < bodyBottom; y += 7 {
			for k := 0; k < 3; k++ {
				rows := make([]float64, n)
				for i := range rows {
					rows[i] = heights[(i*(k+1)+n)%len(heights)]
				}
				if k == 2 {
					rows[n-1] += 25 // table note kept with final row
				}
				fresh, breaks := paginateRows(y, rows)
				checkPagination(t, fmt.Sprintf("n=%d y=%.0f k=%d", n, y, k), y, rows, fresh, breaks)
			}
		}
	}
}

// QA-17/18 end to end: rendered reports with 10..40 steps and varying lead-in
// never leave a single step row on a page.
func TestPDFStepTableNoOrphansOrWidows(t *testing.T) {
	for n := 10; n <= 40; n++ {
		for lead := 0; lead < 6; lead++ {
			r := sampleReport(t)
			r.Business = strings.TrimSpace(strings.Repeat("Gebäudeautomation Groß GmbH ", 1+lead))
			r.Steps = nil
			for i := 0; i < n; i++ {
				r.Steps = append(r.Steps, inspection.Step{Name: fmt.Sprintf("Messung-%02d", i), Detail: strings.Repeat("Prüfhinweis. ", 1+(i+lead)%9), At: r.Started, Passed: true})
			}
			d, err := layout(r)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.stepPages) != n {
				t.Fatalf("n=%d lead=%d: %d rows placed", n, lead, len(d.stepPages))
			}
			count := map[int]int{}
			for _, p := range d.stepPages {
				count[p]++
			}
			for p, c := range count {
				if c < 2 {
					t.Fatalf("n=%d lead=%d: page %d holds %d step row(s): %v", n, lead, p, c, d.stepPages)
				}
			}
		}
	}
}

func TestPDFStepTablePaginationDeterministic(t *testing.T) {
	r := sampleReport(t)
	a, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("identical report rendered different bytes")
	}
}
