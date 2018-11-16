package cyborg

import (
	"bytes"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"goed/edGalaxy"
	"goed/edgic"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/scanner"
)

var (
	rePopularInTheBuble  = regexp.MustCompile(`\s*in\s+the\s*bubb?le\s*`)
	rePopularInTheGalaxy = regexp.MustCompile(`\s*in\s+the\s*galaxy\s*`)
	rePopularAtColonia   = regexp.MustCompile(`\s*at\s+colonia\s*`)
	rePopularNear        = regexp.MustCompile(`\s*near\s*(\S.*\S)`)
	rePopularInside      = regexp.MustCompile(`\s*inside\s*(\d+)\s*from\s+(\S.*\S)`)
)

type incoming_message struct {
	s        *discordgo.Session
	m        *discordgo.MessageCreate
	isDirect bool
}

type system_distance_call_param struct {
	name   string
	radius float64
}

type talker struct {
	version          string
	botName          string
	operators        map[string]int
	incomingMessages chan *incoming_message
	giClient         *edgic.EDInfoCenterClient
	ignoredSystems   map[string]bool
}

func newTalker(ops []string, botName string, ver string, giClient *edgic.EDInfoCenterClient, ignoredSystems []string) *talker {
	t := &talker{
		version:          ver,
		botName:          botName,
		operators:        make(map[string]int),
		incomingMessages: make(chan *incoming_message),
		giClient:         giClient,
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

	if ctx == "help" {
		t.handleHelpRequest(im.s, im.m.ChannelID)
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
	if strings.HasPrefix(ctx, "stations ") {
		t.handleStationsRequest(im.s, im.m.ChannelID, ctx[9:])
		return
	}
	if strings.HasPrefix(ctx, "stat ") {
		t.handleStatRequest(im.s, im.m.ChannelID, ctx[4:])
		return
	}
	if strings.HasPrefix(ctx, "popular ") {
		t.handlePopularSystemsRequest(im.s, im.m.ChannelID, ctx[8:])
		return
	}
	if strings.HasPrefix(ctx, "activity ") {
		t.handleActivityRequest(im.s, im.m.ChannelID, ctx[9:])
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
	case "members":
		t.handleOperatorLSmembers(im)
	default:
		SendMessage(im.s, im.m.ChannelID, "Unknown ls category")
	}
}

func (t *talker) handleOperatorLSmembers(im *incoming_message) {

	membersInfo := make([]string, 0, 1000)

	for _, g := range im.s.State.Guilds {
		membersInfo = append(membersInfo, fmt.Sprintf("%s %s", g.Name, g.ID))
		for _, m := range g.Members {
			membersInfo = append(membersInfo, fmt.Sprintf("    %s %s (%v)", m.User.ID, m.User.Username, m.Nick))
		}
	}
	txt := "```"
	for i, info := range membersInfo {
		txt += "\n"
		txt += info
		if (i+1)%24 == 0 {
			txt += "```"
			SendMessage(im.s, im.m.ChannelID, txt)
			txt = "```"
		}
	}
	if len(txt) > 3 {
		txt += "```"
		SendMessage(im.s, im.m.ChannelID, txt)
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

func (t *talker) handleStatRequest(ds *discordgo.Session, channelID string, categories string) {
	cat := strings.TrimSpace(categories)
	if len(cat) != 0 && !(strings.ToLower(cat) == "humans") {
		SendMessage(ds, channelID, "Sorry, I can only stat human's galaxy now.")
		return
	}
	info, err := t.giClient.GetHumanWorldStat()
	if err != nil {
		SendMessage(ds, channelID, fmt.Sprintf("%v", err))
		return
	}
	txt := fmt.Sprintf("Human galaxy stat:\n```"+
		"Systems:    %s\n"+
		"Stations:   %s\n"+
		"Factions:   %s (%s non-NPC's)\n"+
		"Population: %s```\n",
		humanize.Comma(info.Systems), humanize.Comma(info.Stations),
		humanize.Comma(info.Factions), humanize.Comma(info.HumanFactions),
		humanize.Comma(info.Population))

	SendMessage(ds, channelID, txt)
}

func (t *talker) handleHelpRequest(ds *discordgo.Session, channelID string) {
	txt := fmt.Sprintf("Ciao, I'm %s, talk module version %s.\nKnown to me commands are:```", t.botName, t.version) +
		"system <system name>\n" +
		"\tGives a brief system description\n" +
		"stations <system name>\n" +
		"\tLists the stations in the system\n" +
		"distance <system name 1>/<system name 2>\n" +
		"\tCalculates distance between the systems\n" +
		"stat humans\n" +
		"\tGives some numbers about the galaxy\n" +
		"[popular|activity] ...\n" +
		"\tpopular  - Collects system visit counts\n" +
		"\tactivity - Draws jumps/h and docks/h\n" +
		"\tBoth popular and activity accept:\n" +
		"\t\t--- inside <num L.Y.> from <system name>\n" +
		"\t\t--- near <system name>\n" +
		"\t\t\tA shortcut to --- inside 100 from <system name>\n" +
		"\t\t--- in the bubble\n" +
		"\t\t\tMeans --- inside 1000 from Sol\n" +
		"\t\t--- at Colonia\n" +
		"\t\t\tMeans --- inside 500 from Colonia\n" +
		"\t\t--- in the galaxy\n" +
		"\t\t\tMeans --- inside 100,000 from Sol\n" +
		"```"

	SendMessage(ds, channelID, txt)
}

func (t *talker) chkSystemName(systemName string) string {
	if len(systemName) < 2 {
		return "System name must be at least 2 chars"
	}
	if _, ignored := t.ignoredSystems[strings.ToLower(systemName)]; ignored {
		return fmt.Sprintf("%s is a permit locked system", systemName)
	}
	return ""
}

func (t *talker) handleSystemRequest(ds *discordgo.Session, channelID string, systemName string) {

	if errmsg := t.chkSystemName(systemName); errmsg != "" {
		SendMessage(ds, channelID, errmsg)
		return
	}

	s, err := t.giClient.GetSystemSummary(systemName)

	if err != nil {
		SendMessage(ds, channelID, fmt.Sprintf("%v", err))
		return
	}

	txt := fmt.Sprintf("```\n%s\nDistance from Sol: %.02f\n", s.Name, edGalaxy.Sol.Distance(s.Coords))
	if s.BriefInfo != nil {
		txt += fmt.Sprintf(
			"Population:        %s\n"+
				"Security:          %s\n"+
				"Allegiance:        %s\n"+
				"State:             %s\n",
			humanize.Comma(s.BriefInfo.Population),
			s.BriefInfo.Security,
			s.BriefInfo.Allegiance,
			s.BriefInfo.FactionState)
	}

	SendMessage(ds, channelID, txt+"```")
}

func getStationsTable(stations []*edGalaxy.DockableStationShortInfo) ([][]string, int, int) {
	rows := make([][]string, len(stations))
	mxDistSize := 0
	mxDescrSize := 11
	for i, st := range stations {
		row := make([]string, 3)
		rows[i] = row
		row[0] = fmt.Sprintf("%.0f", st.Distance)
		if len(row[0]) > mxDistSize {
			mxDistSize = len(row[0])
		}
		if st.Planetary {
			row[1] = fmt.Sprintf("%s, Planetary", st.LandingPad)
			mxDescrSize = 13
		} else {
			row[1] = fmt.Sprintf("%s, Orbital", st.LandingPad)
		}
		row[2] = st.Name
	}
	return rows, mxDistSize, mxDescrSize
}

func getVisitedSystemsTable(stat []*edGalaxy.SystemVisitsStatCalculated, total int64) ([][]string, int, int, int, int) {
	//	mxNameLen, mxVisitsLen, mxVisitsPersLen, mxDistLen
	rows := make([][]string, len(stat))

	mxLen := make([]int, 4)
	for i, _ := range mxLen {
		mxLen[i] = 0
	}

	ftotal := float64(total) / 100

	for i, s := range stat {
		row := make([]string, 4)
		rows[i] = row
		row[0] = strings.Title(s.Name)
		row[1] = humanize.Comma(s.Count)
		row[2] = fmt.Sprintf("%.02f", float64(s.Count)/ftotal)
		row[3] = fmt.Sprintf("%.02f", s.Distance)

		for j, txt := range row {
			l := len(txt)
			if l > mxLen[j] {
				mxLen[j] = l
			}
		}
	}
	return rows, mxLen[0], mxLen[1], mxLen[2], mxLen[3]
}

func findPopularSystemParam(rqString string) *system_distance_call_param {
	lcrq := []byte(strings.ToLower(rqString))
	if rePopularInTheBuble.Match(lcrq) {
		return &system_distance_call_param{name: "Sol", radius: 1000}
	}
	if rePopularInTheGalaxy.Match(lcrq) {
		return &system_distance_call_param{name: "Sol", radius: 1000000}
	}
	if rePopularAtColonia.Match(lcrq) {
		return &system_distance_call_param{name: "Colonia", radius: 500}
	}
	lstr := strings.ToLower(rqString)
	mt := rePopularNear.FindAllStringSubmatch(lstr, 1)
	if mt != nil && len(mt) > 0 {
		if mt[0] != nil && len(mt[0]) == 2 {
			return &system_distance_call_param{name: mt[0][1], radius: 100}
		}
	}
	mt = rePopularInside.FindAllStringSubmatch(lstr, 1)
	if mt != nil && len(mt) > 0 {
		if mt[0] != nil && len(mt[0]) == 3 {
			r, err := strconv.ParseFloat(mt[0][1], 64)
			if err == nil {
				if r < 0.01 {
					return nil
				}
				if r > 100000 {
					r = 100000
				}
			}
			return &system_distance_call_param{name: mt[0][2], radius: r}
		}
	}
	return nil
}

func (t *talker) handleActivityRequest(ds *discordgo.Session, channelID string, systemName string) {

	p := findPopularSystemParam(systemName)
	if p == nil {
		SendMessage(ds, channelID, "Sorry, i don't understand you")
		return
	}

	systemName = strings.Title(p.name)

	if errmsg := t.chkSystemName(systemName); errmsg != "" {
		SendMessage(ds, channelID, errmsg)
		return
	}

	stat, err := t.giClient.GetGalaxyActivityStat(p.name, p.radius)
	if err != nil {
		SendMessage(ds, channelID, fmt.Sprintf("%v", err))
		return
	}
	if stat == nil || len(stat) < 2 {
		SendMessage(ds, channelID, fmt.Sprintf("Not enough data to tell something around %s", systemName))
		return
	}

	pngBuffer := &bytes.Buffer{}

	DrawChart(stat, pngBuffer)
	fileName := "galaxyActivity.png"

	ms := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Description: fmt.Sprintf("Activity inside %.1f LY from %s\n", p.radius, systemName),
			Image: &discordgo.MessageEmbedImage{
				URL: "attachment://" + fileName,
			},
		},
		Files: []*discordgo.File{
			&discordgo.File{
				Name:   fileName,
				Reader: pngBuffer,
			},
		},
	}
	ds.ChannelMessageSendComplex(channelID, ms)
}

func (t *talker) handlePopularSystemsRequest(ds *discordgo.Session, channelID string, systemName string) {

	p := findPopularSystemParam(systemName)
	if p == nil {
		SendMessage(ds, channelID, "Sorry, i don't understand you")
		return
	}

	systemName = strings.Title(p.name)

	if errmsg := t.chkSystemName(systemName); errmsg != "" {
		SendMessage(ds, channelID, errmsg)
		return
	}

	stat, total, err := t.giClient.GetMostVisitedSystems(p.name, p.radius, 20)
	if err != nil {
		SendMessage(ds, channelID, fmt.Sprintf("%v", err))
		return
	}
	if total == 0 {
		SendMessage(ds, channelID, fmt.Sprintf("I didn't see any mention of %s in the social media", systemName))
		return
	}
	txt := fmt.Sprintf("I know about %s visit(s) inside %.1f LY from %s\n", humanize.Comma(total), p.radius, systemName)

	rows, mxNameLen, mxVisitsLen, mxVisitsPersLen, mxDistLen := getVisitedSystemsTable(stat, total)

	if mxNameLen < 6 { //"System"
		mxNameLen = 6
	}
	if mxVisitsLen < 7 { //"Visits"
		mxVisitsLen = 7
	}
	if mxVisitsPersLen < 1 { //"%"
		mxVisitsPersLen = 1
	}
	if mxDistLen < 8 { //"Distance"
		mxDistLen = 8
	}

	fmtStr := fmt.Sprintf("%%-%ds  %%%ds  %%%ds  %%%ds\n", mxNameLen, mxVisitsLen, mxVisitsPersLen, mxDistLen)
	log.Printf("Format string: '%s'", fmtStr)

	txt += "```\n"
	txt += fmt.Sprintf(fmtStr, "System", "Visits", "%", "Distance\n")

	for _, row := range rows {
		txt += fmt.Sprintf(fmtStr, row[0], row[1], row[2], row[3])
	}
	txt += "```\n"
	SendMessage(ds, channelID, txt)
}

func (t *talker) handleStationsRequest(ds *discordgo.Session, channelID string, systemName string) {

	if errmsg := t.chkSystemName(systemName); errmsg != "" {
		SendMessage(ds, channelID, errmsg)
		return
	}

	s, err, suggested := t.giClient.GetDockableStations(systemName)

	if err != nil {
		txt := fmt.Sprintf("%v", err)
		if suggested != nil {
			if len(suggested) > 0 {
				if len(suggested) > 1 {
					txt += "\nDid you mean one of the following?```\n"
					for _, sg := range suggested {
						txt += sg + "\n"
					}
					txt += "```\n"
				} else {
					txt += "\nDid you mean " + suggested[0] + "?\n"
				}
			}
		}
		SendMessage(ds, channelID, txt)
		return
	}

	systemName = strings.ToTitle(strings.ToLower(systemName))
	if len(s) == 0 {
		SendMessage(ds, channelID, fmt.Sprintf("%s has no dockable stations", systemName))
		return
	}

	sort.Slice(s, func(i, j int) bool {
		return s[i].Distance < s[j].Distance
	})

	txt := fmt.Sprintf("```\nDockable stations at %s:\n", systemName)
	rows, mxDistLen, mxDescrText := getStationsTable(s)
	if mxDistLen < 9 {
		mxDistLen = 9
	}
	fmtStr := fmt.Sprintf("%%-%ds %%-%ds %%s\n", mxDistLen, mxDescrText)
	txt += fmt.Sprintf(fmtStr, "Distance", "Pad, Type", "Name")
	for _, row := range rows {
		txt += fmt.Sprintf(fmtStr, row[0], row[1], row[2])
	}
	SendMessage(ds, channelID, txt+"```")
}

func (t *talker) handleDistanceRequest(ds *discordgo.Session, channelID string, systemPair string) {
	pair := strings.Split(systemPair, "/")
	if len(pair) != 2 {
		SendMessage(ds, channelID, "Expected 2 names separated by `/`")
		return
	}

	if errmsg := t.chkSystemName(pair[0]); errmsg != "" {
		SendMessage(ds, channelID, errmsg)
		return
	}

	if errmsg := t.chkSystemName(pair[1]); errmsg != "" {
		SendMessage(ds, channelID, errmsg)
		return
	}

	d, err := t.giClient.GetDistance(pair[0], pair[1])

	if err != nil {
		SendMessage(ds, channelID, fmt.Sprintf("%v", err))
		return
	}
	txt := fmt.Sprintf("Distance %s/%s: %s LY\n",
		pair[0], pair[1], humanize.CommafWithDigits(d, 2))
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
