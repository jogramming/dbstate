package dbstate

import (
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
)

// IterateGuilds Iterates over all *discordgo.Guild in state, calling f on them
// if f returns false then iteration will stop
func (s *State) IterateGuilds(txn *badger.Txn, f func(d *discordgo.Guild) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateGuilds(txn, f)
		})
	}

	// Scan over the prefix
	prefix := []byte{byte(KeyTypeGuild)}
	seek := prefix

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
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

// IteratePresences Iterates over all *discordgo.Presence in state, calling f on them
// if f returns false then iteration will stop
func (s *State) IteratePresences(txn *badger.Txn, f func(d *discordgo.Presence) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IteratePresences(txn, f)
		})
	}

	// Scan over the prefix
	prefix := []byte{byte(KeyTypePresence)}
	seek := prefix

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return err
		}

		var dest *discordgo.Presence
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

// IterateGuildMembers Iterates over all *discordgo.Member in state, calling f on them
// if f returns false then iteration will stop
func (s *State) IterateGuildMembers(txn *badger.Txn, guildID string, f func(d *discordgo.Member) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateGuildMembers(txn, guildID, f)
		})
	}

	// Scan over the prefix
	prefix := KeyGuildMembersIteratorPrefix(guildID)
	seek := prefix

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
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

// IterateChannelMessages Iterates over all *discordgo.Message in state, calling f on them
// if f returns false then iteration will stop
func (s *State) IterateChannelMessages(txn *badger.Txn, channelID string, f func(m MessageFlag, d *discordgo.Message) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateChannelMessages(txn, channelID, f)
		})
	}

	// Scan over the prefix
	prefix := KeyChannelMessageIteratorPrefix(channelID)
	seek := prefix

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
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

		meta := MessageFlag(item.UserMeta())

		// Call the callback
		if !f(meta, dest) {
			break
		}
	}
	return nil
}

// IterateChannelMessagesNewerFirst Iterates over all *discordgo.Message in state, calling f on them
// if f returns false then iteration will stop
func (s *State) IterateChannelMessagesNewerFirst(txn *badger.Txn, channelID string, f func(m MessageFlag, d *discordgo.Message) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateChannelMessagesNewerFirst(txn, channelID, f)
		})
	}

	// Scan over the prefix
	prefix := KeyChannelMessageIteratorPrefix(channelID)
	seek := prefix
	seek = make([]byte, 17)
	copy(seek, prefix)
	// Seek to the last possible message in this channel
	seek[9] = 0xff
	seek[10] = 0xff
	seek[11] = 0xff
	seek[12] = 0xff
	seek[13] = 0xff
	seek[14] = 0xff
	seek[15] = 0xff
	seek[16] = 0xff

	opts := badger.DefaultIteratorOptions
	opts.Reverse = true

	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
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

		meta := MessageFlag(item.UserMeta())

		// Call the callback
		if !f(meta, dest) {
			break
		}
	}
	return nil
}

// IterateAllMessages Iterates over all *discordgo.Message in state, calling f on them
// if f returns false then iteration will stop
func (s *State) IterateAllMessages(txn *badger.Txn, f func(m MessageFlag, d *discordgo.Message) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateAllMessages(txn, f)
		})
	}

	// Scan over the prefix
	prefix := []byte{byte(KeyTypeChannelMessage)}
	seek := prefix

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
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

		meta := MessageFlag(item.UserMeta())

		// Call the callback
		if !f(meta, dest) {
			break
		}
	}
	return nil
}

// IterateGuildVoiceStates Iterates over all *discordgo.VoiceState in state, calling f on them
// if f returns false then iteration will stop
func (s *State) IterateGuildVoiceStates(txn *badger.Txn, guildID string, f func(d *discordgo.VoiceState) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.IterateGuildVoiceStates(txn, guildID, f)
		})
	}

	// Scan over the prefix
	prefix := KeyVoiceStateIteratorPrefix(guildID)
	seek := prefix

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return err
		}

		var dest *discordgo.VoiceState
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
