package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/tendermint/tendermint/abci/example/code"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/version"
)

var _ abci.Application = (*Application)(nil)

// Application is an ABCI application for use by end-to-end tests. It is a
// simple key/value store for strings, storing data in memory and persisting
// to disk as JSON.
type Application struct {
	abci.BaseApplication
	logger log.Logger
	state  *State
	cfg    *Config
}

func NewApplication(cfg *Config) (*Application, error) {
	state, err := NewState(cfg.File, cfg.PersistInterval)
	if err != nil {
		return nil, err
	}
	return &Application{
		logger: log.NewTMLogger(log.NewSyncWriter(os.Stdout)),
		state:  state,
		cfg:    cfg,
	}, nil
}

func (app *Application) Info(req abci.RequestInfo) abci.ResponseInfo {
	return abci.ResponseInfo{
		Version:          version.ABCIVersion,
		AppVersion:       1,
		LastBlockHeight:  int64(app.state.Height),
		LastBlockAppHash: app.state.Hash,
	}
}

func (app *Application) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	var err error
	app.state.Requests.InitChain = req
	if len(req.AppStateBytes) > 0 {
		err = app.state.Import(req.AppStateBytes)
		if err != nil {
			panic(err)
		}
	}
	resp := abci.ResponseInitChain{
		AppHash: app.state.Hash,
	}
	if resp.Validators, err = app.validatorUpdates(0); err != nil {
		panic(err)
	}
	return resp
}

func (app *Application) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	app.state.Requests.CheckTx = append(app.state.Requests.CheckTx, req)
	_, _, err := parseTx(req.Tx)
	if err != nil {
		return abci.ResponseCheckTx{
			Code: code.CodeTypeEncodingError,
			Log:  err.Error(),
		}
	}
	return abci.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}
}

func (app *Application) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	app.state.Requests.DeliverTx = append(app.state.Requests.DeliverTx, req)
	key, value, err := parseTx(req.Tx)
	if err != nil {
		panic(err) // shouldn't happen since we verified it in CheckTx
	}
	app.state.Set(key, value)
	return abci.ResponseDeliverTx{Code: code.CodeTypeOK}
}

func (app *Application) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	var err error
	resp := abci.ResponseEndBlock{}
	if resp.ValidatorUpdates, err = app.validatorUpdates(uint64(req.Height)); err != nil {
		panic(err)
	}
	return resp
}

func (app *Application) Commit() abci.ResponseCommit {
	height, hash, err := app.state.Commit(true)
	if err != nil {
		panic(err)
	}
	retainHeight := int64(0)
	if app.cfg.RetainBlocks > 0 {
		retainHeight = int64(height - app.cfg.RetainBlocks + 1)
	}
	return abci.ResponseCommit{
		Data:         hash,
		RetainHeight: retainHeight,
	}
}

func (app *Application) Query(req abci.RequestQuery) abci.ResponseQuery {
	return abci.ResponseQuery{
		Height: int64(app.state.Height),
		Key:    req.Data,
		Value:  []byte(app.state.Get(string(req.Data))),
	}
}

// validatorUpdates generates a validator set update.
func (app *Application) validatorUpdates(height uint64) (abci.ValidatorUpdates, error) {
	updates := app.cfg.ValidatorUpdates[fmt.Sprintf("%v", height)]
	if len(updates) == 0 {
		return nil, nil
	}
	valUpdates := abci.ValidatorUpdates{}
	for keyString, power := range updates {
		keyBytes, err := base64.StdEncoding.DecodeString(keyString)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 pubkey value %q: %w", keyString, err)
		}
		valUpdates = append(valUpdates, abci.Ed25519ValidatorUpdate(keyBytes, int64(power)))
	}
	return valUpdates, nil
}

// parseTx parses a tx in 'key=value' format into a key and value. Keys cannot start with _.
func parseTx(tx []byte) (string, string, error) {
	parts := bytes.Split(tx, []byte("="))
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid tx format: %q", string(tx))
	}
	if len(parts[0]) == 0 {
		return "", "", errors.New("key cannot be empty")
	}
	if parts[0][0] == '_' {
		return "", "", errors.New("keys cannot start with _")
	}
	return string(parts[0]), string(parts[1]), nil
}

/*// ListSnapshots implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) ListSnapshots(req abci.RequestListSnapshots) abci.ResponseListSnapshots {
	resp := abci.ResponseListSnapshots{Snapshots: []*abci.Snapshot{}}
	if app.snapshotManager == nil {
		return resp
	}

	snapshots, err := app.snapshotManager.List()
	if err != nil {
		app.logger.Error("Failed to list snapshots", "err", err)
		return resp
	}
	for _, snapshot := range snapshots {
		abciSnapshot, err := snapshot.ToABCI()
		if err != nil {
			app.logger.Error("Failed to list snapshots", "err", err)
			return resp
		}
		resp.Snapshots = append(resp.Snapshots, &abciSnapshot)
	}

	return resp
}

// LoadSnapshotChunk implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) LoadSnapshotChunk(req abci.RequestLoadSnapshotChunk) abci.ResponseLoadSnapshotChunk {
	if app.snapshotManager == nil {
		return abci.ResponseLoadSnapshotChunk{}
	}
	chunk, err := app.snapshotManager.LoadChunk(req.Height, req.Format, req.Chunk)
	if err != nil {
		app.logger.Error("Failed to load snapshot chunk", "height", req.Height, "format", req.Format,
			"chunk", req.Chunk, "err")
		return abci.ResponseLoadSnapshotChunk{}
	}
	return abci.ResponseLoadSnapshotChunk{Chunk: chunk}
}

// OfferSnapshot implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) OfferSnapshot(req abci.RequestOfferSnapshot) abci.ResponseOfferSnapshot {
	if req.Snapshot == nil {
		app.logger.Error("Received nil snapshot")
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}
	}

	snapshot, err := snapshottypes.SnapshotFromABCI(req.Snapshot)
	if err != nil {
		app.logger.Error("Failed to decode snapshot metadata", "err", err)
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}
	}
	err = app.snapshotManager.Restore(snapshot)
	switch {
	case err == nil:
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ACCEPT}

	case errors.Is(err, snapshottypes.ErrUnknownFormat):
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT_FORMAT}

	case errors.Is(err, snapshottypes.ErrInvalidMetadata):
		app.logger.Error("Rejecting invalid snapshot", "height", req.Snapshot.Height,
			"format", req.Snapshot.Format, "err", err)
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}

	default:
		app.logger.Error("Failed to restore snapshot", "height", req.Snapshot.Height,
			"format", req.Snapshot.Format, "err", err)
		// We currently don't support resetting the IAVL stores and retrying a different snapshot,
		// so we ask Tendermint to abort all snapshot restoration.
		return abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_ABORT}
	}
}

// ApplySnapshotChunk implements the ABCI interface. It delegates to app.snapshotManager if set.
func (app *Application) ApplySnapshotChunk(req abci.RequestApplySnapshotChunk) abci.ResponseApplySnapshotChunk {
	_, err := app.snapshotManager.RestoreChunk(req.Chunk)
	switch {
	case err == nil:
		return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ACCEPT}

	case errors.Is(err, snapshottypes.ErrChunkHashMismatch):
		app.logger.Error("Chunk checksum mismatch, rejecting sender and requesting refetch",
			"chunk", req.Index, "sender", req.Sender, "err", err)
		return abci.ResponseApplySnapshotChunk{
			Result:        abci.ResponseApplySnapshotChunk_RETRY,
			RefetchChunks: []uint32{req.Index},
			RejectSenders: []string{req.Sender},
		}

	default:
		app.logger.Error("Failed to restore snapshot", "err", err)
		return abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ABORT}
	}
}
*/
