package cyborg

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"github.com/bwmarrin/discordgo"
)

type incoming_message struct {
	s        *discordgo.Session
	m        *discordgo.MessageCreate
	isDirect bool
}

type talker struct {
	version string
	operators        map[string]int
	incomingMessages chan *incoming_message
}


func newTalker(ops []string, ver string) * talker {
	t := &talker {
		version: ver,
		operators: make(map[string]int),
		incomingMessages: make(chan *incoming_message),
	}
	for _, op := range ops {
		t.operators[op] = 1
	}
	return t
}

func (t *talker) start() {
	go t.handleIncomingMessages()
	go t.handleIncomingMessages()
}

func (t *talker) close() {
	close(t.incomingMessages)
}

func (t *talker) handleIncomingMessages() {
	log.Printf("handleIncomingMessages started...\n")
	for {
		msg, more := <-t.incomingMessages
		if more {
			t.handleIncomingMessage(msg)
		} else {
			break
		}
	}
	fmt.Println("handleIncomingMessages finished")
}

func (t *talker) handleIncomingMessage(im *incoming_message) {
	log.Printf("Content: '%s'\n", im.m.Content)
	re := regexp.MustCompile(`<@\d+>`)
	ctx := strings.TrimSpace(re.ReplaceAllString(im.m.Content, ""))
	log.Printf("Stripped: '%s'\n", ctx)

	if ctx == "ping" {
		SendMessage(im.s, im.m.ChannelID, "Pong!")
		return
	}

	// If the message is "pong" reply with "Ping!"
	if ctx == "pong" {
		SendMessage(im.s, im.m.ChannelID, "Ping!")
		return
	}

	if ctx == "sleep" {
		SendMessage(im.s, im.m.ChannelID, "about to sleep...")
		time.Sleep(10 * time.Second)
		SendMessage(im.s, im.m.ChannelID, "i'm ready")
		return
	}

	if ctx == "version" {
		SendQuotedMessage(im.s, im.m, ctx, t.version)
		return
	}
}

func (t *talker) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	self := s.State.User.ID
	if m.Author.ID == self {
		return
	}
	log.Printf("Channel : %s, mentions %d\n", m.ChannelID, len(m.Mentions))
	mentioned := false
	for _, u := range m.Mentions {
		log.Printf(" mentioned: %s self %s name %s\n", u.ID, self, u.Username)
		if u.ID == self {
			mentioned = true
			break
		}
	}

	direct := false
	if !mentioned {
		if channel, err := s.State.Channel(m.ChannelID); err != nil {
			log.Printf("Failed on checking channel %s\n", m.ChannelID)
			// well, not mentioned, strange channel -- giveup
			return
		} else {
			if channel.Type != discordgo.ChannelTypeDM {
				return
			}
		}
		log.Printf("Got a message in a private channel\n")
		direct = true
	}

	mm := incoming_message{s, m, direct}

	select {
	case t.incomingMessages <- &mm:
	default:
		SendQuotedMessage(s, m, m.Content, "Oops. Sorry, I'm busy now. Try later.")
	}
}

func SendQuotedMessage(s *discordgo.Session, m *discordgo.MessageCreate, quote string, message string) (*discordgo.Message, error) {
	msg := fmt.Sprintf("`%s> %s`\n%s", m.Author.Username, quote, message)
	return s.ChannelMessageSend(m.ChannelID, msg)
}

func SendMessage(s *discordgo.Session, channelId string, message string) (*discordgo.Message, error) {
	return s.ChannelMessageSend(channelId, message)
}


