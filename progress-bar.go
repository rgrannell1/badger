package main

import (
	"fmt"
	"sync"
	"time"

	tm "github.com/buger/goterm"
)

type ProgressBar struct {
	count     int64
	completed int64
	lock      sync.Mutex
	start     time.Time
	last      time.Time
}

/*
 * Construct a progress-bar
 */
func NewProgressBar(count int64) *ProgressBar {
	return &ProgressBar{
		count:     count,
		completed: 0,

		lock:  sync.Mutex{},
		start: time.Now(),
		last:  time.Now(),
	}
}

/*
 * Render a progress bar in place
 */
func (bar *ProgressBar) Render() {
	elapsed := bar.last.Sub(bar.start).Seconds()

	if int64(elapsed) == 0 {
		return
	}

	perSecond := ((bar.completed / int64(elapsed)) / 1e6) / 8

	pct := ((float64(bar.completed) / float64(bar.count)) * 100)
	message := "🦡 " + fmt.Sprint(pct) + "% " + fmt.Sprint(perSecond) + "MB/s\n"

	tm.MoveCursor(1, 1)
	tm.Println(message)
	tm.Flush()
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
	bar.Render()
	bar.last = time.Now()
	bar.lock.Unlock()
}
