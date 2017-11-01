package dbstate

import (
	"github.com/bwmarrin/discordgo"
	"testing"
)

func TestGuildCreate(t *testing.T) {
	g := &discordgo.Guild{
		ID:          "123",
		Name:        "some fun name",
		Large:       true,
		MemberCount: 10,
	}

	err := testState.HandleGuildCreate(0, g)
	if err != nil {
		t.Fatal("Failed handling guild create: ", err)
	}

	g2, err := testState.Guild("123")
	if err != nil {
		t.Fatal("Failed calling guild(id): ", err)
	}

	if g2.ID != g.ID || g2.Name != g.Name || g2.Large != g.Large || g2.MemberCount != g.MemberCount {
		t.Errorf("Stored guild was modified: correct(%#v) stored(%#v)", g, g2)
	}
}
