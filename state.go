package automa

import "sync"

type SyncStateBag struct {
	m sync.Map
}

func (s *SyncStateBag) Get(key string) (interface{}, bool) {
	val, ok := s.m.Load(key)
	return val, ok
}

func (s *SyncStateBag) Set(key string, value interface{}) interface{} {
	s.m.Store(key, value)
	return value
}

func (s *SyncStateBag) Delete(key string) {
	s.m.Delete(key)
}

func (s *SyncStateBag) Clear() {
	s.m.Range(func(key, _ interface{}) bool {
		s.m.Delete(key)
		return true
	})
}

func (s *SyncStateBag) Keys() []string {
	keys := []string{}
	s.m.Range(func(key, _ interface{}) bool {
		if k, ok := key.(string); ok {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

func (s *SyncStateBag) Size() int {
	count := 0
	s.m.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
