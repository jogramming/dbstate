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

	err := testState.GuildCreate(0, g)
	if err != nil {
		t.Fatal("Failed handling guild create: ", err)
	}

	g2, err := testState.Guild(nil, "123")
	if err != nil {
		t.Fatal("Failed calling guild(id): ", err)
	}

	if g2.ID != g.ID || g2.Name != g.Name || g2.Large != g.Large || g2.MemberCount != g.MemberCount {
		t.Errorf("Stored guild was modified: correct(%#v) stored(%#v)", g, g2)
	}

	// Test iterating
	cnt := 0
	err = testState.IterateGuilds(nil, func(g *discordgo.Guild) bool {
		cnt++
		return true
	})
	if err != nil {
		t.Fatal("Failed iterating guilds: ", err)
	}

	if cnt != 1 {
		t.Errorf("Unexpted guild count! Got: %d, Expected: 1", cnt)
	}

	err = testState.GuildDelete("123")
	if err != nil {
		t.Fatal("Failed removing guild: ", err)
	}

	if _, err := testState.Guild(nil, "123"); err == nil {
		t.Fatal("Guild still there after being deleted")
	}
}

func TestGuildMembers(t *testing.T) {
	m := &discordgo.Member{
		User:    &discordgo.User{ID: "123"},
		GuildID: "321",
		Nick:    "some fun name",
		Roles:   []string{"123", "321"},
	}

	err := testState.MemberUpdate(0, nil, m)
	if err != nil {
		t.Fatal("Failed handling member update: ", err)
	}

	m2, err := testState.GuildMember(nil, "321", "123")
	if err != nil {
		t.Fatal("Failed calling GuildMember(id): ", err)
	}

	if m2.User.ID != m.User.ID || m2.Nick != m.Nick || len(m2.Roles) != len(m.Roles) || m2.GuildID != m.GuildID {
		t.Errorf("Stored member was modified: correct(%#v) stored(%#v)", m, m2)
	}

	for i, v := range m.Roles {
		if m2.Roles[i] != v {
			t.Errorf("Mismatched roles, Got: %q, Expected: %q", m2.Roles[i], v)
		}
	}

	cnt := 0
	err = testState.IterateGuildMembers(nil, "321", func(g *discordgo.Member) bool {
		cnt++
		return true
	})

	if err != nil {
		t.Fatal("Failed iterating guilds: ", err)
	}

	if cnt != 1 {
		t.Errorf("Unexpted member count! Got: %d, Expected: 1", cnt)
	}

	err = testState.MemberRemove(0, nil, "321", "123", false)
	if err != nil {
		t.Fatal("Error removing member: ", err)
	}

	if _, err = testState.GuildMember(nil, "321", "123"); err == nil {
		t.Fatal("Member still there after being removed")
	}

}
