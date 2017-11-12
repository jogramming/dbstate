package dbstate

import (
	"bytes"
	"github.com/dgraph-io/badger"
	"github.com/json-iterator/go"
	"github.com/pkg/errors"
	"sync"
	"time"
)

type MessageFlag byte

const (
	MessageFlagDeleted MessageFlag = 1 << iota
	MessageFlagOld
)

// setKey is a helper to encode and set a get using the provided shards encoder and buffer
// If tx is nil, will create a new transaction
func (w *shardWorker) setKey(tx *badger.Txn, key []byte, val interface{}) error {
	return w.State.SetKey(tx, w.buffer, w.encoder, key, val)
}

func (w *shardWorker) setKeyWithTTL(tx *badger.Txn, key []byte, val interface{}, ttl time.Duration) error {
	return w.State.SetKeyWithTTL(tx, w.buffer, w.encoder, key, val, ttl)
}

func (s *State) SetKey(tx *badger.Txn, buffer *bytes.Buffer, encoder *jsoniter.Encoder, key []byte, val interface{}) error {
	return s.SetKeyWithTTL(tx, buffer, encoder, key, val, -1)
}

func (s *State) SetKeyWithTTL(tx *badger.Txn, buffer *bytes.Buffer, encoder *jsoniter.Encoder, key []byte, val interface{}, ttl time.Duration) error {
	if tx == nil {
		return s.RetryUpdate(func(txn *badger.Txn) error {
			return s.SetKeyWithTTL(txn, buffer, encoder, key, val, ttl)
		})
	}

	encoded, err := s.encodeData(buffer, encoder, val)
	if err != nil {
		return errors.WithMessage(err, "EncodeData")
	}

	if ttl > 0 {
		err = tx.SetWithTTL(key, encoded, ttl)
	} else {
		err = tx.Set(key, encoded)
	}

	if buffer != nil {
		buffer.Reset()
	}

	return err
}

// encodeData encodes the provided value using the provided shards buffer and encoder
// the returned byte slice is only valid until the next modification of buffer
func (s *State) encodeData(buffer *bytes.Buffer, enc *jsoniter.Encoder, val interface{}) ([]byte, error) {
	if buffer == nil || enc == nil {
		buffer = new(bytes.Buffer)
		enc = jsoniter.NewEncoder(buffer)
	}

	err := enc.Encode(val)
	if err != nil {
		buffer.Reset()
		return nil, err
	}

	cop := make([]byte, buffer.Len())
	_, err = buffer.Read(cop)
	buffer.Reset()

	// encoded := buffer.Bytes()
	return cop, err
}

// GetKey is a helper for retrieving a key and decoding it into the destination
// If tx is nil, will create a new transaction
func (s *State) GetKey(txn *badger.Txn, key []byte, dest interface{}) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.GetKey(txn, key, dest)
		})
	}

	item, err := txn.Get(key)
	if err != nil {
		return err
	}

	buf := make([]byte, item.EstimatedSize())

	v, err := item.ValueCopy(buf)
	if err != nil {
		return err
	}

	return s.DecodeData(v, dest)
}

// GetKeyWithBuffer is the same as GetKey but allows you to reuse the buffer
// The buffer may need to grow, in which case it will return a new one
func (s *State) GetKeyWithBuffer(txn *badger.Txn, key []byte, buffer []byte, dest interface{}) ([]byte, error) {
	if txn == nil {
		err := s.DB.View(func(txn *badger.Txn) error {
			var err error
			buffer, err = s.GetKeyWithBuffer(txn, key, buffer, dest)
			return err
		})

		return buffer, err
	}

	item, err := txn.Get(key)
	if err != nil {
		return buffer, err
	}

	v, err := item.ValueCopy(buffer)
	if err != nil {
		return buffer, err
	}

	err = s.DecodeData(v, dest)
	return v, err
}

// DecodeData is a helper for deocding data
func (s *State) DecodeData(data []byte, dest interface{}) error {
	err := jsoniter.Unmarshal(data, dest)
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
			s.opts.Logger.LogWarn("Transaction conflict, retrying...")
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
	f.mu.RUnlock()

	// Upgrade the lock
	f.mu.Lock()
	f.recentlyProcessedPresenceUpdates[shardID] = append(f.recentlyProcessedPresenceUpdates[shardID], userID)
	f.mu.Unlock()
	return false
}
