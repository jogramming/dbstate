package main

import (
	"flag"
	"log"
	"os"
	"text/template"
)

func main() {
	outFile := flag.String("out", "gen.go", "Oupput file")
	flag.Parse()

	file, err := os.Create(*outFile)
	if err != nil {
		log.Fatal("Failed creating output file ", err)
	}

	err = Gen(file)
	file.Close()
	if err != nil {
		log.Fatal("Failed generating: ", err)
	}
}

type Item struct {
	Name      string
	Key       string
	ExtraArgs []Arg
	DestType  string
}

type Arg struct {
	Name string
	Type string
}

var Iterators = []Item{
	Item{
		Name:     "IterateGuilds",
		Key:      "[]byte{byte(KeyTypeGuild)}",
		DestType: "*discordgo.Guild",
	},
	Item{
		Name:      "IterateGuildMembers",
		ExtraArgs: []Arg{{Name: "guildID", Type: "string"}},
		Key:       "KeyGuildMembersIteratorPrefix(guildID)",
		DestType:  "*discordgo.Member",
	},
	Item{
		Name:      "IterateChannelMessages",
		ExtraArgs: []Arg{{Name: "channelID", Type: "string"}},
		Key:       "KeyChannelMessageIteratorPrefix(channelID)",
		DestType:  "*discordgo.Message",
	},
	Item{
		Name:     "IterateAllMessages",
		Key:      "[]byte{byte(KeyTypeChannelMessage)}",
		DestType: "*discordgo.Message",
	},
	Item{
		Name:      "IterateGuildVoiceStates",
		ExtraArgs: []Arg{{Name: "guildID", Type: "string"}},
		Key:       "KeyVoiceStateIteratorPrefix(guildID)",
		DestType:  "*discordgo.VoiceState",
	},
}

const (
	RawTemplate = `package dbstate

import (
	"github.com/bwmarrin/discordgo"
	"github.com/dgraph-io/badger"
)
{{range .}}
// {{.Name}} Iterates over all {{.DestType}} in state, calling f on them
// if f returns false then iteration will stop
func (s *State) {{.Name}}(txn *badger.Txn, {{range .ExtraArgs}}{{.Name}} {{.Type}}, {{end}}f func(d {{.DestType}}) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.{{.Name}}(txn, {{range .ExtraArgs}}{{.Name}}, {{end}}f)
		})
	}

	// Scan over the prefix
	prefix := {{.Key}}

	opts := badger.DefaultIteratorOptions
	it := txn.NewIterator(opts)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return err
		}

		var dest {{.DestType}}
		err = s.DecodeData(v, &dest)
		if err != nil {
			return err
		}

		// Call the callback
		if !f(dest) {
			break
		}
	}
	return nil
}
{{end}}
`
)

var Tmpl = template.Must(template.New("").Parse(RawTemplate))

func Gen(file *os.File) error {
	err := Tmpl.Execute(file, Iterators)
	return err
}
