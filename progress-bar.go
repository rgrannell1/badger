package main

import (
	"fmt"
	"math"
	"os"
	"sync"
	"text/template"
	"time"
)

type ProgressBar struct {
	count      int64
	completed  int64
	lock       sync.Mutex
	start      time.Time
	last       time.Time
	facts      *Facts
	photoCount int
	rawCount   int
	videoCount int
}

type ProgressView struct {
	Percentage  float64
	RateMB      float64
	CopiedMB    int
	TotalMB     int
	RemainingMB int
	Src         string
	Dst         string
	Facts       Facts
	Count       int
	PhotoCount  int
	RawCount    int
	VideoCount  int
}

const ProgressBarTemplate = `
🦡
Clustered & Copied {{.Percentage}}% Media @ {{.RateMB}}MB/s

Copied {{.Src}} -> {{.Dst}}

Copied:      {{.CopiedMB}}MB
Total:       {{.TotalMB}}MB
Remaining:   {{.RemainingMB}}MB

Photos:      {{.PhotoCount}} / {{.Facts.PhotoCount}}
Raw Images:  {{.RawCount}} / {{.Facts.RawCount}}
Videos:      {{.VideoCount}} / {{.Facts.VideoCount}}
`

/*
 * Construct a progress-bar
 */
func NewProgressBar(count int64, facts *Facts) *ProgressBar {
	return &ProgressBar{
		count:     count,
		completed: 0,

		lock:  sync.Mutex{},
		start: time.Now(),
		last:  time.Now(),
		facts: facts,
	}
}

/*
 * Render a progress bar in place
 */
func (bar *ProgressBar) Render(media *Media) {
	pct := (float64(bar.completed) / float64(bar.count)) * 100

	copied := bar.completed / 1e6
	total := bar.count / 1e6
	remaining := (bar.count - bar.completed) / 1e6

	switch media.GetType() {
	case PHOTO:
		bar.photoCount++
	case RAW:
		bar.rawCount++
	case VIDEO:
		bar.videoCount++
	}

	view := ProgressView{
		Percentage:  math.Round(pct*100) / 100,
		RateMB:      0,
		CopiedMB:    int(copied),
		TotalMB:     int(total),
		RemainingMB: int(remaining),
		Src:         media.source,
		Dst:         media.GetChosenName(),
		Facts:       *bar.facts,
		Count:       int(bar.count),
		PhotoCount:  bar.photoCount,
		RawCount:    bar.rawCount,
		VideoCount:  bar.videoCount,
	}
	tmpl, err := template.New("progress-bar").Parse(ProgressBarTemplate)

	if err != nil {
		panic(err)
	}

	fmt.Print("\033[H\033[2J")
	err = tmpl.Execute(os.Stdout, view)
	if err != nil {
		panic(err)
	}
}

/*
 * Update progress information
 */
func (bar *ProgressBar) Update(media *Media) {
	bar.lock.Lock()

	size, err := media.Size()
	if err != nil {
		panic(err)
	}

	bar.completed += size
	bar.Render(media)
	bar.last = time.Now()
	bar.lock.Unlock()
}
