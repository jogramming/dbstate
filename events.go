package dbstate

import (
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	// "github.com/dgraph-io/badger"
)

func (s *State) HandleEvent(session *discordgo.Session, eventInterface interface{}) {
	if _, ok := eventInterface.(*discordgo.Event); ok {
		// Fast path this since this is sent for every single event
		return
	}

	var err error

	switch event := eventInterface.(type) {
	case *discordgo.Ready:
		if !session.SyncEvents {
			// We rely on only 1 events per shard being handled at a time to make reusing buffers without locks easier
			panic("Session.SyncEvents not set, this mode is unsupported.")
		}

		if session.ShardCount != s.numShards && session.ShardCount > 0 {
			// If this is true it would panic down the line anyways, make it clear what went wrong
			panic("Incorrect shard counts passed to NewState, session.ShardCount had different value")
		}

		logrus.Infof("Received ready for shard %d", session.ShardID)
		err = s.HandleReady(session.ShardID, event)
	}

	if err != nil {
		logrus.WithError(err).Error("Error handling event")
	}
}

func (s *State) HandleReady(shardID int, r *discordgo.Ready) error {

	// Handle the initial load
	err := s.setKey(shardID, nil, KeySelfUser, r.User)
	if err != nil {
		return err
	}

	for _, g := range r.Guilds {
		err := s.HandleGuildCreate(shardID, g)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *State) HandleGuildCreate(shardID int, g *discordgo.Guild) error {
	var gCopy = new(discordgo.Guild)
	*gCopy = *g

	gCopy.Members = nil
	gCopy.Presences = nil
	gCopy.VoiceStates = nil

	// Handle the initial load
	err := s.setKey(shardID, nil, KeyGuild(g.ID), gCopy)

	return err
}
