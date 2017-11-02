package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dbstate"
	"github.com/jonas747/dshardmanager"
	"log"
	"net/http"
	"os"
)

var (
	State *dbstate.State
)

func main() {
	manager := dshardmanager.New(os.Getenv("DG_TOKEN"))
	reccomended, err := manager.GetRecommendedCount()
	if err != nil {
		log.Fatal("Error retrieving reccomended shard count: ", err)
	}

	state, err := dbstate.NewState("", reccomended)
	if err != nil {
		log.Fatal("Failed initializing state: ", err)
	}
	State = state

	manager.AddHandler(state.HandleEvent)

	manager.SessionFunc = func(token string) (*discordgo.Session, error) {
		session, err := discordgo.New(token)
		if err != nil {
			return nil, err
		}
		session.StateEnabled = false
		session.SyncEvents = true
		return session, nil
	}

	err = manager.Start()
	if err != nil {
		log.Fatal("Failed starting shard manager: ", err)
	}

	log.Println("Running....")
	log.Fatal(http.ListenAndServe(":7441", nil))
}
