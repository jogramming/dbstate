package dbstate

import (
	"github.com/bwmarrin/discordgo"
	// "testing"
)

var (
	testState   *State
	mockSession = &discordgo.Session{
		ShardCount: 1,
	}
)

func init() {
	testState = SetupTestState()
}

func SetupTestState() *State {
	state, err := NewState("testing_db", 1)
	if err != nil {
		panic(err)
	}

	return state
}
