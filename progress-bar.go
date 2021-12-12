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
	}
}

func (bar *ProgressBar) Render() {
	pct := (float64(bar.completed) / float64(bar.count)) * 100
	message := fmt.Sprint(pct) + "%"

	fmt.Println(message)
}

func (bar *ProgressBar) Update(delta int64) {
	bar.lock.Lock()
	bar.completed += delta
	bar.Render()
	bar.lock.Unlock()
}
