package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
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

	state, err := dbstate.NewState("", reccomended, true)
	if err != nil {
		log.Fatal("Failed initializing state: ", err)
	}
	State = state

	manager.AddHandler(state.HandleEventChannelSynced)

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
	http.HandleFunc("/gs", HandleGuildSize)
	log.Fatal(http.ListenAndServe(":7441", nil))
}

func HandleGuildSize(w http.ResponseWriter, r *http.Request) {
	gId := r.URL.Query().Get("guild")
	if gId == "" {
		return
	}

	g, err := State.Guild(nil, gId)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	buffer := new(bytes.Buffer)
	encoder := gob.NewEncoder(buffer)

	err = encoder.Encode(g)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	encoded := make([]byte, buffer.Len())
	_, err = buffer.Read(encoded)

	buffer.Reset()
	if err != nil {
		return
	}

	w.Write([]byte(fmt.Sprintf("Size of guild %s: %d bytes", g.Name, len(encoded))))
}
