package gui

import (
	"strings"
	"sync"
)

const logLimit = 200_000

type logBuffer struct {
	mu     sync.Mutex
	text   []byte
	onLine func(string)
}

func (b *logBuffer) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\r\n") + "\n"
	b.mu.Lock()
	b.text = append(b.text, line...)
	if len(b.text) > logLimit {
		cut := len(b.text) - logLimit*3/4
		if i := strings.IndexByte(string(b.text[cut:]), '\n'); i >= 0 {
			cut += i + 1
		}
		b.text = append([]byte(nil), b.text[cut:]...)
	}
	onLine := b.onLine
	b.mu.Unlock()
	if onLine != nil {
		onLine(line)
	}
	return len(p), nil
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.text)
}

func (b *logBuffer) follow(onLine func(string)) {
	b.mu.Lock()
	b.onLine = onLine
	b.mu.Unlock()
}
