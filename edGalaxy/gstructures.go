package edGalaxy

import (
	"errors"
	"log"
	"sync"
)

type BriefSystemInfo struct {
	Allegiance   string
	Government   string
	Faction      string
	FactionState string
	Population   int64
	Reserve      string
	Security     string
	Economy      string
}

type StarInfo struct {
	Name        string
	Type        string
	IsScoopable bool
}

type ShortFactionState struct {
	Name       string
	State      string
	Allegiance string
}

type InterestingSystem4State struct {
	Name       string
	Population int64
	Coords     *Point3D
	Factions   []*ShortFactionState
}

type SystemSummary struct {
	Name        string
	EDSMid      int64
	EDSMid64    int64
	EDDBid      int64
	Coords      *Point3D
	BriefInfo   *BriefSystemInfo
	PrimaryStar *StarInfo
}

type DockableStationShortInfo struct {
	Name       string
	LandingPad string
	Distance   float64
	Planetary  bool
}

type SystemVisitsStat struct {
	Name  string
	Coords *Point3D
	Count int64
}

type SystemVisitsStatCalculated struct {
	Name  string
	Count int64
	Distance float64
}

type SystemSummaryReply struct {
	RequestedSystemName string
	System              *SystemSummary
	Err                 error
}

type HumanWorldStat struct {
	Systems       int64
	Stations      int64
	Factions      int64
	HumanFactions int64
	Population    int64
}

type ActivityStatItem struct {
  Timestamp int64
  NumJumps  int64
  NumDocks  int64
}


type VisitsStatProvider interface {
	GetSystemVisitsStat(coords *Point3D, maxDistance float64, limit int)([]*SystemVisitsStat, int64, error)
	GetActivityStat(coords *Point3D, maxDistance float64)([]*ActivityStatItem)
}


type SystemSummaryReplyChan chan *SystemSummaryReply

type SystemSummaryByNameProvider interface {
	SystemSummaryByName(string, SystemSummaryReplyChan)
}

type summary_by_name_provider_info struct {
	provider SystemSummaryByNameProvider
	priority int
}

type GalaxyInfoCenter struct {
	summaryProviders map[string]summary_by_name_provider_info
	sync.RWMutex
}

func NewGalaxyInfoCenter() *GalaxyInfoCenter {
	return &GalaxyInfoCenter{
		summaryProviders: make(map[string]summary_by_name_provider_info),
	}
}

func (ic *GalaxyInfoCenter) AddSummaryProvider(name string, provider SystemSummaryByNameProvider) {
	ic.Lock()
	ic.summaryProviders[name] = summary_by_name_provider_info{
		provider: provider,
		priority: 0,
	}
	ic.Unlock()
}

func (ic *GalaxyInfoCenter) SystemSummaryByName(name string, ch SystemSummaryReplyChan) {
	p := ic.getProvider()
	if p != nil {
		p.SystemSummaryByName(name, ch)
	} else {
		log.Println("hmm.. No info providers")
		go func() {
			ch <- &SystemSummaryReply{
				System: nil,
				Err:    errors.New("No galaxy info providers"),
			}
		}()
	}
}

func (ic *GalaxyInfoCenter) getProvider() SystemSummaryByNameProvider {
	ic.RLock()
	defer ic.RUnlock()

	for _, p := range ic.summaryProviders {
		return p.provider
	}

	return nil
}
