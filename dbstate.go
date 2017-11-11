// Package dbstate is a package that provides a discord state tracker using badger as the underlying store
// allowing for the state to grow beyond the avilable system memory
package dbstate

//go:generate go run "cmd/gen_iterators/gen_iterators.go" -out "iterators_gen.go"

import (
	"bytes"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
	"github.com/json-iterator/go"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	VersionMajor = 0
	VersionMinor = 1
	VersionPatch = 0

	// The database format version, different versions are incompatible with eachother
	// pre-v2 were string based keys
	// v2 introduced compact binary keys
	// v3 changed the format itself to json
	FormatVersion = 3
)

var (
	VersionString = fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)
)

var (
	// Used when the loaded db version differs from the one used in this package version
	ErrDifferentFormatVersion = errors.New("Trying to open database with different format version than the current one")
)

type State struct {
	DB *badger.DB

	opts *Options

	numShards int

	memoryState *memoryState

	// Reuse the buffers used for encoding values into state
	shards []*shardWorker

	// Filters multiple presence updates from the same users in the same moment (by e.g sharing multiple servers with the bot)
	presenceUpdateFilter *presenceUpdateFilter

	stopChan chan interface{}
}

type Logger interface {
	LogWarn(...interface{})
	LogError(...interface{})
	LogInfo(...interface{})
}

type shardWorker struct {
	State *State

	shardID int
	buffer  *bytes.Buffer
	encoder *jsoniter.Encoder
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

type Options struct {
	// Badger db options
	// If nil, then it will use the default badger db options
	// with path to /tmp/
	DBOpts *badger.Options

	// channelSyncMode: if this is set it will also spin up the event channel receivers per shard
	// set this is you're gonna use the channel sync mode (see State.HandleEvent*)
	UseChannelSyncMode bool

	// Wether to track messages and eventually expire them (ttl = time to live)
	TrackMessages bool
	MessageTTL    time.Duration

	TrackPresences bool
	TrackMembers   bool
	TrackRoles     bool
	TrackChannels  bool

	// Set to keep old messages in state from previous runs
	KeepOldMessagesOnStart bool

	// Custom logger to use, the state itself implements this so it defaults to state if nil
	Logger Logger
}

// ReommendedBadgerOptions returns the recommended options for badger
// to be used
// This is mostly TODO and requires more profiling
func RecommendedBadgerOptions(dir string) *badger.Options {
	opts := badger.DefaultOptions

	if dir == "" {
		dir = fmt.Sprintf(filepath.Join(os.TempDir(), fmt.Sprintf("dbstate_%d", time.Now().Unix())))
	}

	opts.Dir = dir
	opts.ValueDir = dir

	// Disable this for faster writes
	opts.SyncWrites = false

	return &opts
}

// NewState creates a new state tracker, or an error if something went wrong
func NewState(numShards int, options Options) (*State, error) {
	if options.DBOpts == nil {
		options.DBOpts = RecommendedBadgerOptions("")
	}

	err := initFolder(options.DBOpts.Dir)
	if err != nil {
		return nil, errors.WithMessage(err, "InitFolder")
	}

	db, err := badger.Open(*options.DBOpts)
	if err != nil {
		return nil, errors.WithMessage(err, "BadgerOpen")
	}

	if numShards < 1 {
		numShards = 1
	}

	shards := make([]*shardWorker, numShards)

	s := &State{
		DB:                   db,
		opts:                 &options,
		numShards:            numShards,
		shards:               shards,
		memoryState:          &memoryState{},
		presenceUpdateFilter: &presenceUpdateFilter{numShards: numShards},
		stopChan:             make(chan interface{}),
	}

	if options.Logger == nil {
		options.Logger = s
	}

	err = s.initDB()
	if err != nil {
		return nil, errors.WithMessage(err, "initDB")
	}

	go s.gcWorker()
	s.initWorkers(shards, options.UseChannelSyncMode)
	return s, nil
}

// Close shuts the tracker down, closing the DB aswell
func (s *State) Close() {
	close(s.stopChan)
	s.DB.Close()
}

func (s *State) gcWorker() {
	t1m := time.NewTicker(time.Minute)
	t1s := time.NewTicker(time.Second)
	for {
		select {
		case <-s.stopChan:
			t1m.Stop()
			t1s.Stop()
			return
		case <-t1s.C:
			s.presenceUpdateFilter.clear()
		case <-t1m.C:
			s.opts.Logger.LogInfo("starting badger gc")

			// Enabling this made all the keys suddenly stop working, i think i may be doing something wrong in this regard
			// db.PurgeOlderVersions()

			s.DB.RunValueLogGC(0.5)
			s.opts.Logger.LogInfo("Done with badger gc")
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

		workers[i].encoder = jsoniter.NewEncoder(workers[i].buffer)

		if run {
			go workers[i].run()
		}
	}
}

type MetaInfo struct {
	FormatVersion int
}

func (s *State) initDB() error {
	var meta *MetaInfo
	err := s.GetKey(nil, KeyMeta, &meta)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			meta = &MetaInfo{
				FormatVersion: FormatVersion,
			}
			return s.SetKey(nil, nil, nil, KeyMeta, meta)
		}

		return err
	}

	if meta.FormatVersion != FormatVersion {
		return ErrDifferentFormatVersion
	}

	err = s.flushOldDBData()
	return errors.WithMessage(err, "flushOldDBData")
}

func (s *State) flushOldDBData() error {
	// s.opts.Logger.LogInfo("Flushing old db data")

	// We have to use multiple transactions otherwise it gets too big
	done := false
	first := true
	delCount := 0
	for !done {
		curKey := []byte{}

		err := s.DB.Update(func(txn *badger.Txn) error {

			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			it := txn.NewIterator(opts)

			if first {
				it.Rewind()
				first = false
			} else {
				it.Seek(curKey)
			}

			i := 0
			for ; it.Valid(); it.Next() {
				if s.opts.KeepOldMessagesOnStart && it.ValidForPrefix([]byte{byte(KeyTypeChannelMessage)}) {
					// Keep old messages
					continue
				}

				if it.ValidForPrefix(KeyMeta) {
					// Meta
					continue
				}

				i++
				if i >= 100000 {
					return nil
				}
				delCount++
				item := it.Item()
				key := item.Key()
				curKey = make([]byte, len(key))
				copy(curKey, key)

				err := txn.Delete(curKey)
				if err != nil {
					return err
				}
			}

			done = true
			return nil
		})

		if err != nil {
			return err
		}

		// s.opts.Logger.LogInfo("Deleted: ", delCount, ", ", string(curKey[0]), curKey)
		delCount = 0
	}

	s.opts.Logger.LogInfo("Done Flushing old db data")

	return nil
}

func initFolder(path string) error {
	if path == "" {
		return errors.New("No path specified")
	}

	if path == "/" {
		return errors.New("Passed root fs as path, are you trying to break your system?")
	}

	err := os.MkdirAll(path, os.ModeDir|os.ModePerm)
	return errors.WithMessage(err, "Failed creating db folder")
}

// Standard implementation of the logger
func (s *State) LogWarn(m ...interface{}) {
	logrus.Warn(append([]interface{}{"[dbstate]: "}, m...)...)
}
func (s *State) LogError(m ...interface{}) {
	logrus.Error(append([]interface{}{"[dbstate]: "}, m...)...)
}
func (s *State) LogInfo(m ...interface{}) {
	logrus.Info(append([]interface{}{"[dbstate]: "}, m...)...)
}
