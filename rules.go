package progszy

import (
	"regexp"
	"sync"
)

// TODO(js) This should probably have its own tests.

type rulesMap struct {
	mu              sync.RWMutex
	regexpByPattern map[string]*regexp.Regexp
}

func newRulesMap() *rulesMap {
	m := rulesMap{
		regexpByPattern: make(map[string]*regexp.Regexp),
	}
	return &m
}

func (m *rulesMap) get(pat string) (*regexp.Regexp, error) {
	m.mu.RLock()
	re, ok := m.regexpByPattern[pat]
	m.mu.RUnlock()
	if ok {
		return re, nil
	}
	return m.put(pat)
}

func (m *rulesMap) getAll(pats []string) ([]*regexp.Regexp, error) {
	relist := make([]*regexp.Regexp, len(pats))
	for i, pat := range pats {
		re, err := m.get(pat)
		if err != nil {
			return nil, err
		}
		relist[i] = re
	}
	return relist, nil
}

func (m *rulesMap) put(pat string) (*regexp.Regexp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	re, ok := m.regexpByPattern[pat]
	if ok {
		// Already exists, nothing to do.
		return re, nil
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, err
	}
	m.regexpByPattern[pat] = re
	return re, nil
}
