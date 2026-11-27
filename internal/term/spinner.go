package term

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StderrIsTerminal reports whether stderr is an interactive terminal where a
// spinner can redraw one line. Windows consoles stay quiet.
func StderrIsTerminal(getenv func(string) string) bool {
	if runtime.GOOS == "windows" || getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// Spinner shows that a long task is alive: an animated frame, a label and the
// elapsed time, redrawn on one line of w. Label must be cleaned text.
type Spinner struct {
	w        io.Writer
	label    string
	interval time.Duration
	start    time.Time
	stop     chan struct{}
	done     sync.WaitGroup
}

// StartSpinner begins drawing on w and returns the spinner. Call Stop to erase it.
func StartSpinner(w io.Writer, label string) *Spinner {
	return startSpinner(w, label, 100*time.Millisecond)
}

func startSpinner(w io.Writer, label string, interval time.Duration) *Spinner {
	s := &Spinner{w: w, label: label, interval: interval, start: time.Now(), stop: make(chan struct{})}
	s.done.Add(1)
	go s.run()
	return s
}

func (s *Spinner) draw(i int) {
	fmt.Fprintf(s.w, "\r\x1b[K%s %s %s", frames[i%len(frames)], s.label, Elapsed(time.Since(s.start)))
}

func (s *Spinner) run() {
	defer s.done.Done()
	t := time.NewTicker(s.interval)
	defer t.Stop()
	s.draw(0)
	for i := 1; ; i++ {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.draw(i)
		}
	}
}

// Stop erases the spinner line and returns how long it ran. It is safe to call once.
func (s *Spinner) Stop() time.Duration {
	close(s.stop)
	s.done.Wait()
	fmt.Fprint(s.w, "\r\x1b[K")
	return time.Since(s.start)
}

// Elapsed formats a duration as m:ss (or h:mm:ss).
func Elapsed(d time.Duration) string {
	n := int(d.Round(time.Second) / time.Second)
	if n >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", n/3600, n%3600/60, n%60)
	}
	return fmt.Sprintf("%d:%02d", n/60, n%60)
}
