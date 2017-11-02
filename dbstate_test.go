package dbstate

import (
	"github.com/bwmarrin/discordgo"
	"testing"
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

func TestEncodeDecode(t *testing.T) {
	m := &discordgo.Member{
		User:    &discordgo.User{ID: "123"},
		GuildID: "321",
	}

	encoded, err := testState.encodeData(0, m)
	if err != nil {
		t.Error("Enc: ", err)
		return
	}

	var dest discordgo.Member
	err = testState.DecodeData(encoded, &dest)
	if err != nil {
		t.Error("Dec: ", err)
	}

	if m.User.ID != dest.User.ID || m.GuildID != dest.GuildID {
		t.Errorf("Mismatched results: got %#v, Exptected: %#v", dest, m)
	}
}
