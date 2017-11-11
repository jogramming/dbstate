package dbstate

import (
	"github.com/bwmarrin/discordgo"
	"strconv"
	"testing"
)

func TestGuilds(t *testing.T) {
	g := &discordgo.Guild{
		ID:          "123",
		Name:        "some fun name",
		Large:       true,
		MemberCount: 10,
	}

	err := testWorker.GuildCreate(g)
	AssertFatal(t, err, "failed handling guild create")

	g2, err := testState.Guild(nil, "123")
	AssertFatal(t, err, "failed calling Guild(ID)")

	if g2.ID != g.ID || g2.Name != g.Name || g2.Large != g.Large || g2.MemberCount != g.MemberCount {
		t.Errorf("stored guild was modified: correct(%#v) stored(%#v)", g, g2)
	}

	// Test iterating
	cnt := 0
	err = testState.IterateGuilds(nil, func(g *discordgo.Guild) bool {
		cnt++
		return true
	})
	AssertFatal(t, err, "failed iterating guilds")

	if cnt != 1 {
		t.Errorf("unexpted guild count! Got: %d, Expected: 1", cnt)
	}

	err = testWorker.GuildDelete("123")
	AssertFatal(t, err, "failed removing guild")

	if _, err := testState.Guild(nil, "123"); err == nil {
		t.Fatal("guild still there after being deleted")
	}
}

func TestGuildMembers(t *testing.T) {
	m := &discordgo.Member{
		User:    &discordgo.User{ID: "123"},
		GuildID: "321",
		Nick:    "some fun name",
		Roles:   []string{"123", "321"},
	}

	err := testWorker.MemberUpdate(nil, m)
	AssertFatal(t, err, "failed handling member update")

	m2, err := testState.GuildMember(nil, "321", "123")
	AssertFatal(t, err, "failed calling GuildMember(gid, uid)")

	if m2.User.ID != m.User.ID || m2.Nick != m.Nick || len(m2.Roles) != len(m.Roles) || m2.GuildID != m.GuildID {
		t.Errorf("stored member was modified: correct(%#v) stored(%#v)", m, m2)
	}

	for i, v := range m.Roles {
		if m2.Roles[i] != v {
			t.Errorf("mismatched roles, Got: %q, Expected: %q", m2.Roles[i], v)
		}
	}

	cnt := 0
	err = testState.IterateGuildMembers(nil, "321", func(g *discordgo.Member) bool {
		cnt++
		return true
	})

	AssertFatal(t, err, "failed iterating members")

	if cnt != 1 {
		t.Errorf("unexpted member count! Got: %d, Expected: 1", cnt)
	}

	err = testWorker.MemberRemove(nil, "321", "123", false)
	AssertFatal(t, err, "failed removing member")

	if _, err = testState.GuildMember(nil, "321", "123"); err == nil {
		t.Fatal("member still there after being removed")
	}
}

func TestGuildChannels(t *testing.T) {
	c := &discordgo.Channel{
		GuildID: "1",
		ID:      "2",
		Type:    discordgo.ChannelTypeGuildText,
	}
	g := &discordgo.Guild{
		ID: "1",
	}

	AssertFatal(t, testWorker.GuildCreate(g), "failed creating guild")
	AssertFatal(t, testWorker.ChannelCreateUpdate(nil, c, true), "failed creating channel")

	fetched, err := testState.Channel(nil, "2")
	AssertFatal(t, err, "failed retrieving channel")

	if fetched.ID != c.ID || fetched.GuildID != c.GuildID || fetched.Type != c.Type {
		t.Errorf("mismatched results, got %#v, expected %#v", fetched, c)
	}

	AssertFatal(t, testWorker.ChannelDelete(nil, "2"), "failed deleting channel")
	if _, err = testState.Channel(nil, "2"); err == nil {
		t.Fatal("channel still there after being removed")
	}
	AssertErr(t, testWorker.GuildDelete("1"), "failed removing guild")
}

func TestGuildRoles(t *testing.T) {
	r := &discordgo.Role{
		ID:    "2",
		Color: 100,
		Name:  "Hello there",
	}

	g := &discordgo.Guild{
		ID: "1",
	}

	AssertFatal(t, testWorker.GuildCreate(g), "failed creating guild")
	AssertFatal(t, testWorker.RoleCreateUpdate(nil, g.ID, r), "failed creating role")

	gFetched, err := testState.Guild(nil, g.ID)
	AssertFatal(t, err, "failed retrieving guild")

	rFetched := gFetched.FindRole(r.ID)

	// fetched, err := testState.Channel(nil, "2")

	if rFetched.ID != r.ID || rFetched.Color != r.Color || rFetched.Name != r.Name {
		t.Errorf("mismatched results, got %#v, expected %#v", rFetched, r)
	}

	AssertFatal(t, testWorker.RoleDelete(nil, g.ID, r.ID), "failed deleting role")

	gFetched, err = testState.Guild(nil, g.ID)
	AssertFatal(t, err, "failed retrieving guild")

	if gFetched.FindRole(r.ID) != nil {
		t.Error("role still in state after being deleted")
	}

	AssertErr(t, testWorker.GuildDelete(g.ID), "failed removing guild")
}

func TestGuildEmojis(t *testing.T) {
	e := &discordgo.Emoji{
		ID:   "7",
		Name: "Hello there",
	}

	g := &discordgo.Guild{
		ID: "1",
	}

	AssertFatal(t, testWorker.GuildCreate(g), "failed creating guild")
	AssertFatal(t, testWorker.EmojisUpdate(nil, g.ID, []*discordgo.Emoji{e}), "failed creating emoji")

	gFetched, err := testState.Guild(nil, g.ID)
	AssertFatal(t, err, "failed retrieving guild")

	eFetched := gFetched.FindEmoji(e.ID)

	if eFetched.ID != e.ID || eFetched.Name != e.Name {
		t.Errorf("mismatched results, got %#v, expected %#v", eFetched, e)
	}

	AssertErr(t, testWorker.GuildDelete(g.ID), "failed removing guild")
}

func TestChannelMessages(t *testing.T) {
	m := &discordgo.Message{
		ID:        "3",
		ChannelID: "2",
		Content:   "Hello there",
		Author: &discordgo.User{
			ID:       "5",
			Username: "bob",
		},
	}

	AssertFatal(t, testWorker.MessageCreateUpdate(nil, m), "failed creating message")

	fetched, err := testState.ChannelMessage(nil, m.ChannelID, m.ID)
	AssertFatal(t, err, "failed retrieving message")

	if fetched.ID != m.ID || fetched.Content != m.Content || fetched.Author.Username != m.Author.Username || fetched.Author.ID != m.Author.ID {
		t.Errorf("mismatched results, got %#v, expected %#v", fetched, m)
	}

	AssertFatal(t, testWorker.MessageDelete(nil, m.ChannelID, m.ID), "failed deleting message")

	if _, err := testState.ChannelMessage(nil, m.ChannelID, m.ID); err == nil {
		t.Error("message still exists after being removed from state")
	}
}

func TestPresences(t *testing.T) {
	p := &discordgo.Presence{
		Nick: "boiman",
		User: &discordgo.User{
			ID:       "5",
			Username: "bob",
		},
	}

	AssertFatal(t, testWorker.PresenceAddUpdate(nil, false, p), "failed creating presence")

	fetched, err := testState.Presence(nil, p.User.ID)
	AssertFatal(t, err, "failed retrieving presence")

	if fetched.User.ID != p.User.ID || fetched.Nick != p.Nick || fetched.User.Username != p.User.Username {
		t.Errorf("mismatched results, got %#v, expected %#v", fetched, p)
	}
}

func TestVoiceStates(t *testing.T) {
	vs := &discordgo.VoiceState{
		GuildID:   "1",
		UserID:    "5",
		ChannelID: "2",
		Mute:      true,
	}

	AssertFatal(t, testWorker.VoiceStateUpdate(nil, vs), "failed creating voicestate")

	fetched, err := testState.VoiceState(nil, vs.GuildID, vs.UserID)
	AssertFatal(t, err, "failed retrieving voice state")

	if fetched.UserID != vs.UserID || fetched.GuildID != vs.GuildID || fetched.Mute != vs.Mute {
		t.Errorf("mismatched results, got %#v, expected %#v", fetched, vs)
	}
}

func BenchmarkPresenceUpdates(b *testing.B) {
	badgerOpts := RecommendedBadgerOptions("bench_db")

	opts := Options{
		DBOpts: badgerOpts,
	}

	state, err := NewState(1, opts)
	if err != nil {
		panic(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := &discordgo.Presence{
			User: &discordgo.User{
				ID:       strconv.Itoa(i),
				Username: "bob",
			},
			Status: "online",
			Nick:   "billy",
		}

		err := state.shards[0].PresenceAddUpdate(nil, false, p)
		if err != nil {
			panic(err)
		}
	}
	b.StopTimer()
	state.Close()
}
