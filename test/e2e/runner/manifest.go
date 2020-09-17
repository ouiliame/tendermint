package main

import (
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

// Manifest represents a testnet manifest, specified as TOML.
type Manifest struct {
	Name             string
	IP               string
	InitialHeight    uint64                      `toml:"initial_height"`
	InitialState     map[string]string           `toml:"initial_state"`
	ValidatorUpdates map[string]map[string]uint8 `toml:"validator_update"`
	Nodes            map[string]ManifestNode     `toml:"node"`
}

// ManifestNode represents a testnet manifest node.
type ManifestNode struct {
	IP              string
	ProxyPort       uint32
	StartAt         uint64 `toml:"start_at"`
	FastSync        string `toml:"fast_sync"`
	Database        string
	ABCIProtocol    string  `toml:"abci_protocol"`
	PersistInterval *uint64 `toml:"persist_interval"`
	RetainBlocks    uint64  `toml:"retain_blocks"`
	PrivvalProtocol string  `toml:"privval_protocol"`
}

// ParseManifest parses a testnet manifest from TOML.
func ParseManifest(r io.Reader) (Manifest, error) {
	manifest := Manifest{}
	_, err := toml.DecodeReader(r, &manifest)
	return manifest, err
}

// LoadManifest loads a testnet manifest from a file.
func LoadManifest(file string) (Manifest, error) {
	r, err := os.Open(file)
	if err != nil {
		return Manifest{}, fmt.Errorf("failed to open testnet manifest %q: %w", file, err)
	}
	return ParseManifest(r)
}
