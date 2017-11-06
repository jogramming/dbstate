package dbstate

import (
	"encoding/binary"
	"strconv"
)

const (
// KeyGuildsIteratorPrefix = "guilds:"
)

var (
	KeySelfUser = []byte{'q'}
)

type KeyType byte

const (
	KeyTypeGuild          KeyType = 'g'
	KeyTypeMember         KeyType = 'm'
	KeyTypeChannel        KeyType = 'c'
	KeyTypeChannelMessage KeyType = 's'
)

func KeyUser(userID string) string { return "users:" + userID }
func KeyGuild(guildID string) []byte {

	// 0 keytype, 8 id
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeGuild)

	parsed, _ := strconv.ParseUint(guildID, 10, 64)
	binary.LittleEndian.PutUint64(buf[1:], parsed)

	return buf
}

func KeyGuildMember(guildID, userID string) []byte {

	// 1 keytype, 8 guildID, 8 userID = 17
	buf := make([]byte, 17)
	buf[0] = byte(KeyTypeMember)

	parsedG, _ := strconv.ParseUint(guildID, 10, 64)
	binary.LittleEndian.PutUint64(buf[1:], parsedG)

	parsedM, _ := strconv.ParseUint(userID, 10, 64)
	binary.LittleEndian.PutUint64(buf[9:], parsedM)

	return buf
}

func KeyGuildMembersIteratorPrefix(guildID string) []byte {
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeMember)

	parsedG, _ := strconv.ParseUint(guildID, 10, 64)
	binary.LittleEndian.PutUint64(buf[1:], parsedG)

	return buf
}

func KeyChannel(channelID string) []byte {
	// 1 keytype, 8 channelID
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeChannel)

	parsedC, _ := strconv.ParseUint(channelID, 10, 64)
	binary.LittleEndian.PutUint64(buf[1:], parsedC)

	return buf
}

func KeyChannelMessage(channelID, messageID string) []byte {

	// 1 keytype, 8 channelID, 8 messageID
	buf := make([]byte, 17)
	buf[0] = byte(KeyTypeChannelMessage)

	parsedC, _ := strconv.ParseUint(channelID, 10, 64)
	binary.LittleEndian.PutUint64(buf[1:], parsedC)

	parsedM, _ := strconv.ParseUint(messageID, 10, 64)
	binary.LittleEndian.PutUint64(buf[9:], parsedM)

	return buf
}

func KeyChannelMessageIteratorPrefix(channelID string) []byte {
	// 1 keytype, 8 channelID
	buf := make([]byte, 9)
	buf[0] = byte(KeyTypeChannelMessage)

	parsedC, _ := strconv.ParseUint(channelID, 10, 64)
	binary.LittleEndian.PutUint64(buf[1:], parsedC)

	return buf
}
