package dbstate

import (
	"encoding/binary"
	"strconv"
)

// 1st byte of all keys is the key type (members and channels for example)
// that is usually followed by 1 or 2 uint64 id's
type KeyType byte

var (
	KeySelfUser = []byte{'q'}
	KeyMeta     = []byte{'t'}
)

const (
	KeyTypeGuild          KeyType = 'g'
	KeyTypeMember         KeyType = 'f'
	KeyTypeChannel        KeyType = 'c'
	KeyTypeChannelMessage KeyType = 'm'
	KeyTypePresence       KeyType = 'p'
	KeyTypeVoiceState     KeyType = 'v'
	KeyTypeLastMessage    KeyType = 'l'
)

func KeyGuild(guildID string) []byte {

	// 0 keytype, 8 id
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeGuild)

	parsed, _ := strconv.ParseUint(guildID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsed)

	return buf
}

func KeyGuildMember(guildID, userID string) []byte {

	// 1 keytype, 8 guildID, 8 userID = 17
	buf := make([]byte, 17)
	buf[0] = byte(KeyTypeMember)

	parsedG, _ := strconv.ParseUint(guildID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedG)

	parsedM, _ := strconv.ParseUint(userID, 10, 64)
	binary.BigEndian.PutUint64(buf[9:], parsedM)

	return buf
}

func KeyGuildMembersIteratorPrefix(guildID string) []byte {
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeMember)

	parsedG, _ := strconv.ParseUint(guildID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedG)

	return buf
}

func KeyChannel(channelID string) []byte {
	// 1 keytype, 8 channelID
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeChannel)

	parsedC, _ := strconv.ParseUint(channelID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedC)

	return buf
}

func KeyChannelMessage(channelID, messageID string) []byte {

	// 1 keytype, 8 channelID, 8 messageID
	buf := make([]byte, 17)
	buf[0] = byte(KeyTypeChannelMessage)

	parsedC, _ := strconv.ParseUint(channelID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedC)

	parsedM, _ := strconv.ParseUint(messageID, 10, 64)
	binary.BigEndian.PutUint64(buf[9:], parsedM)

	return buf
}

func KeyChannelMessageIteratorPrefix(channelID string) []byte {
	// 1 keytype, 8 channelID
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeChannelMessage)

	parsedC, _ := strconv.ParseUint(channelID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedC)

	return buf
}

func KeyPresence(userID string) []byte {
	// 1 keytype, 8 channelID
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypePresence)

	parsedU, _ := strconv.ParseUint(userID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedU)

	return buf
}

func KeyVoiceState(guildID, userID string) []byte {
	// 1 keytype, 8 guildID, 8 userID
	buf := make([]byte, 17)
	buf[0] = byte(KeyTypeVoiceState)

	parsedG, _ := strconv.ParseUint(guildID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedG)

	parsedU, _ := strconv.ParseUint(userID, 10, 64)
	binary.BigEndian.PutUint64(buf[9:], parsedU)

	return buf
}

func KeyVoiceStateIteratorPrefix(guildID string) []byte {
	// 1 keytype, 8 guildID
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeVoiceState)

	parsedG, _ := strconv.ParseUint(guildID, 10, 64)
	binary.BigEndian.PutUint64(buf[1:], parsedG)

	return buf
}
