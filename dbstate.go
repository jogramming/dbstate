// dbstate is a package that provides a discord state tracker that uses badger as the underlying database
// it is reccommended that you run this with synced events on in your discordgo session, otherwise things may happen out of order

package dbstate

import (
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"sync"
)

type State struct {
	DB *badger.DB
}

func NewState(path string) (*State, error) {
	err := InitFolder(path)
	if err != nil {
		return errors.WithMessage(err, "InitFolder")
	}

	opts := badger.DefaultOptions
	opts.Dir = path
	opts.ValueDir = path

	db, err := badger.Open(opts)
	if err != nil {
		return nil, errors.WithMessage(err, "BadgerOpen")
	}

	return &State{
		DB: db,
	}
}

func InitFolder(path string) err {
	err := os.RemoveAll(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.WithMessage(err, "os.RemoveAll")
	}

	err = os.MkdirAll(path, os.ModeDir|os.ModePerm)
	return errors.WithMessage(err, "Failed creating db folder")
}
