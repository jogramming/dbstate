package dbstate

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"testing"
	// "testing"
)

var (
	testState   *State
	testWorker  *shardWorker
	mockSession = &discordgo.Session{
		ShardCount: 1,
	}
)

func init() {
	testState = SetupTestState()
}

func SetupTestState() *State {
	state, err := NewState("testing_db", 1, false)
	if err != nil {
		panic(err)
	}

	testWorker = state.shards[0]

	return state
}

func TestEncodeDecode(t *testing.T) {
	m := &discordgo.Member{
		User:    &discordgo.User{ID: "123"},
		GuildID: "321",
	}

	encoded, err := testWorker.encodeData(m)
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

func AssertFatal(t *testing.T, err error, msg ...interface{}) {
	if err != nil {
		t.Fatal(fmt.Sprint(msg...) + ": " + err.Error())
	}
}

func AssertErr(t *testing.T, err error, msg ...interface{}) {
	if err != nil {
		t.Error(fmt.Sprint(msg...) + ": " + err.Error())
	}
}
