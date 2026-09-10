package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/timzifer/blazon/blazontest"
	"github.com/timzifer/blazon/internal/phash"
)

// printReport writes a distance report as a table plus a histogram.
//
// The point of printing it rather than only asserting on it is calibration: a
// threshold set from a histogram is a measurement, and one set from intuition
// is a guess that will either never fire or fire on the first honest change.
func printReport(w io.Writer, rep *blazontest.Report) {
	fmt.Fprintf(w, "renderer %s, policy %s, %d versions, distances over %d bits\n",
		rep.Renderer, rep.Policy, rep.Versions, phash.MaxDistance)
	fmt.Fprintf(w, "%-14s %7s %5s %7s %5s\n", "relation", "pairs", "min", "mean", "max")

	rows := []struct {
		name string
		s    blazontest.Stats
	}{
		{"patch", rep.Patch},
		{"minor", rep.Minor},
		{"major", rep.Major},
		{"prerelease", rep.Prerelease},
		{"all", rep.All},
		{"intra-major", rep.IntraMajor},
		{"inter-major", rep.InterMajor},
	}
	for _, r := range rows {
		if r.s.Count == 0 {
			fmt.Fprintf(w, "%-14s %7d %5s %7s %5s\n", r.name, 0, "-", "-", "-")
			continue
		}
		fmt.Fprintf(w, "%-14s %7d %5d %7.1f %5d\n", r.name, r.s.Count, r.s.Min, r.s.Mean, r.s.Max)
	}

	fmt.Fprintf(w, "\nclosest pairs overall: %s\n", rep.All.WorstString(6))
	if rep.Patch.Count > 0 {
		fmt.Fprintf(w, "closest patch pairs:   %s\n", rep.Patch.WorstString(6))
	}

	fmt.Fprintln(w, "\nall-pair distance histogram:")
	writeHistogram(w, rep.All)

	// The suggested thresholds are what a first calibration would freeze:
	// comfortably below each observed minimum, so an honest change has room
	// but a regression does not.
	fmt.Fprintln(w, "\nsuggested thresholds (about 85% of each observed minimum):")
	fmt.Fprintf(w, "  MinPatch: %d, MinMinor: %d, MinMajor: %d, Floor: %d\n",
		suggest(rep.Patch), suggest(rep.Minor), suggest(rep.Major), suggest(rep.All))
}

func suggest(s blazontest.Stats) int {
	if s.Count == 0 {
		return 0
	}
	v := s.Min * 70 / 100
	if v < 1 {
		return 1
	}
	return v
}

// histogramBucket groups the 129 possible distances into readable rows.
const histogramBucket = 4

func writeHistogram(w io.Writer, s blazontest.Stats) {
	if s.Count == 0 {
		return
	}
	buckets := make([]int, (phash.MaxDistance/histogramBucket)+1)
	peak := 0
	for d, n := range s.Hist {
		b := d / histogramBucket
		buckets[b] += n
		if buckets[b] > peak {
			peak = buckets[b]
		}
	}
	if peak == 0 {
		return
	}
	const width = 48
	for b, n := range buckets {
		if n == 0 {
			continue
		}
		bar := n * width / peak
		if bar == 0 {
			bar = 1
		}
		fmt.Fprintf(w, "  %3d-%-3d %7d %s\n",
			b*histogramBucket, b*histogramBucket+histogramBucket-1, n, strings.Repeat("#", bar))
	}
}
