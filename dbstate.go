// dbstate is a package that provides a discord state tracker that uses badger as the underlying database
// it is reccommended that you run this with synced events on in your discordgo session, otherwise things may happen out of order

package dbstate

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	DB *badger.DB

	numShards int

	// Reuse the buffers used for encoding values into state
	shards []*shardWorker
}

type shardWorker struct {
	shardID int
	buffer  *bytes.Buffer
	encoder *gob.Encoder
}

// Creates a new state in path folder
// Warning: path will be deleted to flush existing state
// Pass empty path to use "/tmp/dbstate_currentunixtime"
func NewState(path string, numShards int) (*State, error) {
	if path == "" {
		path = fmt.Sprintf(filepath.Join(os.TempDir(), fmt.Sprintf("dbstate_%d", time.Now().Unix())))
	}

	err := InitFolder(path)
	if err != nil {
		return nil, errors.WithMessage(err, "InitFolder")
	}

	opts := badger.DefaultOptions
	opts.Dir = path
	opts.ValueDir = path

	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.WithMessage(err, "BadgerOpen")
	}

	if numShards < 1 {
		numShards = 1
	}

	shards := make([]*shardWorker, numShards)
	initWorkers(shards)
	go gcWorker(db)
	return &State{
		DB:        db,
		numShards: numShards,
		shards:    shards,
	}, nil
}

func gcWorker(db *badger.DB) {
	for {
		db.PurgeOlderVersions()
		db.RunValueLogGC(0.5)
		time.Sleep(time.Minute)
	}
}

func initWorkers(workers []*shardWorker) {
	for i, _ := range workers {
		workers[i] = &shardWorker{
			shardID: i,
			buffer:  new(bytes.Buffer),
		}

		workers[i].encoder = gob.NewEncoder(workers[i].buffer)
	}
}

func InitFolder(path string) error {
	if path == "" {
		return errors.New("No path specified")
	}

	if path == "/" {
		return errors.New("Passed root fs as path, are you trying to break your system?")
	}

	if _, err := os.Stat(path); err == nil {
		err := flushExistingState(path)
		if err != nil {
			return err
		}
	}

	err := rmDir(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.WithMessage(err, "os.RemoveAll")
	}

	err = os.MkdirAll(path, os.ModeDir|os.ModePerm)
	return errors.WithMessage(err, "Failed creating db folder")
}

func flushExistingState(path string) error {
	if _, err := os.Stat(path + "_tmp"); err == nil {
		return errors.New(path + "_tmp folder already exists")
	}

	// Rename it temporarily because removing isnt instant sometimes on windows?
	err := os.Rename(path, path+"_tmp")
	if err != nil {
		return err
	}

	err = rmDir(path + "_tmp")
	if err != nil {
		return err
	}

	return os.Remove(path + "_tmp")
}

func rmDir(path string) error {
	dir, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, f := range dir {
		fPath := filepath.Join(path, f.Name())

		if f.IsDir() {
			err := rmDir(fPath)
			if err != nil {
				return err
			}
		}

		err := os.Remove(fPath)
		if err != nil {
			return err
		}
	}

	return nil
}
