package dbstate

import (
	"github.com/bwmarrin/discordgo"
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

	// if _, err = testState.Channel(nil, "2"); err == nil {
	// 	t.Fatal("role still there after being removed")
	// }

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
