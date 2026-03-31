package concurrency

import "sync"

type SyncRunner struct {
	sync.Mutex
}

func (s *SyncRunner) Go(f func() error) error {
	s.Lock()
	defer s.Unlock()
	return f()
}

type MultiSyncRunner struct {
	locks sync.Map
}

func (s *MultiSyncRunner) Go(key string, f func() error) error {
	var keyLock sync.Mutex
	l, _ := s.locks.LoadOrStore(key, &keyLock)
	lock := l.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	return f()
}
