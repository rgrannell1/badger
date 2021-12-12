package main

import (
	"fmt"
	"sync"
	"time"
)

type ProgressBar struct {
	count     int64
	completed int64
	lock      sync.Mutex
	start     time.Time
	last      time.Time
}

func NewProgressBar(count int64) *ProgressBar {
	return &ProgressBar{
		count:     count,
		completed: 0,
		lock:      sync.Mutex{},
		start:     time.Now(),
		last:      time.Now(),
	}
}

func (bar *ProgressBar) Render() {
	elapsed := bar.last.Sub(bar.start).Seconds()

	if int64(elapsed) == 0 {
		return
	}

	perSecond := ((bar.completed / int64(elapsed)) / 1e6) / 8

	pct := ((float64(bar.completed) / float64(bar.count)) * 100)
	message := fmt.Sprint(pct) + "% " + fmt.Sprint(perSecond) + "MB/s"

	fmt.Println(message)
}

func (bar *ProgressBar) Update(delta int64) {
	bar.lock.Lock()
	bar.completed += delta
	bar.Render()
	bar.last = time.Now()
	bar.lock.Unlock()
}
