//nolint: gosec
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"
)

// State is the application state.
type State struct {
	sync.RWMutex
	Height   uint64
	Values   map[string]string
	Hash     []byte
	Requests struct {
		InitChain abci.RequestInitChain
		CheckTx   []abci.RequestCheckTx
		DeliverTx []abci.RequestDeliverTx
	}

	file            string
	persistInterval uint64
}

// NewState creates a new state.
func NewState(file string, persistInterval uint64) (*State, error) {
	state := &State{
		Values:          make(map[string]string, 1024),
		file:            file,
		persistInterval: persistInterval,
	}
	state.Hash = state.hashValues()
	err := state.Load()
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	}
	return state, nil
}

// Import imports key/value pairs from JSON bytes, used for InitChain.AppStateBytes.
func (s *State) Import(jsonBytes []byte) error {
	values := map[string]string{}
	err := json.Unmarshal(jsonBytes, &values)
	if err != nil {
		return fmt.Errorf("failed to decode imported JSON data: %w", err)
	}
	s.Values = values
	s.Hash = s.hashValues()
	return nil
}

// Load loads state from disk.
func (s *State) Load() error {
	bz, err := ioutil.ReadFile(s.file)
	if err != nil {
		return fmt.Errorf("failed to read state from %q: %w", s.file, err)
	}
	err = json.Unmarshal(bz, s)
	if err != nil {
		return fmt.Errorf("invalid state data in %q: %w", s.file, err)
	}
	return nil
}

// Save saves the state to disk.
func (s *State) Save() error {
	bz, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	err = ioutil.WriteFile(s.file, bz, 0644)
	if err != nil {
		return fmt.Errorf("failed to write state to %q: %w", s.file, err)
	}
	return nil
}

// Get fetches a value. A missing value is returned as an empty string.
func (s *State) Get(key string) string {
	s.RLock()
	defer s.RUnlock()
	return s.Values[key]
}

// Set sets a value. Setting an empty value is equivalent to deleting it.
func (s *State) Set(key, value string) {
	s.Lock()
	defer s.Unlock()
	if value == "" {
		delete(s.Values, key)
	} else {
		s.Values[key] = value
	}
}

// Commit commits the current state.
func (s *State) Commit(flush bool) (uint64, []byte, error) {
	s.Lock()
	defer s.Unlock()
	s.Hash = s.hashValues()
	switch {
	case s.Height > 0:
		s.Height++
	case s.Requests.InitChain.InitialHeight > 0:
		s.Height = uint64(s.Requests.InitChain.InitialHeight)
	default:
		s.Height = 1
	}
	if s.persistInterval > 0 && s.Height%s.persistInterval == 0 {
		err := s.Save()
		if err != nil {
			return 0, nil, err
		}
	}
	return s.Height, s.Hash, nil
}

// hashValues hashes the current value set.
func (s *State) hashValues() []byte {
	keys := make([]string, 0, len(s.Values))
	for key := range s.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hasher := sha256.New()
	for _, key := range keys {
		_, _ = hasher.Write([]byte(key))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write([]byte(s.Values[key]))
		_, _ = hasher.Write([]byte{0})
	}
	return hasher.Sum(nil)
}
