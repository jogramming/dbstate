package dbstate

import (
	"bytes"
	"encoding/gob"
	"github.com/dgraph-io/badger"
)

func SetKey(tx *badger.Txn, key string, val interface{}, buf *bytes.Buffer, encoder *gob.Encoder) error {
	if buf == nil {
		buf = new(bytes.Buffer)
	}

	if encoder == nil {
		encoder = gob.NewEncoder(buf)
	}

	err := encoder.Encode(val)
	if err != nil {
		buf.Reset()
		return err
	}

	encoded := buf.Bytes()
	err = tx.Set([]byte(key), encoded, 0)
}
