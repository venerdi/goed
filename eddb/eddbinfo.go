package eddb

import (
	"errors"
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
}

type SuitablePoint struct {
	station  *StationRecordV5
	system   *SystemRecordV5
	listing  *ListingRecordV5
	distance float64
}

func BuildEDDBInfo(dataCache *DataCacheConfig) (*EDDBInfo, error) {
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
	stations, err := ReadStationsFile(dataCache.Stations.LocalFile)
	if err != nil {
		log.Printf("Failed to load stations: %v", err)
		return nil, err
	}
	log.Printf("Got %d systems\n", len(*systems))
	log.Printf("Got %d stations\n", len(*stations))

	err = BindStations(dataCache.Listings.LocalFile, commodities, stations)
	if err != nil {
		log.Printf("Unexpected error binding stations: %v\n", err)
		return nil, err
	}
	systemsByName := make(map[string]*SystemRecordV5)
	for _, sys := range *systems {
		systemsByName[strings.ToUpper(sys.Name)] = sys
	}

	return &EDDBInfo{commodities: commodities, systems: systems, stations: stations, systemsByName: &systemsByName}, nil
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
