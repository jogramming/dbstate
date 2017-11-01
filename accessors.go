package dbstate

import (
	"github.com/bwmarrin/discordgo"
)

func (s *State) Guild(id string) (*discordgo.Guild, error) {
	var dest *discordgo.Guild
	err := s.GetKey(nil, KeyGuild(id), &dest)
	return dest, err
}
