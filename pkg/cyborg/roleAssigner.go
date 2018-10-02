package cyborg

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"sync"
)

type role_add_request struct {
	s       *discordgo.Session
	guildId string
	userId  string
	roleId  string
}

type target_role_info struct {
	targetRole  string
	excludeUIDs map[string]bool
}

type role_assigner struct {
	humanRoleAssigns []AssignRoleOnGame
	mtx              sync.RWMutex
	discordAutoRoles map[string]map[string]*target_role_info // guild->game name->roleInfo
	addRolesChannel  chan *role_add_request
}

func newRoleAssigner(humanRoleAssigns []AssignRoleOnGame) *role_assigner {
	return &role_assigner {
		humanRoleAssigns: humanRoleAssigns,
		discordAutoRoles: make(map[string]map[string]*target_role_info),
		addRolesChannel: make(chan *role_add_request, 100),
	}
}

func (self *role_assigner) start() {
	go handleOutgoingRoleAdd(self.addRolesChannel)
}

func (self *role_assigner) close() {
	close(self.addRolesChannel)
}

func (self *role_assigner) updateGuildInfo(g *discordgo.Guild) {
	assignMap := make(map[string]*target_role_info)
	for _, assignRoleOnGame := range self.humanRoleAssigns {
		log.Printf("Cmp '%s' vs '%s' \n", assignRoleOnGame.GuildName, g.Name)
		if assignRoleOnGame.GuildName == g.Name {
			for _, r := range g.Roles {
				log.Printf("Cmp '%s' vs '%s' \n", assignRoleOnGame.RoleName, r.Name)
				if assignRoleOnGame.RoleName == r.Name {
					var tri target_role_info
					tri.targetRole = r.ID
					tri.excludeUIDs = make(map[string]bool)
					for _, uid := range assignRoleOnGame.ExcludeUIDs {
						tri.excludeUIDs[uid] = true
					}
					assignMap[assignRoleOnGame.GameName] = &tri
					log.Printf("Assigned '%s' to '%s' with %d exclusions\n", tri.targetRole, assignRoleOnGame.GameName, len(tri.excludeUIDs))
				}
			}
		}
	}
	GID := g.ID

	self.mtx.Lock()
	defer self.mtx.Unlock()

	if len(assignMap) > 0 {
		self.discordAutoRoles[GID] = assignMap
		log.Println("Some asiignt roles map set")
	} else {
		delete(self.discordAutoRoles, GID)
		log.Println("Asignt roles map cleared")
	}
}

func handleOutgoingRoleAdd(addRolesChannel chan *role_add_request) {
	log.Printf("handleOutgoingRoleAdd started...\n")
	for {
		r, more := <-addRolesChannel
		if more {
			log.Printf("Adding role %s to %s on guild %s\n", r.roleId, r.userId, r.guildId)
			err := r.s.GuildMemberRoleAdd(r.guildId, r.userId, r.roleId)
			if err != nil {
				log.Printf("Error adding role: %s\n", err)
			} else {
				log.Printf("Role added")
			}
		} else {
			break
		}
	}
	log.Printf("handleOutgoingRoleAdd finished\n")
}

func (ra *role_assigner) onPresenceUpdate(s *discordgo.Session, p *discordgo.PresenceUpdate) {
	if p.Game == nil {
		return
	}
	gameName := p.Game.Name
	ra.mtx.RLock()
	defer ra.mtx.RUnlock()

	gmap, exists := ra.discordAutoRoles[p.GuildID]
	if !exists {
		return
	}
	roleInfo, exists := gmap[gameName]
	if !exists {
		return
	}
	currentUid := p.Presence.User.ID
	log.Printf("Testing exclusion map %d\n", len(roleInfo.excludeUIDs))
	_, exists = roleInfo.excludeUIDs[currentUid]
	if exists {
		log.Printf("Ignoring user %s (in exclude map)\n", currentUid)
		return
	}
	for _, r := range p.Roles {
		if roleInfo.targetRole == r {
			log.Printf("User %s already has role %s\n", currentUid, r)
			return
		}
	}
	log.Printf("Role %s should be assigned to user %s\n", roleInfo.targetRole, currentUid)

	rq := role_add_request{
		s:       s,
		guildId: p.GuildID,
		userId:  currentUid,
		roleId:  roleInfo.targetRole,
	}

	ra.addRolesChannel <- &rq
}
