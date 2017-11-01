package dbstate

const (
	KeySelfUser = "selfuser"
)

func KeyUser(userID string) string   { return "users:" + userID }
func KeyGuild(guildID string) string { return "guilds:" + guildID }
