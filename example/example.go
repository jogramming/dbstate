package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dbstate"
	"github.com/jonas747/dshardmanager"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	State *dbstate.State
)

func main() {
	manager := dshardmanager.New(os.Getenv("DG_TOKEN"))
	recommended, err := manager.GetRecommendedCount()
	if err != nil {
		log.Fatal("Error retrieving recommended shard count: ", err)
	}

	opts := dbstate.Options{
		DBOpts:             dbstate.RecommendedBadgerOptions("state"),
		UseChannelSyncMode: true,

		TrackChannels:  true,
		TrackMembers:   true,
		TrackMessages:  true,
		TrackPresences: true,
		TrackRoles:     true,
	}

	state, err := dbstate.NewState(recommended, opts)
	if err != nil {
		log.Fatal("Failed initializing state: ", err)
	}

	State = state

	eventCounter := new(int64)

	manager.AddHandler(func(session *discordgo.Session, evt interface{}) {
		if _, ok := evt.(*discordgo.Event); ok {
			// Fast path this since this is sent for every single event
			return
		}

		atomic.AddInt64(eventCounter, 1)

		state.HandleEventChannelSynced(session.ShardID, evt)
	})

	go printStats(eventCounter)

	manager.SessionFunc = func(token string) (*discordgo.Session, error) {
		session, err := discordgo.New(token)
		if err != nil {
			return nil, err
		}
		session.StateEnabled = false
		session.SyncEvents = true
		// session.LogLevel = discordgo.LogDebug
		return session, nil
	}

	go func() {
		// Fun handlers to inspect the state while running, do not run on production because these are expensive
		http.HandleFunc("/gs", HandleGuildSize)
		http.HandleFunc("/g", HandleIterateGuilds)
		http.HandleFunc("/msgs", HandleIterateMessages)
		log.Fatal(http.ListenAndServe(":7441", nil))
	}()

	err = manager.Start()
	if err != nil {
		log.Fatal("Failed starting shard manager: ", err)
	}

	log.Println("Running....")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	manager.StopAll()
	state.Close()
	log.Println("Bye")
}

func printStats(counter *int64) {
	ticker := time.NewTicker(time.Second)
	for {
		<-ticker.C

		current := atomic.SwapInt64(counter, 0)
		logrus.Info("Handled ", current, " events the last second")
	}
}

func HandleGuildSize(w http.ResponseWriter, r *http.Request) {
	gId := r.URL.Query().Get("guild")
	if gId == "" {
		return
	}

	g, err := State.Guild(nil, gId)
	if err != nil {
		w.Write([]byte(err.Error() + ": " + gId))
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

func HandleIterateGuilds(w http.ResponseWriter, r *http.Request) {
	count := int64(0)
	State.IterateGuilds(nil, func(g *discordgo.Guild) bool {
		w.Write([]byte(g.ID + ": " + g.Name + "\n"))
		count++
		return true
	})
	w.Write([]byte("Total: " + strconv.FormatInt(count, 10) + "\n"))
}

func HandleIterateMessages(w http.ResponseWriter, r *http.Request) {
	cID := r.URL.Query().Get("channel")
	if cID == "" {
		return
	}

	count := int64(0)
	State.IterateChannelMessages(nil, cID, func(m *discordgo.Message) bool {
		w.Write([]byte(m.ID + ": " + m.Author.Username + ": " + m.ContentWithMentionsReplaced() + "\n"))
		count++
		return true
	})
	w.Write([]byte("Total: " + strconv.FormatInt(count, 10) + "\n"))
}
