package cli

import (
	"fmt"
	"time"

	"fortio.org/progressbar"
)

const (
	prefixWidth = 20
)

type Progresser interface {
	Done()
	DoWork()
	Max(int)
	Next()
	StopWork()
	Text(string)
}

type BarProgresser struct {
	bar     *progressbar.Bar
	max     int
	current int
	text    string
	doWork  bool
}

type NoopProgresser struct {
}

func NewProgress(max int, initalText string, visible bool) Progresser {
	if !visible {
		return NoopProgresser{}
	}
	cfg := progressbar.DefaultConfig()
	cfg.UpdateInterval = 100 * time.Millisecond
	p := &BarProgresser{
		bar: cfg.NewBar(),
		max: max,
	}
	if initalText != "" {
		p.Text(initalText)
		p.bar.Progress(0)
	}
	return p
}

// Next will increase the progress by one increment until max is reached.
func (p *BarProgresser) Next() {
	if p.current < p.max {
		p.current += 1
	}
	p.bar.Progress(100. * float64(p.current) / float64(p.max))
}

func (p *BarProgresser) prefix() string {
	return rightPad(p.text)
}

func rightPad(s string) string {
	if len(s) > prefixWidth {
		s = s[:prefixWidth]
	}
	return fmt.Sprintf("%-*s ", prefixWidth, s)
}

// Done marks the progress as done.
func (p *BarProgresser) Done() {
	p.bar.End()
}

// Max sets the maximum value. During execution calls to Next will increase
// the progress until max is reached.
func (p *BarProgresser) Max(max int) {
	p.max = max
}

// Update the prefix text. Text will be truncated to 20 characters.
func (p *BarProgresser) Text(text string) {
	p.text = text
	p.bar.UpdatePrefix(p.prefix())
}

// DoWork will emulate work being done. Run in a go routine to make the spinner
// spin. Call StopWork to make it stop. Useful when running jobs that don't
// have a max value, for example starting a process to find out what our max is.
func (p *BarProgresser) DoWork() {
	p.doWork = true
	for {
		p.Next()
		if !p.doWork {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// StopWork will stop endless spinner started by DoWork.
func (p *BarProgresser) StopWork() {
	p.doWork = false
	time.Sleep(200 * time.Millisecond)
}

func (p NoopProgresser) Done()       {}
func (p NoopProgresser) DoWork()     {}
func (p NoopProgresser) Max(int)     {}
func (p NoopProgresser) Next()       {}
func (p NoopProgresser) StopWork()   {}
func (p NoopProgresser) Text(string) {}
