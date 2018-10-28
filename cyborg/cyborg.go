package cyborg

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"goed/edgic"
	"log"
	"strings"
)

type AssignRoleOnGame struct {
	GuildName   string
	GameName    string
	RoleName    string
	ExcludeUIDs []string
}

type CyborgBotDiscordConfig struct {
	Token          string
	BotName        string
	Operators      []string
	AutoRoles      []AssignRoleOnGame
	IgnoredSystems []string
}

func (c *CyborgBotDiscordConfig) CheckConfig() error {
	if len(c.Token) == 0 {
		return errors.New("Empty discord bot token")
	}
	return nil
}

type CybordBot struct {
	Token        string
	BotName      string
	Version      string
	enableRA     bool
	operators    map[string]int
	roleAssigner *role_assigner
	t            *talker
	DgSession    *discordgo.Session
	giClient     *edgic.EDInfoCenterClient
}

func NewCybordBot(cfg *CyborgBotDiscordConfig, galaxyInfoAddress string) *CybordBot {
	ver := "0.0.4"

	giClient := edgic.NewEDInfoCenterClient(galaxyInfoAddress)

	botName := "Cyborg"
	if len(cfg.BotName) > 0 {
		botName = cfg.BotName
	} else {
		log.Println("Bot name is not set, assuming " + botName)
	}

	b := &CybordBot{
		Token:        cfg.Token,
		BotName:      botName,
		Version:      ver,
		roleAssigner: newRoleAssigner(cfg.AutoRoles),
		t:            newTalker(cfg.Operators, botName, ver, giClient, cfg.IgnoredSystems),
		giClient:     giClient,
	}
	b.operators = make(map[string]int)
	for _, op := range cfg.Operators {
		b.operators[op] = 1
	}
	return b
}

func (bot *CybordBot) Connect(logLevel int) (err error) {
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + bot.Token)

	dg.StateEnabled = true

	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return err
	}

	dg.LogLevel = logLevel
	bot.DgSession = dg

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		bot.onReady(r)
	})

	dg.AddHandler(func(s *discordgo.Session, gc *discordgo.GuildCreate) {
		bot.onGuildCreate(gc)
	})

	dg.AddHandler(func(s *discordgo.Session, p *discordgo.PresenceUpdate) {
		bot.roleAssigner.onPresenceUpdate(s, p)
	})

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		bot.t.onMessageCreate(s, m)
	})

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Printf("error opening connection, %v\n", err)
		return err
	}

	log.Printf("Assigning self-id(main)\n")

	bot.roleAssigner.start()
	bot.t.start()
	return nil
}

func dumpMember(n int, m *discordgo.Member) {
	log.Printf(" member %5d %s: roles %s\n", n, m.User.Username, strings.Join(m.Roles, ", "))
}

func dumpRole(n int, r *discordgo.Role) {
	log.Printf(" role pos %d %s %s color %v\n", r.Position, r.ID, r.Name, r.Color)
}

func dumpGuild(g *discordgo.Guild) {
	log.Printf("Guild id %s name %s members: %d\n", g.ID, g.Name, g.MemberCount)
	for i, m := range g.Roles {
		dumpRole(i, m)
	}
	for i, m := range g.Members {
		dumpMember(i, m)
	}
}

func (bot *CybordBot) onReady(r *discordgo.Ready) {
	fmt.Println("Got the ready event")
	for i, g := range r.Guilds {
		log.Printf("Guild %d: id %s name %s members: %d\n", i, g.ID, g.Name, g.MemberCount)
	}
}

func (bot *CybordBot) onGuildCreate(gc *discordgo.GuildCreate) {
	log.Printf("discordgo.GuildCreate\n")
	bot.roleAssigner.updateGuildInfo(gc.Guild)
	dumpGuild(gc.Guild)
}

func (bot *CybordBot) Close() (err error) {
	bot.roleAssigner.close()
	bot.t.close()
	return bot.DgSession.Close()
}
