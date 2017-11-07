// Package dbstate is a package that provides a discord state tracker using badger as the underlying store
// allowing for the state to grow beyond the avilable system memory
package dbstate

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type State struct {
	DB *badger.DB

	// Configuration options, set these before starting to feed it with events
	// unless you want race conditions
	MessageTTL     time.Duration
	TrackPresences bool
	TrackMessages  bool

	numShards int

	memoryState *memoryState

	// Reuse the buffers used for encoding values into state
	shards []*shardWorker

	// Filters multiple presence updates from the same users in the same moment (by e.g sharing multiple servers with the bot)
	presenceUpdateFilter *presenceUpdateFilter
}

type shardWorker struct {
	State *State

	shardID int
	buffer  *bytes.Buffer
	encoder *gob.Encoder
	working bool

	// Used for the channel sync mode
	eventChan chan interface{}

	// Used for the mutex sync mode
	mu *sync.Mutex
}

// Small in memory state that holds a small amount of information
type memoryState struct {
	sync.RWMutex

	User *discordgo.User
}

// Creates a new state in path folder
// Warning: path will be deleted to flush existing state
// Pass empty path to use "/tmp/dbstate_currentunixtime"
//
// channelSyncMode: if this is set it will also spin up the event channel receivers per shard
// set this is you're gonna use the channel sync mode (see State.HandleEvent*)
func NewState(path string, numShards int, channelSyncMode bool) (*State, error) {
	if path == "" {
		path = fmt.Sprintf(filepath.Join(os.TempDir(), fmt.Sprintf("dbstate_%d", time.Now().Unix())))
	}

	err := initFolder(path)
	if err != nil {
		return nil, errors.WithMessage(err, "InitFolder")
	}

	opts := badger.DefaultOptions
	opts.Dir = path
	opts.ValueDir = path
	opts.SyncWrites = false

	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.WithMessage(err, "BadgerOpen")
	}

	if numShards < 1 {
		numShards = 1
	}

	shards := make([]*shardWorker, numShards)

	s := &State{
		DB:                   db,
		numShards:            numShards,
		shards:               shards,
		memoryState:          &memoryState{},
		MessageTTL:           time.Hour,
		presenceUpdateFilter: &presenceUpdateFilter{numShards: numShards},
	}

	go s.gcWorker()
	s.initWorkers(shards, channelSyncMode)
	return s, nil
}

func (s *State) gcWorker() {
	t1m := time.NewTicker(time.Minute)
	t1s := time.NewTicker(time.Second)
	for {
		select {
		case <-t1s.C:
			s.presenceUpdateFilter.clear()
		case <-t1m.C:
			logrus.Info("starting badger gc")

			// Enabling this made all the keys suddenly stop working, i think i may be doing something wrong in this regard
			// db.PurgeOlderVersions()

			s.DB.RunValueLogGC(0.5)
			logrus.Info("Done with badger gc")
		}
	}
}

func (s *State) initWorkers(workers []*shardWorker, run bool) {
	for i, _ := range workers {
		workers[i] = &shardWorker{
			State:     s,
			shardID:   i,
			buffer:    new(bytes.Buffer),
			eventChan: make(chan interface{}, 10),
		}

		workers[i].encoder = gob.NewEncoder(workers[i].buffer)

		if run {
			go workers[i].run()
		}
	}
}

func initFolder(path string) error {
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
	} else if !os.IsNotExist(err) {
		return err
	}

	err := os.MkdirAll(path, os.ModeDir|os.ModePerm)
	return errors.WithMessage(err, "Failed creating db folder")
}

func flushExistingState(path string) error {
	if _, err := os.Stat(path + "_tmp"); err == nil {
		return errors.New(path + "_tmp folder already exists")
	}

	// Rename it temporarily because removing isnt instant sometimes on windows?
	// and this needs to be instant.
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

// rmDir recursively deletes a directory and all it's contents
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
