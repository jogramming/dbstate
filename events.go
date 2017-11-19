package dbstate

import (
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/jonas747/discordgo"
	"github.com/pkg/errors"
	"reflect"
	"time"
)

// HandleEventChannelSynced handles an incoming event
//
// Use this as opposed to HandleEventMutexSynced if you're okay with
// this function returning before the state has actually been updated from the event
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
// Use this as opposed to HandleEventChannelSynced
// if you need make sure the state has been updated by the time this returns
func (s *State) HandleEventMutexSynced(shardID int, eventInterface interface{}) error {
	if !s.handleEventPreCheck(shardID, eventInterface) {
		return nil
	}

	w := s.shards[shardID]
	w.MU.Lock()
	defer w.MU.Unlock()

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
		case <-w.State.stopChan:
			return
		case event := <-w.eventChan:
			err := w.handleEvent(event)
			if err != nil {
				w.State.opts.Logger.LogError("Failed handling event: ", err)
			}
		}
	}
}

// handleEvent does the actual handling of the event, this is not thread safe,
// if multiple goroutines calls this at the same time, state will get corrupted
func (w *shardWorker) handleEvent(eventInterface interface{}) error {

	var err error

	switch event := eventInterface.(type) {
	case *discordgo.PresenceUpdate:
		if !w.State.opts.TrackPresences {
			return nil
		}

		if w.State.presenceUpdateFilter.checkUserID(w.shardID, event.User.ID) {
			return nil
		}

		err = w.PresenceAddUpdate(nil, false, &event.Presence)
	case *discordgo.Ready:
		err = w.HandleReady(event)

	// Guilds
	case *discordgo.GuildCreate:
		err = w.GuildCreate(event.Guild)
	case *discordgo.GuildUpdate:
		err = w.GuildUpdate(event.Guild)
	case *discordgo.GuildDelete:
		err = w.GuildDelete(event.Guild.ID)

	// Members
	case *discordgo.GuildMemberAdd:
		if !w.State.opts.TrackMembers {
			return nil
		}
		err = w.MemberAdd(nil, event.Member, true)
	case *discordgo.GuildMemberUpdate:
		if !w.State.opts.TrackMembers {
			return nil
		}
		err = w.MemberUpdate(nil, event.Member)
	case *discordgo.GuildMemberRemove:
		if !w.State.opts.TrackMembers {
			return nil
		}
		err = w.MemberRemove(nil, event.Member.GuildID, event.Member.User.ID, true)

	// Roles
	case *discordgo.GuildRoleCreate:
		if !w.State.opts.TrackRoles {
			return nil
		}
		err = w.RoleCreateUpdate(nil, event.GuildID, event.Role)
	case *discordgo.GuildRoleUpdate:
		if !w.State.opts.TrackRoles {
			return nil
		}
		err = w.RoleCreateUpdate(nil, event.GuildID, event.Role)
	case *discordgo.GuildRoleDelete:
		if !w.State.opts.TrackRoles {
			return nil
		}
		err = w.RoleDelete(nil, event.GuildID, event.RoleID)

	// Channels
	case *discordgo.ChannelCreate:
		if !w.State.opts.TrackChannels {
			return nil
		}
		err = w.ChannelCreateUpdate(nil, event.Channel, true)
	case *discordgo.ChannelUpdate:
		if !w.State.opts.TrackChannels {
			return nil
		}
		err = w.ChannelCreateUpdate(nil, event.Channel, true)
	case *discordgo.ChannelDelete:
		if !w.State.opts.TrackChannels {
			return nil
		}
		err = w.ChannelDelete(nil, event.Channel.ID)

	// Messages
	case *discordgo.MessageCreate:
		if !w.State.opts.TrackMessages {
			return nil
		}
		err = w.MessageCreateUpdate(nil, event.Message)
	case *discordgo.MessageUpdate:
		if !w.State.opts.TrackMessages {
			return nil
		}
		err = w.MessageCreateUpdate(nil, event.Message)
	case *discordgo.MessageDelete:
		if !w.State.opts.TrackMessages {
			return nil
		}
		err = w.MessageDelete(nil, event.ChannelID, event.ID)

	// Misc
	case *discordgo.GuildEmojisUpdate:
		err = w.EmojisUpdate(nil, event.GuildID, event.Emojis)
	case *discordgo.VoiceStateUpdate:
		err = w.VoiceStateUpdate(nil, event.VoiceState)
	default:
		return nil
	}

	typ := reflect.Indirect(reflect.ValueOf(eventInterface)).Type()
	evtName := typ.Name()
	// logrus.Infof("Handled event %s", evtName)

	if err != nil {
		return errors.WithMessage(err, evtName)
	}

	return nil
}

// HandleReady handles the ready event, doing some initial loading
func (w *shardWorker) HandleReady(r *discordgo.Ready) error {

	w.State.memoryState.Lock()
	w.State.memoryState.User = r.User.User
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
	err := w.State.RetryUpdate(func(txn *badger.Txn) error {
		// Handle the initial load
		err := w.setKey(txn, KeyGuild(g.ID), gCopy)
		if err != nil {
			return err
		}

		// Load channels to global registry
		for _, c := range g.Channels {
			c.GuildID = g.ID
			err := w.ChannelCreateUpdate(txn, c, false)
			if err != nil {
				return err
			}
		}

		// Load voice states
		for _, vs := range g.VoiceStates {
			err := w.VoiceStateUpdate(txn, vs)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if w.State.opts.TrackMembers {
		err = w.LoadMembers(g.ID, g.Members)
		if err != nil {
			return err
		}
	}

	if w.State.opts.TrackPresences {
		err = w.LoadPresences(g.Presences)
		if err != nil {
			return err
		}
	}

	if len(g.Members) > 1000 {
		w.State.opts.Logger.LogInfo(fmt.Sprintf("Handled %d members in %s", len(g.Members), time.Since(started)))
	}

	return err
}

// LoadMembers Loads the members using multiple transactions to avoid going above the tx limit
func (w *shardWorker) LoadMembers(gID string, members []*discordgo.Member) error {
	i := 0

	for {

		err := w.State.RetryUpdate(func(txn *badger.Txn) error {
			// Update the members until we either have gone through the member slice or
			// we have updates more than 1k members
			startedI := i
			iCop := i
			for ; iCop < len(members); iCop++ {

				m := members[iCop]
				m.GuildID = gID

				err := w.MemberUpdate(txn, m)
				if err != nil {
					return err
				}

				if iCop-startedI >= 1000 {
					i = iCop + 1
					return nil
				}
			}

			// Done
			i = iCop
			return nil
		})

		if err != nil {
			return err
		}

		// Also done
		if i >= len(members) {
			return nil
		}
	}
}

// LoadPresences Loads the presences using multiple transactions to avoid going above the tx limit
func (w *shardWorker) LoadPresences(presences []*discordgo.Presence) error {
	i := 0

	for {

		err := w.State.RetryUpdate(func(txn *badger.Txn) error {
			// Update the presences until we either have gone through the presence slice or
			// we have updated more than 1k presences
			startedI := i
			iCop := i
			for ; iCop < len(presences); iCop++ {

				p := presences[iCop]
				err := w.PresenceAddUpdate(txn, true, p)
				if err != nil {
					return err
				}

				if iCop-startedI >= 1000 {
					i = iCop + 1
					return nil
				}
			}

			// Done
			i = iCop
			return nil
		})

		if err != nil {
			return err
		}

		// Also done
		if i >= len(presences) {
			return nil
		}
	}
}

func (w *shardWorker) GuildUpdate(g *discordgo.Guild) error {
	err := w.State.RetryUpdate(func(txn *badger.Txn) error {
		current, err := w.guild(txn, g.ID)
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
	return w.State.RetryUpdate(func(txn *badger.Txn) error {
		return txn.Delete([]byte(KeyGuild(guildID)))
	})
}

// MemberAdd will increment membercount and update the member,
// if you call this on members already in the count, your membercount will be off
func (w *shardWorker) MemberAdd(txn *badger.Txn, m *discordgo.Member, updateCount bool) error {
	if txn == nil {
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.MemberAdd(txn, m, updateCount)
		})
	}

	if updateCount {
		guild, err := w.guild(txn, m.GuildID)
		if err != nil {
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
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.MemberRemove(txn, guildID, userID, updateCount)
		})
	}

	if updateCount {
		guild, err := w.guild(txn, guildID)
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
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.ChannelCreateUpdate(txn, channel, addToGuild)
		})
	}

	// Update the channels object on the guild
	if channel.Type == discordgo.ChannelTypeGuildText && addToGuild {
		guild, err := w.guild(txn, channel.GuildID)
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
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.ChannelDelete(txn, channelID)
		})
	}

	channel, err := w.channel(txn, channelID)
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return err
		}

		// Dosen't exist in our state
		return nil
	}

	// Update the channels object on the guild
	if channel.Type == discordgo.ChannelTypeGuildText {
		guild, err := w.guild(txn, channel.GuildID)
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
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.RoleCreateUpdate(txn, guildID, role)
		})
	}

	// Update the roles slice on the guild
	guild, err := w.guild(txn, guildID)
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
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.RoleDelete(txn, guildID, roleID)
		})
	}

	guild, err := w.guild(txn, guildID)
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
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.MessageCreateUpdate(txn, newMsg)
		})
	}

	msg, _, err := w.channelMessage(txn, newMsg.ChannelID, newMsg.ID)
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

	return w.setKeyWithTTL(txn, KeyChannelMessage(newMsg.ChannelID, newMsg.ID), msg, w.State.opts.MessageTTL)
}

func (w *shardWorker) MessageDelete(txn *badger.Txn, channelID, messageID string) error {
	if txn == nil {
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.MessageDelete(txn, channelID, messageID)
		})
	}

	if !w.State.opts.KeepDeletedMessages {
		return txn.Delete([]byte(KeyChannelMessage(channelID, messageID)))
	}

	current, flags, err := w.channelMessage(txn, channelID, messageID)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil
		}
		return err
	}

	flags |= MessageFlagDeleted
	return w.setKeyWithMeta(txn, KeyChannelMessage(channelID, messageID), current, byte(flags))
}

// RoleDelete removes a role from state
func (w *shardWorker) EmojisUpdate(txn *badger.Txn, guildID string, emojis []*discordgo.Emoji) error {
	if txn == nil {
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.EmojisUpdate(txn, guildID, emojis)
		})
	}

	guild, err := w.guild(txn, guildID)
	if err != nil {
		return errors.WithMessage(err, "Guild")
	}

OUTER:
	for _, newEmoji := range emojis {
		for _, v := range guild.Emojis {
			if v.ID == newEmoji.ID {
				continue OUTER
			}
		}

		guild.Emojis = append(guild.Emojis, newEmoji)
	}

	err = w.setKey(txn, KeyGuild(guild.ID), guild)
	if err != nil {
		return errors.WithMessage(err, "SetGuild")
	}

	return nil
}

// PresenceUpdate will add or update an existing presence in state
func (w *shardWorker) PresenceAddUpdate(txn *badger.Txn, forceAdd bool, p *discordgo.Presence) error {
	if txn == nil {
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.PresenceAddUpdate(txn, forceAdd, p)
		})
	}

	var current *discordgo.Presence
	if !forceAdd {
		var err error
		current, err = w.presence(txn, p.User.ID)
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}
	}

	if current != nil {
		// update the existing one
		if p.User.Username != "" {
			current.User.Username = p.User.Username
		}
		if p.User.Discriminator != "" {
			current.User.Discriminator = p.User.Discriminator
		}
		if p.User.Avatar != "" {
			current.User.Avatar = p.User.Avatar
		}
		if p.Status != "" {
			current.Status = p.Status
		}

		current.Game = p.Game
		current.Nick = p.Nick
	} else {
		current = p
	}

	return w.setKey(txn, KeyPresence(p.User.ID), current)
}

// VoiceStateUpdate will add/update/delete a voice state in state
func (w *shardWorker) VoiceStateUpdate(txn *badger.Txn, vs *discordgo.VoiceState) error {
	if txn == nil {
		return w.State.RetryUpdate(func(txn *badger.Txn) error {
			return w.VoiceStateUpdate(txn, vs)
		})
	}

	if vs.ChannelID == "" {
		// Left the channel
		err := txn.Delete(KeyVoiceState(vs.GuildID, vs.UserID))
		if err != badger.ErrKeyNotFound && err != nil {
			return err
		}
	} else {
		return w.setKey(txn, KeyVoiceState(vs.GuildID, vs.UserID), vs)
	}

	return nil
}
