package dbstate

import (
	"bytes"
	"fmt"
	"github.com/json-iterator/go"
	// "github.com/alecthomas/binary"
	"github.com/bwmarrin/discordgo"
	"testing"
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

	badgerOpts := RecommendedBadgerOptions("")
	opts := Options{
		DBOpts: badgerOpts,
	}

	state, err := NewState(1, opts)
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

	encoded, err := testState.encodeData(nil, nil, m)
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

func BenchmarkEncode(b *testing.B) {
	testStruct := &discordgo.Presence{
		User: &discordgo.User{
			ID:       "123",
			Username: "bob",
		},
		Status: "online",
		Nick:   "billy",
	}

	for i := 0; i < b.N; i++ {
		_, err := (*State)(nil).encodeData(nil, nil, testStruct)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkEncodeReuseBufEnc(b *testing.B) {
	testStruct := &discordgo.Presence{
		User: &discordgo.User{
			ID:       "123",
			Username: "bob",
		},
		Status: "online",
		Nick:   "billy",
	}

	var buf bytes.Buffer
	encoder := jsoniter.NewEncoder(&buf)

	for i := 0; i < b.N; i++ {

		_, err := (*State)(nil).encodeData(&buf, encoder, testStruct)
		if err != nil {
			panic(err)
		}

		buf.Reset()
	}
}
