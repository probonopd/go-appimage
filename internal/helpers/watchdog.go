package helpers

import (
	"time"
)

// Watchdog calls a function once after a certain delay has elapsed; kicking it resets the delay.
// Kicking it after the function has already been called restarts it.
// https://codereview.stackexchange.com/questions/144273/watchdog-in-golang
type Watchdog struct {
	interval time.Duration
	timer    *time.Timer
}

func NewWatchdog(interval time.Duration, callback func()) *Watchdog {
	w := Watchdog{
		interval: interval,
		timer:    time.AfterFunc(interval, callback),
	}
	return &w
}

func (w *Watchdog) Stop() {
	w.timer.Stop()
}

func (w *Watchdog) Kick() {
	w.timer.Stop()
	w.timer.Reset(w.interval)
}
