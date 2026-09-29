package logfile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

const (
	Name       = "awlauncher.log"
	MaxSize    = 5 << 20
	KeepOld    = 9
	timeLayout = "2006-01-02 15:04:05.000"
)

type File struct {
	mu   sync.Mutex
	dir  string
	f    *os.File
	size int64
}

func Open(dir string) (*File, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	l := &File{dir: dir}
	l.salvageCrash()
	if err := l.rotate(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *File) Dir() string { return l.dir }

func (l *File) Path() string { return filepath.Join(l.dir, Name) }

func (l *File) Write(p []byte) (int, error) {
	text := strings.TrimRight(string(p), "\r\n")
	stamp := time.Now().Format(timeLayout)
	var b strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		b.WriteString(stamp)
		b.WriteByte(' ')
		b.WriteString(line)
		b.WriteString("\r\n")
	}
	if b.Len() == 0 {
		return len(p), nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return len(p), nil
	}
	if l.size > 0 && l.size+int64(b.Len()) > MaxSize {
		if l.rotate() != nil {
			return len(p), nil
		}
	}
	n, _ := l.f.WriteString(b.String())
	l.size += int64(n)
	return len(p), nil
}

func (l *File) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

func (l *File) rotate() error {
	if l.f != nil {
		l.f.Close()
		l.f = nil
	}
	current := l.Path()
	if info, err := os.Stat(current); err == nil && info.Size() > 0 {
		os.Remove(l.old(KeepOld))
		for i := KeepOld - 1; i >= 1; i-- {
			os.Rename(l.old(i), l.old(i+1))
		}
		os.Rename(current, l.old(1))
	}
	f, err := os.OpenFile(current, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	l.f, l.size = f, 0
	if info, err := f.Stat(); err == nil && info.Size() < MaxSize {
		l.size = info.Size()
	}
	return nil
}

func (l *File) CatchCrashes() error {
	f, err := os.Create(l.crashPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return debug.SetCrashOutput(f, debug.CrashOptions{})
}

func (l *File) salvageCrash() {
	crash, err := os.ReadFile(l.crashPath())
	if err != nil || len(strings.TrimSpace(string(crash))) == 0 {
		return
	}
	f, err := os.OpenFile(l.Path(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	_, err = f.WriteString("---- crash ----\r\n" + strings.ReplaceAll(strings.ReplaceAll(string(crash), "\r\n", "\n"), "\n", "\r\n"))
	if f.Close() == nil && err == nil {
		os.Remove(l.crashPath())
	}
}

func (l *File) crashPath() string { return filepath.Join(l.dir, "crash.log") }

func (l *File) old(i int) string {
	return filepath.Join(l.dir, fmt.Sprintf("awlauncher.%d.log", i))
}
