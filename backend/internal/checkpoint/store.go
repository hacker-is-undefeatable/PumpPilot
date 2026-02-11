package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	path string
	mu   sync.Mutex
	last uint64
}

type state struct {
	LastProcessedBlock uint64 `json:"last_processed_block"`
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Load() (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var st state
	if err := json.Unmarshal(b, &st); err != nil {
		return 0, err
	}
	s.last = st.LastProcessedBlock
	return s.last, nil
}

func (s *Store) Save(last uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	st := state{LastProcessedBlock: last}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("checkpoint rename: %w", err)
	}
	s.last = last
	return nil
}

func (s *Store) Last() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}
