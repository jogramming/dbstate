package dbstate

import (
	"bytes"
	"encoding/gob"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
)

func (s *State) HandleEvent(session *discordgo.Session, eventInterface interface{}) {
	if ok, _ := eventInterface.(*discordgo.Event); ok {
		// Fast path this since this is sent for every single event
		return
	}

	var err error

	switch event := eventInterface.(type) {
	case *discordgo.Ready:
		if !session.SyncEvents {
			logrus.Warn("Undefined behaviour will occur in certain siutations when sync events is not set")
		}

		logrus.Infof("Received ready for shard %d", session.ShardID)
		err = s.HandleReady(event)
	}

	if err != nil {
		logrus.WithError(err).Error("Error handling event")
	}
}

func (s *State) HandleReady(r *discordgo.Ready) error {

	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	err := encoder.Encode(r.User)
	if err != nil {
		return errors.WithMessage(err, "Encode Bot User")
	}

	// Handle the initial load
	s.DB.Update(func(tx *badger.Txn) error {
		tx.Set([]byte("bot_user"), buf.Bytes(), userMeta)
		buf.Reset()
		return nil
	})

	for _, g := range r.Guilds {
		err := s.HandleGuildCreate(g)
		if err != nil {
			return err
		}
	}

	for _, g := range r.Guilds {
		if !g.Unavailable {
			continue
		}

	}

}

func (s *State) HandleGuildCreate(g *discordgo.Guild) error {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	var gCopy = new(discordgo.Guild)
	*gCopy = *g

	gCopy.Members = nil
	gCopy.Presences = nil
	gCopy.VoiceStates = nil

	err := encoder.Encode(gCopy)
	if err != nil {
		return errors.WithMessage(err, "gobEncode gcopy")
	}

	// Handle the initial load
	s.DB.Update(func(tx *badger.Txn) error {

		// encoderSerialized := buf.Bytes()

		tx.Set([]byte("bot_user"), buf.Bytes(), userMeta)
		buf.Reset()

	})

	return nil
}
