package eddb

import (
	"errors"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"goed/edGalaxy"
	"log"
	"math"
	"sort"
	"strings"
	"time"
)

type EDDBInfo struct {
	commodities   *map[int]*CommodityRecordV5
	systems       *map[int]*SystemRecordV5
	stations      *map[int]*StationRecordV5
	systemsByName *map[string]*SystemRecordV5
	factions      *map[int]*FactionRecordV5
}

type SuitablePoint struct {
	station  *StationRecordV5
	system   *SystemRecordV5
	listing  *ListingRecordV5
	distance float64
}

func BuildEDDBInfo(dataCache *DataCacheConfig) (*EDDBInfo, error) {
	log.Println("Reading eddb galaxy...")
	commodities, err := ReadCommoditiesFile(dataCache.Commodities.LocalFile)
	if err != nil {
		log.Printf("Failed to load commodities: %v", err)
		return nil, err
	}
	systems, err := ReadSystemsFile(dataCache.Systems.LocalFile)
	if err != nil {
		log.Printf("Failed to load systems: %v", err)
		return nil, err
	}
	log.Printf("Got %d systems\n", len(*systems))

	stations, err := ReadStationsFile(dataCache.Stations.LocalFile)
	if err != nil {
		log.Printf("Failed to load stations: %v", err)
		return nil, err
	}
	log.Printf("Got %d stations\n", len(*stations))
	factions, err := ReadFactionsFile(dataCache.Factions.LocalFile)
	if err != nil {
		log.Printf("Failed to load factions: %v", err)
		return nil, err
	}
	log.Printf("Got %d factions\n", len(*factions))
	log.Println("Binding commodities...")
	err = BindStations(dataCache.Listings.LocalFile, commodities, stations)
	if err != nil {
		log.Printf("Unexpected error binding stations: %v\n", err)
		return nil, err
	}
	log.Println("Mapping")
	systemsByName := make(map[string]*SystemRecordV5)
	for _, sys := range *systems {
		systemsByName[strings.ToUpper(sys.Name)] = sys
	}

	for _, station := range *stations {
		system, exists := (*systems)[station.SystemId]
		if !exists {
			log.Printf("Stations %d can not be mapped to system %d\n", station.Id, station.SystemId)
			continue
		}
		if system.stations == nil {
			m := make(map[int]*StationRecordV5)
			system.stations = &m
		}
		(*(system.stations))[station.Id] = station
	}
	log.Println("Ready")
	return &EDDBInfo{commodities: commodities,
		systems:       systems,
		stations:      stations,
		systemsByName: &systemsByName,
		factions:      factions}, nil
}

func (i *EDDBInfo) GetSimilarSystemNames(sname string) []string {
	sz := len(*(i.systems))
	names := make([]string, sz)
	idx := 0
	for _, s := range *i.systems {
		if idx < sz {
			names[idx] = s.Name
			idx++
		} else {
			log.Println("Oops. out of range))")
		}
	}
	return fuzzy.FindFold(sname, names)
}

func (i *EDDBInfo) getCommodity(cName string) (*CommodityRecordV5, bool) {
	cName = strings.ToLower(cName)
	for _, c := range *i.commodities {
		if strings.Compare(strings.ToLower(c.Name), cName) == 0 {
			return c, true
		}
	}
	return nil, false
}

func (i *EDDBInfo) SystemSummaryByName(sName string) (*edGalaxy.SystemSummary, bool) {
	s, exists := (*i.systemsByName)[strings.ToUpper(sName)]
	if !exists {
		return nil, false
	}
	return eddb2galaxy(s), exists
}

func eddb2galaxy(s *SystemRecordV5) *edGalaxy.SystemSummary {
	if s == nil {
		return nil
	}
	return &edGalaxy.SystemSummary{
		Name:   s.Name,
		EDDBid: int64(s.Id),
		Coords: &edGalaxy.Point3D{X: s.X, Y: s.Y, Z: s.Z},
		BriefInfo: &edGalaxy.BriefSystemInfo{
			Allegiance:   s.Alegiance,
			Government:   s.Government,
			Faction:      s.ControllingMinorFactionName,
			FactionState: s.State,
			Population:   s.Population,
			Reserve:      s.ReserveType,
			Security:     s.Security,
			Economy:      s.PrimaryEconomy},
		PrimaryStar: nil,
	}
}

func eddbStation2galaxyDockableStationShortInfo(s *StationRecordV5) *edGalaxy.DockableStationShortInfo {
	if s == nil {
		return nil
	}
	return &edGalaxy.DockableStationShortInfo{
		Name:       s.Name,
		LandingPad: s.MaxLandingPad,
		Distance:   s.DistanceToStar,
		Planetary:  s.Planerary}
}

func (i *EDDBInfo) GetSystemByName(sName string) (*SystemRecordV5, bool) {
	s, exists := (*i.systemsByName)[strings.ToUpper(sName)]
	return s, exists
}

func (i *EDDBInfo) GetSystemCoordsByName(sName string) (*edGalaxy.Point3D, bool) {
	s, ok := i.GetSystemByName(sName)
	if ok {
		return &edGalaxy.Point3D{X: s.X, Y: s.Y, Z: s.Z}, true
	}
	return nil, false
}
func (i *EDDBInfo) GetDockableStations(sName string) ([]*edGalaxy.DockableStationShortInfo, bool) {
	s, exists := i.GetSystemByName(sName)
	if !exists {
		return nil, false
	}
	sInfos := make([]*edGalaxy.DockableStationShortInfo, 0)
	if s.stations != nil {
		for _, st := range *(s.stations) {
			if st.HasDocking {
				sInfos = append(sInfos, eddbStation2galaxyDockableStationShortInfo(st))
			}
		}
	}
	return sInfos, true
}

func (i *EDDBInfo) FindCommodity(cName string, sName string, minSupply int, minPad string, allowPlanetary bool, maxLocalDist float64, maxDistance float64, maxUpdateAge int64) ([]*SuitablePoint, error) {
	c, ok := i.getCommodity(cName)
	if !ok {
		return nil, errors.New("Unknown commodity")
	}

	originSystem, ok := i.GetSystemByName(sName)
	if !ok {
		return nil, errors.New("Unknown system")
	}
	minPad = strings.ToUpper(minPad)
	nowSenonds := time.Now().Unix()

	spoints := make([]*SuitablePoint, 0)
	for _, l := range c.Selling {
		if l.Supply < minSupply {
			continue
		}
		st := l.Station
		if st == nil {
			log.Printf("Nil station in the listing")
			continue
		}
		if !allowPlanetary && st.Planerary {
			continue
		}
		if minPad == "L" && strings.ToUpper(st.MaxLandingPad) != minPad {
			continue
		}
		if nowSenonds-st.MarketUpdated > maxUpdateAge {
			continue
		}
		ss := (*i.systems)[st.SystemId]
		if ss == nil {
			log.Printf("Can't find system for station %d %s\n", st.Id, st.Name)
		}
		if ss != nil {
			dx := originSystem.X - ss.X
			dy := originSystem.Y - ss.Y
			dz := originSystem.Z - ss.Z
			stardis := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if stardis < maxDistance {
				spoints = append(spoints, &SuitablePoint{st, ss, l, stardis})
			}
		}
	}
	sort.Slice(spoints, func(i, j int) bool {
		return spoints[i].distance < spoints[j].distance
	})
	return spoints, nil
}
func (s *SystemRecordV5) GetCoordinates() *edGalaxy.Point3D {
	return &edGalaxy.Point3D{X: s.X, Y: s.Y, Z: s.Z}
}

func (i *EDDBInfo) getShortFactionState(factionId int) *edGalaxy.ShortFactionState {
	fInfo, exists := (*i.factions)[factionId]
	if !exists {
		return nil
	}
	return &edGalaxy.ShortFactionState{Name: fInfo.Name, State: fInfo.State, Allegiance: fInfo.Allegiance}
}

func (i *EDDBInfo) FindStates(states []string, place *edGalaxy.Point3D, minPop int64, maxDistance float64, maxEntries int) []*edGalaxy.InterestingSystem4State {

	wantedStates := make(map[string]bool)

	for _, state := range states {
		wantedStates[strings.ToUpper(state)] = true
	}

	suitableSystems := make([]*edGalaxy.InterestingSystem4State, 0)
	for _, s := range *i.systems {
		if _, wanted := wantedStates[strings.ToUpper(s.State)]; !wanted {
			continue
		}
		if s.Population < minPop {
			continue
		}
		coords := s.GetCoordinates()
		if place.Distance(coords) > maxDistance {
			continue
		}
		if s.FactionPresences == nil {
			continue
		}
		factions := make([]*edGalaxy.ShortFactionState, 0)
		for _, fc := range s.FactionPresences {
			si := i.getShortFactionState(fc.FactionId)
			if si != nil {
				factions = append(factions, si)
			}
		}
		if len(factions) > 0 {
			suitableSystems = append(suitableSystems,
				&edGalaxy.InterestingSystem4State{Name: s.Name, Population: s.Population, Coords: coords, Factions: factions})
			if len(suitableSystems) >= maxEntries {
				break
			}			
		}
	}
	return suitableSystems
}

func (i *EDDBInfo) GetHumanWorldStat() *edGalaxy.HumanWorldStat {
	var population int64 = 0
	for _, system := range *i.systems {
		population += system.Population
	}
	var humanFactions int64 = 0
	for _, f := range *i.factions {
		if f.IsPlayerFaction {
			humanFactions++
		}
	}
	return &edGalaxy.HumanWorldStat{
		Systems:       int64(len(*i.systems)),
		Stations:      int64(len(*i.stations)),
		Factions:      int64(len(*i.factions)),
		HumanFactions: humanFactions,
		Population:    population}
}
