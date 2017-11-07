package dbstate

import (
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
)

// SelfUser returns the current user from the ready payload,
// if the ready payload from atleast 1 shard hasn't been received this will return nil
func (s *State) SelfUser() *discordgo.User {
	s.memoryState.RLock()
	u := s.memoryState.User
	s.memoryState.RUnlock()

	return u
}

// Guild retrieves a guild form the state
// Note that members and presences will not be included in this
// and will have to be queried seperately
func (s *State) Guild(txn *badger.Txn, id string) (*discordgo.Guild, error) {
	var dest *discordgo.Guild
	err := s.GetKey(txn, KeyGuild(id), &dest)
	return dest, err
}

func (s *State) GuildMember(txn *badger.Txn, guildID, userID string) (*discordgo.Member, error) {
	var dest discordgo.Member
	err := s.GetKey(txn, KeyGuildMember(guildID, userID), &dest)
	return &dest, err
}

// Channel returns a channel from the state
func (s *State) Channel(txn *badger.Txn, channelID string) (*discordgo.Channel, error) {
	var dest discordgo.Channel
	err := s.GetKey(txn, KeyChannel(channelID), &dest)
	return &dest, err
}

func (s *State) ChannelMessage(txn *badger.Txn, channelID, messageID string) (st *discordgo.Message, err error) {
	err = s.GetKey(txn, KeyChannelMessage(channelID, messageID), &st)
	return
}

// Presence returns a presence from state
func (s *State) Presence(txn *badger.Txn, userID string) (st *discordgo.Presence, err error) {
	err = s.GetKey(txn, KeyPresence(userID), &st)
	return
}

// VoiceState returns a VoiceState from state
func (s *State) VoiceState(txn *badger.Txn, guildID, userID string) (st *discordgo.VoiceState, err error) {
	err = s.GetKey(txn, KeyVoiceState(guildID, userID), &st)
	return
}
