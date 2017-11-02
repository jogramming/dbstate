package dbstate

import (
	"bytes"
	"encoding/gob"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
)

// setKey is a helper to encode and set a get using the provided shards encoder and buffer
// If tx is nil, will create a new transaction
func (s *State) setKey(shardID int, tx *badger.Txn, key string, val interface{}) error {
	if tx == nil {
		return s.DB.Update(func(txn *badger.Txn) error {
			return s.setKey(shardID, txn, key, val)
		})
	}

	encoded, err := s.encodeData(shardID, val)
	if err != nil {
		return errors.WithMessage(err, "EncodeData")
	}

	err = tx.Set([]byte(key), encoded, 0)
	return err
}

// encodeData encodes the provided value using the provided shards buffer and encoder
func (s *State) encodeData(shardID int, val interface{}) ([]byte, error) {
	worker := s.shards[shardID]
	// Reusing encoders gave issues
	encoder := gob.NewEncoder(worker.buffer)

	err := encoder.Encode(val)
	if err != nil {
		worker.buffer.Reset()
		return nil, err
	}

	encoded := make([]byte, worker.buffer.Len())
	_, err = worker.buffer.Read(encoded)

	worker.buffer.Reset()
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

// GetKey is a helper for retrieving a key and decoding it into the destination
// If tx is nil, will create a new transaction
func (s *State) GetKey(tx *badger.Txn, key string, dest interface{}) error {
	if tx == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.GetKey(txn, key, dest)
		})
	}

	item, err := tx.Get([]byte(key))
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
