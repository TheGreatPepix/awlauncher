package workers

import (
	"sync"
	"sync/atomic"
)

func Each[T any](items []T, jobs int, fn func(T) error) error {
	if jobs < 1 {
		jobs = 1
	}
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	var once sync.Once
	var failed atomic.Bool
	var first error
	for _, it := range items {
		sem <- struct{}{}
		if failed.Load() {
			<-sem
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(it); err != nil {
				once.Do(func() { first = err })
				failed.Store(true)
			}
		}()
	}
	wg.Wait()
	return first
}
