package dbstate

import (
	"bytes"
	"encoding/gob"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"sync"
	"time"
)

// setKey is a helper to encode and set a get using the provided shards encoder and buffer
// If tx is nil, will create a new transaction
func (w *shardWorker) setKey(tx *badger.Txn, key []byte, val interface{}) error {
	return w.setKeyWithTTL(tx, key, val, -1)
}

func (w *shardWorker) setKeyWithTTL(tx *badger.Txn, key []byte, val interface{}, ttl time.Duration) error {
	if tx == nil {
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.setKey(txn, key, val)
		})
	}

	encoded, err := w.encodeData(val)
	if err != nil {
		return errors.WithMessage(err, "EncodeData")
	}

	if ttl > 0 {
		err = tx.SetWithTTL(key, encoded, ttl)
	} else {
		err = tx.Set(key, encoded)
	}
	return err
}

// encodeData encodes the provided value using the provided shards buffer and encoder
func (w *shardWorker) encodeData(val interface{}) ([]byte, error) {
	// Reusing encoders gave issues
	encoder := gob.NewEncoder(w.buffer)
	// encoder := w.encoder

	err := encoder.Encode(val)
	if err != nil {
		w.buffer.Reset()
		return nil, err
	}

	encoded := make([]byte, w.buffer.Len())
	_, err = w.buffer.Read(encoded)

	w.buffer.Reset()
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

// GetKey is a helper for retrieving a key and decoding it into the destination
// If tx is nil, will create a new transaction
func (s *State) GetKey(tx *badger.Txn, key []byte, dest interface{}) error {
	if tx == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.GetKey(txn, key, dest)
		})
	}

	item, err := tx.Get(key)
	if err != nil {
		return err
	}
	v, err := item.Value()
	if err != nil {
		return err
	}

	return s.DecodeData(v, dest)
}

// DecodeData is a helper for deocding data
func (s *State) DecodeData(data []byte, dest interface{}) error {
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(dest)
	return err
}

// RetryUpdate will run s.DB.Update with `fn` and re-run it if ErrConflict is returned, until no there are no conflicts
// therefor `fn` may be called multiple times, but each time there is a conflict writes are thrown away
func (s *State) RetryUpdate(fn func(txn *badger.Txn) error) error {
	for {
		err := s.DB.Update(fn)
		if err == nil {
			return nil
		}

		if err == badger.ErrConflict {
			s.Logger.LogWarn("Transaction conflict, retrying...")
			time.Sleep(time.Millisecond)
			continue
		}

		return err
	}
}

type presenceUpdateFilter struct {
	recentlyProcessedPresenceUpdates [][]string
	numShards                        int
	mu                               sync.RWMutex
}

func (f *presenceUpdateFilter) clear() {
	f.mu.Lock()
	f.recentlyProcessedPresenceUpdates = make([][]string, f.numShards)
	f.mu.Unlock()
}

// Not safe to be ran by multiple goroutines using the same shard
// Multiple goroutines with different shards is fine
func (f *presenceUpdateFilter) checkUserID(shardID int, userID string) (checkedRecently bool) {
	f.mu.RLock()
	for _, shardList := range f.recentlyProcessedPresenceUpdates {
		for _, v := range shardList {
			if v == userID {
				f.mu.RUnlock()
				return true
			}
		}
	}

	f.recentlyProcessedPresenceUpdates[shardID] = append(f.recentlyProcessedPresenceUpdates[shardID], userID)
	f.mu.RUnlock()
	return false
}
