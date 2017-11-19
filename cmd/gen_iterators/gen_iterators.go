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
	Name            string
	Key             string
	ExtraArgs       []Arg
	CBMeta          string
	DestType        string
	IteratorOptions string
	Seek            string
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
		Name:     "IteratePresences",
		Key:      "[]byte{byte(KeyTypePresence)}",
		DestType: "*discordgo.Presence",
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
		CBMeta:    "MessageFlag",
	},
	Item{
		Name:      "IterateChannelMessagesNewerFirst",
		ExtraArgs: []Arg{{Name: "channelID", Type: "string"}},
		Key:       `KeyChannelMessageIteratorPrefix(channelID)`,
		DestType:  "*discordgo.Message",
		CBMeta:    "MessageFlag",
		IteratorOptions: `
	opts.Reverse = true
`,
		Seek: `
	seek = make([]byte, 17)
	copy(seek, prefix)
	// Seek to the last possible message in this channel
	seek[9] = 0xff
	seek[10] = 0xff
	seek[11] = 0xff
	seek[12] = 0xff
	seek[13] = 0xff
	seek[14] = 0xff
	seek[15] = 0xff
	seek[16] = 0xff`,
	},
	Item{
		Name:     "IterateAllMessages",
		Key:      "[]byte{byte(KeyTypeChannelMessage)}",
		DestType: "*discordgo.Message",
		CBMeta:   "MessageFlag",
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
func (s *State) {{.Name}}(txn *badger.Txn, {{range .ExtraArgs}}{{.Name}} {{.Type}}, {{end}}f func({{if .CBMeta}}m {{.CBMeta}}, {{end}}d {{.DestType}}) bool) error {
	if txn == nil {
		return s.DB.View(func(txn *badger.Txn) error {
			return s.{{.Name}}(txn, {{range .ExtraArgs}}{{.Name}}, {{end}}f)
		})
	}

	// Scan over the prefix
	prefix := {{.Key}}
	seek := prefix{{.Seek}}

	opts := badger.DefaultIteratorOptions{{.IteratorOptions}}
	it := txn.NewIterator(opts)
	for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return err
		}

		var dest {{.DestType}}
		err = s.DecodeData(v, &dest)
		if err != nil {
			return err
		}{{if .CBMeta}}

		meta := {{.CBMeta}}(item.UserMeta()){{end}}

		// Call the callback
		if !f({{if .CBMeta}}meta, {{end}}dest) {
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
