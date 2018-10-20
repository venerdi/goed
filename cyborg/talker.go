package cyborg

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"goed/pkg/edGalaxy"
	"log"
	"regexp"
	"strconv"
	"strings"
	"text/scanner"
)

type incoming_message struct {
	s        *discordgo.Session
	m        *discordgo.MessageCreate
	isDirect bool
}

type talker struct {
	version          string
	operators        map[string]int
	incomingMessages chan *incoming_message
	galaxyInfoCenter *edGalaxy.GalaxyInfoCenter
	ignoredSystems   map[string]bool
}

func newTalker(ops []string, ver string, galaxyInfoCenter *edGalaxy.GalaxyInfoCenter, ignoredSystems []string) *talker {
	t := &talker{
		version:          ver,
		operators:        make(map[string]int),
		incomingMessages: make(chan *incoming_message),
		galaxyInfoCenter: galaxyInfoCenter,
		ignoredSystems:   make(map[string]bool),
	}
	for _, op := range ops {
		t.operators[op] = 1
	}
	for _, sn := range ignoredSystems {
		t.ignoredSystems[sn] = true
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

	if ctx == "version" {
		SendQuotedMessage(im.s, im.m, ctx, t.version)
		return
	}

	if strings.HasPrefix(ctx, "system ") {
		t.handleSystemRequest(im.s, im.m.ChannelID, ctx[7:])
		return
	}
	if strings.HasPrefix(ctx, "distance ") {
		t.handleDistanceRequest(im.s, im.m.ChannelID, ctx[9:])
		return
	}
	if _, op := t.operators[im.m.Author.ID]; im.isDirect && op {
		t.handleDirectOperatorMessage(im)
	}
}

func (t *talker) handleDirectOperatorMessage(im *incoming_message) {
	var s scanner.Scanner
	s.Init(strings.NewReader(im.m.Content))
	tokens := make([]string, 0, 5)
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		tokens = append(tokens, s.TokenText())
	}
	if len(tokens) == 0 {
		SendMessage(im.s, im.m.ChannelID, "hmm... no tokens")
		return
	}

	switch strings.ToLower(tokens[0]) {
	case "ls":
		t.handleOperatorLS(im, tokens[1:])
	case "say":
		t.handleOperatorSay(im, tokens[1:])
	default:
		SendMessage(im.s, im.m.ChannelID, "Unknown command")
		return
	}
}

func (t *talker) handleOperatorSay(im *incoming_message, tokens []string) {
	if len(tokens) < 2 {
		SendMessage(im.s, im.m.ChannelID, "syntax is: say channeldId message")
		return
	}
	idx := strings.Index(im.m.Content, tokens[0])
	if idx < 0 {
		SendMessage(im.s, im.m.ChannelID, "Say: can't find channelId")
		return
	}
	stripSize := idx + len(tokens[0]) + 1
	if len(im.m.Content) <= stripSize {
		SendMessage(im.s, im.m.ChannelID, "Say: empty message")
	}
	SendMessage(im.s, tokens[0], im.m.Content[stripSize:])
}

func (t *talker) handleOperatorLS(im *incoming_message, tokens []string) {
	if len(tokens) == 0 {
		SendMessage(im.s, im.m.ChannelID, "syntax is: ls category")
		return
	}
	switch strings.ToLower(tokens[0]) {
	case "channels":
		t.handleOperatorLSchannels(im)
	default:
		SendMessage(im.s, im.m.ChannelID, "Unknown ls category")
	}
}

func (t *talker) handleOperatorLSchannels(im *incoming_message) {

	channelsInfo := make([]string, 0, 50)

	channelsInfo = append(channelsInfo, "```")

	for _, g := range im.s.State.Guilds {
		channelsInfo = append(channelsInfo, fmt.Sprintf("%s %s", g.Name, g.ID))
		for _, c := range g.Channels {
			var tp string
			switch c.Type {
			case discordgo.ChannelTypeGuildText:
				tp = "text"
			case discordgo.ChannelTypeGuildVoice:
				tp = "voice"
			default:
				continue
			}
			channelsInfo = append(channelsInfo, fmt.Sprintf("    %s %s (%v)", c.ID, c.Name, tp))
		}
	}
	channelsInfo = append(channelsInfo, "```")
	out := strings.Join(channelsInfo, "\n")
	SendMessage(im.s, im.m.ChannelID, out)
}

func (t *talker) handleSystemRequest(ds *discordgo.Session, channelID string, systemName string) {

	if len(systemName) < 2 {
		SendMessage(ds, channelID, "System name must be at least 2 chars")
		return
	}

	if _, ignored := t.ignoredSystems[strings.ToLower(systemName)]; ignored {
		SendMessage(ds, channelID, fmt.Sprintf("%s is a permit locked system", systemName))
		return
	}

	ch := make(edGalaxy.SystemSummaryReplyChan)
	go t.galaxyInfoCenter.SystemSymmaryByName(systemName, ch)
	rpl := <-ch
	if rpl.Err != nil {
		SendMessage(ds, channelID, fmt.Sprintf("Couldn't get system info for %s\n%v\n", systemName, rpl.Err))
	} else {
		s := rpl.System
		txt := fmt.Sprintf(
			"```\n"+
				"%s\n"+
				"Distance from Sol: %.02f\n"+
				"Population:        %s\n"+
				"Security:          %s\n"+
				"Allegiance:        %s\n"+
				"State:             %s\n```",
			s.Name,
			edGalaxy.Sol.Distance(rpl.System.Coords),
			humanString(s.BriefInfo.Population),
			s.BriefInfo.Security,
			s.BriefInfo.Allegiance,
			s.BriefInfo.FactionState)
		SendMessage(ds, channelID, txt)
	}
}

func (t *talker) handleDistanceRequest(ds *discordgo.Session, channelID string, systemPair string) {
	pair := strings.Split(systemPair, "/")
	if len(pair) != 2 {
		SendMessage(ds, channelID, "Expected 2 names separated by `/`")
		return
	}

	if len(pair[0]) < 2 || len(pair[1]) < 2 {
		SendMessage(ds, channelID, "System name must be at least 2 chars")
		return
	}

	if _, ignored := t.ignoredSystems[strings.ToLower(pair[0])]; ignored {
		SendMessage(ds, channelID, fmt.Sprintf("%s is a permit locked system", pair[0]))
		return
	}

	if _, ignored := t.ignoredSystems[strings.ToLower(pair[1])]; ignored {
		SendMessage(ds, channelID, fmt.Sprintf("%s is a permit locked system", pair[1]))
		return
	}

	ch := make(edGalaxy.SystemSummaryReplyChan)

	rpls := make([]*edGalaxy.SystemSummaryReply, 2)

	go t.galaxyInfoCenter.SystemSymmaryByName(pair[0], ch)
	go t.galaxyInfoCenter.SystemSymmaryByName(pair[1], ch)

	hasErrors := false

	for i := 0; i < 2; i++ {
		rpl := <-ch
		if rpl.Err != nil {
			SendMessage(ds, channelID, fmt.Sprintf("Couldn't get system info for %s\n%v\n", rpl.RequestedSystemName, rpl.Err))
			hasErrors = true
		}
		rpls[i] = rpl
	}
	if hasErrors || rpls[0].System == nil || rpls[1].System == nil {
		return
	}
	if rpls[0].System.Coords == nil {
		SendMessage(ds, channelID, fmt.Sprintf("Couldn't get system coordinates for %s\n", rpls[0].RequestedSystemName))
		return
	}
	if rpls[1].System.Coords == nil {
		SendMessage(ds, channelID, fmt.Sprintf("Couldn't get system coordinates for %s\n", rpls[1].RequestedSystemName))
		return
	}
	txt := fmt.Sprintf("Distance %s/%s is %.02f\n",
		rpls[0].System.Name, rpls[1].System.Name,
		rpls[0].System.Coords.Distance(rpls[1].System.Coords))
	SendMessage(ds, channelID, txt)
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

func humanString(n int64) string {
	in := strconv.FormatInt(n, 10)
	out := make([]byte, len(in)+(len(in)-2+int(in[0]/'0'))/3)
	if in[0] == '-' {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}
