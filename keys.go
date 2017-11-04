package dbstate

const (
	KeySelfUser = "selfuser"

	KeyGuildsIteratorPrefix = "guilds:"
)

func KeyUser(userID string) string                        { return "users:" + userID }
func KeyGuild(guildID string) string                      { return "guilds:" + guildID }
func KeyGuildMemberCount(guildID string) string           { return "gm_count:" + guildID }
func KeyGuildMember(guildID, userID string) string        { return "guild_members:" + guildID + ":" + userID }
func KeyGuildMembersIteratorPrefix(guildID string) string { return "guild_members:" + guildID + ":" }
func KeyChannel(channelID string) string                  { return "channels:" + channelID }
func KeyChannelMessage(channelID, messageID string) string {
	return "messages:" + channelID + ":" + messageID
}
func KeyChannelMessageIteratorPrefix(channelID string) string { return "messages:" + channelID + ":" }
