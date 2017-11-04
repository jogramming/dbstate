package dbstate

import (
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"reflect"
	"time"
)

// HandleEventChannelSynced handles an incoming event
//
// Use this as opposed to HandleEventMutexSynced if you're okay with other
// handlers being called before the state has been updated
func (s *State) HandleEventChannelSynced(shardID int, eventInterface interface{}) error {
	if !s.handleEventPreCheck(shardID, eventInterface) {
		return nil
	}

	// Send the event to the proper worker
	s.shards[shardID].eventChan <- eventInterface
	return nil
}

// HandleEventMutexSynced handles an incoming event
//
// Use this ass opposed to HandleEventChannelSynced
// if you need make sure the state has been updated by the time this returns
func (s *State) HandleEventMutexSynced(shardID int, eventInterface interface{}) error {
	if !s.handleEventPreCheck(shardID, eventInterface) {
		return nil
	}

	w := s.shards[shardID]
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.handleEvent(eventInterface)
}

// HandleEventNoSync handles an incoming event
//
// Use this as opposed to mutex synced and channel synced when you provide your own synchronization
// if this is called by 2 goroutines at once then the state gets corrupted
func (s *State) HandleEventNoSync(shardID int, eventInterface interface{}) error {
	if !s.handleEventPreCheck(shardID, eventInterface) {
		return nil
	}

	// Send the event to the proper worker
	return s.shards[shardID].handleEvent(eventInterface)
}

func (s *State) handleEventPreCheck(shardID int, eventInterface interface{}) bool {
	if _, ok := eventInterface.(*discordgo.Event); ok {
		// Fast path this since this is sent for every single event
		return false
	}

	if s.numShards <= shardID {
		// If this is true it would panic down the line anyways, make it clear what went wrong
		panic("ShardID is higher than count passed to state")
	}

	return true
}

func (w *shardWorker) run() {
	for {
		select {
		case event := <-w.eventChan:
			err := w.handleEvent(event)
			if err != nil {
				// TODO: Add support for custom loggers
				logrus.WithError(err).Error("Failed handling event")
			}
		}
	}
}

// handleEvent does the actual handling of the event, this is not thread safe,
// if multiple goroutines calls this at the same time, state will get corrupted
func (w *shardWorker) handleEvent(eventInterface interface{}) error {

	var err error

	switch event := eventInterface.(type) {
	case *discordgo.Ready:
		logrus.Infof("Received ready for shard %d", w.shardID)
		err = w.HandleReady(event)
	case *discordgo.GuildCreate:
		err = w.GuildCreate(event.Guild)
	case *discordgo.GuildUpdate:
		err = w.GuildUpdate(event.Guild)
	case *discordgo.GuildDelete:
		err = w.GuildDelete(event.Guild.ID)
	case *discordgo.GuildMemberAdd:
		err = w.MemberAdd(nil, event.Member, true)
	case *discordgo.GuildMemberUpdate:
		err = w.MemberUpdate(nil, event.Member)
	case *discordgo.GuildMemberRemove:
		err = w.MemberRemove(nil, event.Member.GuildID, event.Member.User.ID, true)
	case *discordgo.ChannelCreate:
		err = w.ChannelCreateUpdate(nil, event.Channel, true)
	case *discordgo.ChannelUpdate:
		err = w.ChannelCreateUpdate(nil, event.Channel, true)
	case *discordgo.ChannelDelete:
		err = w.ChannelDelete(nil, event.Channel.ID)
	case *discordgo.GuildRoleCreate:
		err = w.RoleCreateUpdate(nil, event.GuildID, event.Role)
	case *discordgo.GuildRoleUpdate:
		err = w.RoleCreateUpdate(nil, event.GuildID, event.Role)
	case *discordgo.GuildRoleDelete:
		err = w.RoleDelete(nil, event.GuildID, event.RoleID)
	case *discordgo.MessageCreate:
		err = w.MessageCreateUpdate(nil, event.Message)
	case *discordgo.MessageUpdate:
		err = w.MessageCreateUpdate(nil, event.Message)
	case *discordgo.MessageDelete:
		err = w.MessageDelete(nil, event.ChannelID, event.ID)
	default:
		return nil
	}

	typ := reflect.Indirect(reflect.ValueOf(eventInterface)).Type()
	evtName := typ.Name()
	logrus.Infof("Handled event %s", evtName)

	if err != nil {
		return errors.WithMessage(err, evtName)
	}

	return nil
}

// HandleReady handles the ready event, doing some initial loading
func (w *shardWorker) HandleReady(r *discordgo.Ready) error {

	w.State.memoryState.Lock()
	w.State.memoryState.User = r.User
	w.State.memoryState.Unlock()

	// Handle the initial load
	err := w.setKey(nil, KeySelfUser, r.User)
	if err != nil {
		return err
	}

	for _, g := range r.Guilds {
		err := w.GuildCreate(g)
		if err != nil {
			return err
		}
	}

	return nil
}

// GuildCreate adds a guild to the state
func (w *shardWorker) GuildCreate(g *discordgo.Guild) error {
	var gCopy = new(discordgo.Guild)
	*gCopy = *g

	gCopy.Members = nil
	gCopy.Presences = nil
	gCopy.VoiceStates = nil

	started := time.Now()
	err := w.State.DB.Update(func(txn *badger.Txn) error {
		// Handle the initial load
		err := w.setKey(txn, KeyGuild(g.ID), gCopy)
		if err != nil {
			return err
		}

		// Load members
		for _, m := range g.Members {
			m.GuildID = g.ID
			err := w.MemberUpdate(txn, m)
			if err != nil {
				return err
			}
		}

		// Load channels to global registry
		for _, c := range g.Channels {
			c.GuildID = g.ID
			err := w.ChannelCreateUpdate(txn, c, false)
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

func (w *shardWorker) GuildUpdate(g *discordgo.Guild) error {
	err := w.State.DB.Update(func(txn *badger.Txn) error {
		current, err := w.State.Guild(txn, g.ID)
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

		return w.setKey(txn, KeyGuild(g.ID), current)
	})

	return err
}

// GuildDelete removes a guild from the state
func (w *shardWorker) GuildDelete(guildID string) error {
	return w.State.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(KeyGuild(guildID)))
	})
}

// MemberAdd will increment membercount and update the member,
// if you call this on members already in the count, your membercount will be off
func (w *shardWorker) MemberAdd(txn *badger.Txn, m *discordgo.Member, updateCount bool) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.MemberAdd(txn, m, updateCount)
		})
	}

	if updateCount {
		guild, err := w.State.Guild(txn, m.GuildID)
		if err != nil {
			logrus.Info(m.GuildID)
			return errors.WithMessage(err, "Guild")
		}
		guild.MemberCount++
		err = w.setKey(txn, KeyGuild(m.GuildID), guild)
		if err != nil {
			return err
		}
	}

	return w.MemberUpdate(txn, m)
}

// MemberUpdate updates the current stored state of said member
func (w *shardWorker) MemberUpdate(txn *badger.Txn, m *discordgo.Member) error {
	return w.setKey(txn, KeyGuildMember(m.GuildID, m.User.ID), m)
}

// MemberRemove will decrement membercount if "updateCount" and remove the member form state
func (w *shardWorker) MemberRemove(txn *badger.Txn, guildID, userID string, updateCount bool) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.MemberRemove(txn, guildID, userID, updateCount)
		})
	}

	if updateCount {
		guild, err := w.State.Guild(txn, guildID)
		if err != nil {
			return errors.WithMessage(err, "Guild")
		}
		guild.MemberCount--
		err = w.setKey(txn, KeyGuild(guildID), guild)
		if err != nil {
			return errors.WithMessage(err, "SetGuild")
		}
	}

	return txn.Delete([]byte(KeyGuildMember(guildID, userID)))
}

// ChannelCreateUpdate creates or updates a channel in the state
// if addtoguild is set, it will add and update it on the actual guild object aswell
func (w *shardWorker) ChannelCreateUpdate(txn *badger.Txn, channel *discordgo.Channel, addToGuild bool) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.ChannelCreateUpdate(txn, channel, addToGuild)
		})
	}

	// Update the channels object on the guild
	if channel.Type == discordgo.ChannelTypeGuildText && addToGuild {
		guild, err := w.State.Guild(txn, channel.GuildID)
		if err != nil {
			return errors.WithMessage(err, "ChannelUpdate")
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

		err = w.setKey(txn, KeyGuild(guild.ID), guild)
		if err != nil {
			return err
		}
	}

	// Update the global entry
	return w.setKey(txn, KeyChannel(channel.ID), channel)
}

// ChannelDelete removes a channel from state
func (w *shardWorker) ChannelDelete(txn *badger.Txn, channelID string) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.ChannelDelete(txn, channelID)
		})
	}

	channel, err := w.State.Channel(txn, channelID)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}

		// Dosen't exist in our state
		return nil
	}

	// Update the channels object on the guild
	if channel.Type == discordgo.ChannelTypeGuildText {
		guild, err := w.State.Guild(txn, channel.GuildID)
		if err != nil {
			return errors.WithMessage(err, "ChannelCreateUpdate")
		}

		for i, v := range guild.Channels {
			if v.ID == channel.ID {
				guild.Channels = append(guild.Channels[:i], guild.Channels[i+1:]...)
				break
			}
		}

		err = w.setKey(txn, KeyGuild(guild.ID), guild)
		if err != nil {
			return err
		}
	}

	// Update the global entry
	return txn.Delete([]byte(KeyChannel(channelID)))
}

// RoleCreateUpdate creates or updates a role in the state
// These roles are actually on the guild at the moment
func (w *shardWorker) RoleCreateUpdate(txn *badger.Txn, guildID string, role *discordgo.Role) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.RoleCreateUpdate(txn, guildID, role)
		})
	}

	// Update the roles slice on the guild
	guild, err := w.State.Guild(txn, guildID)
	if err != nil {
		return errors.WithMessage(err, "Guild")
	}

	found := false
	for i, v := range guild.Roles {
		if v.ID == role.ID {
			guild.Roles[i] = role
			found = true
			break
		}
	}

	if !found {
		guild.Roles = append(guild.Roles, role)
	}

	err = w.setKey(txn, KeyGuild(guild.ID), guild)
	if err != nil {
		return err
	}

	return err
}

// RoleDelete removes a role from state
func (w *shardWorker) RoleDelete(txn *badger.Txn, guildID, roleID string) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.RoleDelete(txn, guildID, roleID)
		})
	}

	guild, err := w.State.Guild(txn, guildID)
	if err != nil {
		return errors.WithMessage(err, "Guild")
	}

	for i, v := range guild.Roles {
		if v.ID == roleID {
			guild.Roles = append(guild.Roles[:i], guild.Roles[i+1:]...)
			break
		}
	}

	err = w.setKey(txn, KeyGuild(guild.ID), guild)
	if err != nil {
		return errors.WithMessage(err, "SetGuild")
	}

	return nil
}

func (w *shardWorker) MessageCreateUpdate(txn *badger.Txn, newMsg *discordgo.Message) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.MessageCreateUpdate(txn, newMsg)
		})
	}

	msg, err := w.State.ChannelMessage(txn, newMsg.ChannelID, newMsg.ID)
	if err == nil && msg != nil {
		if newMsg.Content != "" {
			msg.Content = newMsg.Content
		}
		if newMsg.EditedTimestamp != "" {
			msg.EditedTimestamp = newMsg.EditedTimestamp
		}
		if newMsg.Mentions != nil {
			msg.Mentions = newMsg.Mentions
		}
		if newMsg.Embeds != nil {
			msg.Embeds = newMsg.Embeds
		}
		if newMsg.Attachments != nil {
			msg.Attachments = newMsg.Attachments
		}
		if newMsg.Timestamp != "" {
			msg.Timestamp = newMsg.Timestamp
		}
		if newMsg.Author != nil {
			msg.Author = newMsg.Author
		}
	} else {
		msg = newMsg
	}

	return w.setKeyWithTTL(txn, KeyChannelMessage(newMsg.ChannelID, newMsg.ID), msg, w.State.MessageTTL)
}

func (w *shardWorker) MessageDelete(txn *badger.Txn, channelID, messageID string) error {
	if txn == nil {
		return w.State.DB.Update(func(txn *badger.Txn) error {
			return w.MessageDelete(txn, channelID, messageID)
		})
	}

	return txn.Delete([]byte(KeyChannelMessage(channelID, messageID)))
}
