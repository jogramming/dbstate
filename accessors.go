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

// IterateGuilds Iterates over all guilds, calling f on them
// if f returns false, iterating will stop
func (s *State) IterateGuilds(txn *badger.Txn, f func(g *discordgo.Guild) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateGuilds(txn, f)
		})
	}

	// Scan over the prefix
	prefix := []byte{byte(KeyTypeGuild)}

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return err
		}

		var dest *discordgo.Guild
		err = s.DecodeData(v, &dest)
		if err != nil {
			return err
		}

		// Call the callback
		if !f(dest) {
			break
		}
	}
	return nil
}

func (s *State) GuildMember(txn *badger.Txn, guildID, userID string) (*discordgo.Member, error) {
	var dest discordgo.Member
	err := s.GetKey(txn, KeyGuildMember(guildID, userID), &dest)
	return &dest, err
}

// IterateGuildMembers Iterates over all guild members of provided guildID,
// calling f on them, if f returns false, iterating will stop
func (s *State) IterateGuildMembers(txn *badger.Txn, guildID string, f func(m *discordgo.Member) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateGuildMembers(txn, guildID, f)
		})
	}

	// Scan over the prefix
	prefix := []byte(KeyGuildMembersIteratorPrefix(guildID))

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return err
		}

		var dest *discordgo.Member
		err = s.DecodeData(v, &dest)
		if err != nil {
			return err
		}

		// Call the callback
		if !f(dest) {
			break
		}
	}
	return nil
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

// IterateChannelMessages Iterates over all guild members of provided guildID,
// calling f on them, if f returns false, iterating will stop
func (s *State) IterateChannelMessages(txn *badger.Txn, channelID string, f func(m *discordgo.Message) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateChannelMessages(txn, channelID, f)
		})
	}

	// Scan over the prefix
	prefix := []byte(KeyChannelMessageIteratorPrefix(channelID))

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return err
		}

		var dest *discordgo.Message
		err = s.DecodeData(v, &dest)
		if err != nil {
			return err
		}

		// Call the callback
		if !f(dest) {
			break
		}
	}
	return nil
}

// Presence returns a presence from state
func (s *State) Presence(txn *badger.Txn, userID string) (st *discordgo.Presence, err error) {
	err = s.GetKey(txn, KeyPresence(userID), &st)
	return
}
