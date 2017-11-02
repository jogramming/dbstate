package dbstate

import (
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"time"
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
	case *discordgo.GuildCreate:
		err = s.GuildCreate(session.ShardID, event.Guild)
	case *discordgo.GuildUpdate:
		err = s.GuildUpdate(0, event.Guild)
	case *discordgo.GuildDelete:
		err = s.GuildDelete(event.Guild.ID)
	case *discordgo.GuildMemberAdd:
		err = s.MemberAdd(session.ShardID, nil, event.Member, true)
	case *discordgo.GuildMemberUpdate:
		err = s.MemberUpdate(session.ShardID, nil, event.Member)
	case *discordgo.GuildMemberRemove:
		err = s.MemberRemove(session.ShardID, nil, event.Member.GuildID, event.Member.User.ID, true)
	case *discordgo.ChannelCreate:
		err = s.ChannelCreateUpdate(session.ShardID, nil, event.Channel, true)
	case *discordgo.ChannelUpdate:
		err = s.ChannelCreateUpdate(session.ShardID, nil, event.Channel, true)
	case *discordgo.ChannelDelete:
		err = s.ChannelDelete(session.ShardID, nil, event.Channel.ID)
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
		err := s.GuildCreate(shardID, g)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *State) GuildCreate(shardID int, g *discordgo.Guild) error {
	var gCopy = new(discordgo.Guild)
	*gCopy = *g

	gCopy.Members = nil
	gCopy.Presences = nil
	gCopy.VoiceStates = nil

	started := time.Now()
	err := s.DB.Update(func(txn *badger.Txn) error {
		// Handle the initial load
		err := s.setKey(shardID, txn, KeyGuild(g.ID), gCopy)
		if err != nil {
			return err
		}

		// Load members
		for _, m := range g.Members {
			m.GuildID = g.ID
			err := s.MemberUpdate(shardID, txn, m)
			if err != nil {
				return err
			}
		}

		// Load channels to global registry
		for _, c := range g.Channels {
			c.GuildID = g.ID
			err := s.ChannelCreateUpdate(0, txn, c, false)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if len(g.Members) > 100 {
		logrus.Infof("Handled %d members in %s", len(g.Members), time.Since(started))
	}

	return err
}

func (s *State) GuildUpdate(shardID int, g *discordgo.Guild) error {
	err := s.DB.Update(func(txn *badger.Txn) error {
		current, err := s.Guild(txn, g.ID)
		if err != nil {
			return errors.WithMessage(err, "GuildUpdate")
		}

		current.Name = g.Name
		current.Icon = g.Icon
		current.Splash = g.Splash
		current.OwnerID = g.OwnerID
		current.Region = g.Region
		current.AfkTimeout = g.AfkTimeout
		current.AfkChannelID = g.AfkChannelID
		current.EmbedEnabled = g.EmbedEnabled
		current.EmbedChannelID = g.EmbedChannelID
		current.VerificationLevel = g.VerificationLevel
		current.DefaultMessageNotifications = g.DefaultMessageNotifications

		return s.setKey(shardID, txn, KeyGuild(g.ID), current)
	})

	return err
}

// GuildDelete removes a guild from the state
func (s *State) GuildDelete(guildID string) error {
	return s.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(KeyGuild(guildID)))
	})
}

// MemberAdd will increment membercount and update the member,
// if you call this on members already in the count, your membercount will be off
func (s *State) MemberAdd(shardID int, txn *badger.Txn, m *discordgo.Member, updateCount bool) error {
	if txn == nil {
		return s.DB.Update(func(txn *badger.Txn) error {
			return s.MemberAdd(shardID, txn, m, updateCount)
		})
	}

	if updateCount {
		guild, err := s.Guild(txn, m.GuildID)
		if err != nil {
			return err
		}
		guild.MemberCount++
		err = s.setKey(shardID, txn, KeyGuild(m.GuildID), guild)
		if err != nil {
			return err
		}
	}

	return s.MemberUpdate(shardID, txn, m)
}

// MemberUpdate updates the current stored state of said member
func (s *State) MemberUpdate(shardID int, txn *badger.Txn, m *discordgo.Member) error {
	if txn == nil {
		return s.DB.Update(func(txn *badger.Txn) error {
			return s.MemberUpdate(shardID, txn, m)
		})
	}

	return s.setKey(shardID, txn, KeyGuildMember(m.GuildID, m.User.ID), m)
}

// MemberRemove will decrement membercount if "updateCount" and remove the member form state
func (s *State) MemberRemove(shardID int, txn *badger.Txn, guildID, userID string, updateCount bool) error {
	if txn == nil {
		return s.DB.Update(func(txn *badger.Txn) error {
			return s.MemberRemove(shardID, txn, guildID, userID, updateCount)
		})
	}

	if updateCount {
		guild, err := s.Guild(txn, guildID)
		if err != nil {
			return err
		}
		guild.MemberCount--
		err = s.setKey(shardID, txn, KeyGuild(guildID), guild)
		if err != nil {
			return err
		}
	}

	return txn.Delete([]byte(KeyGuildMember(guildID, userID)))
}

// ChannelCreateUpdate creates or updates a channel in the state
// if addtoguild is set, it will add and update it on the actual guild object aswell
func (s *State) ChannelCreateUpdate(shardID int, txn *badger.Txn, channel *discordgo.Channel, addToGuild bool) error {
	if txn == nil {
		return s.DB.Update(func(txn *badger.Txn) error {
			return s.ChannelCreateUpdate(shardID, txn, channel, addToGuild)
		})
	}

	err := s.DB.Update(func(txn *badger.Txn) error {
		// Update the channels object on the guild
		if channel.Type == discordgo.ChannelTypeGuildText && addToGuild {
			guild, err := s.Guild(txn, channel.GuildID)
			if err != nil {
				return errors.WithMessage(err, "GuildUpdate")
			}

			found := false
			for i, v := range guild.Channels {
				if v.ID == channel.ID {
					if channel.PermissionOverwrites == nil {
						channel.PermissionOverwrites = v.PermissionOverwrites
					}

					guild.Channels[i] = channel
					found = true
					break
				}
			}

			if !found {
				guild.Channels = append(guild.Channels, channel)
			}

			err = s.setKey(shardID, txn, KeyGuild(guild.ID), guild)
			if err != nil {
				return err
			}
		}

		// Update the global entry
		return s.setKey(shardID, txn, KeyChannel(channel.ID), channel)
	})

	return err
}

// ChannelDelete removes a channel from state
func (s *State) ChannelDelete(shardID int, txn *badger.Txn, channelID string) error {
	channel, err := s.Channel(txn, channelID)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}

		// Dosen't exist in our state
		return nil
	}

	err = s.DB.Update(func(txn *badger.Txn) error {
		// Update the channels object on the guild
		if channel.Type == discordgo.ChannelTypeGuildText {
			guild, err := s.Guild(txn, channel.GuildID)
			if err != nil {
				return errors.WithMessage(err, "GuildUpdate")
			}

			for i, v := range guild.Channels {
				if v.ID == channel.ID {
					guild.Channels = append(guild.Channels[:i], guild.Channels[i+1:]...)
					break
				}
			}

			err = s.setKey(shardID, txn, KeyGuild(guild.ID), guild)
			if err != nil {
				return err
			}
		}

		// Update the global entry
		return txn.Delete([]byte(KeyChannel(channelID)))
	})

	return err
}
