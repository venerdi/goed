package eddb

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

type CommodityCategoryRecordV5 struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}
type ListingRecordV5 struct {
	Id             int
	Station        *StationRecordV5
	Commodity      *CommodityRecordV5
	Supply         int
	Supply_bracket int
	Buy_price      int
	Sell_price     int
	Demand         int
	Demand_bracket int
	Collected_at   int64
}
type CommodityRecordV5 struct {
	Id                    int                       `json:"id"`
	Name                  string                    `json:"name"`
	CategoryId            int                       `json:"category_id"`
	AveragePrice          int                       `json:"average_price"`
	IsRare                int                       `json:"is_rare"`
	MaxBuyPrice           int                       `json:"max_buy_price"`
	MaxSellPrice          int                       `json:"max_sell_price"`
	MinBuyPrice           int                       `json:"min_buy_price"`
	MinSellPrice          int                       `json:"min_sell_price"`
	BuyPriceLowerAverage  int                       `json:"buy_price_lower_average"`
	SellPriceUpperAverage int                       `"sell_price_upper_average"`
	IsNonMarketable       int                       `json:"is_non_marketable"`
	EdId                  int                       `json:"ed_id"`
	Category              CommodityCategoryRecordV5 `json:"category"`
	Selling               map[int]*ListingRecordV5
	Buying                map[int]*ListingRecordV5
}

type MinorFactionPresenceRecordV5 struct {
	FactionId        int     `json:"minor_faction_id"`
	FactionStateId   int     `json:"state_id"`
	FactionInfluence float64 `json:"influence"`
	FactionState     string  `json:"state"`
}

type SystemRecordV5 struct {
	Id                          int                            `json:"id"`
	EdsmId                      int                            `json:"edsm_id"`
	Name                        string                         `json:"name"`
	X                           float64                        `json:"x"`
	Y                           float64                        `json:"y"`
	Z                           float64                        `json:"z"`
	Population                  int64                          `json:"population"`
	Populated                   bool                           `json:"is_populated"`
	Governmantid                int                            `json:"government_id"`
	Government                  string                         `json:"government"`
	AllegianceId                int                            `json:"allegiance_id"`
	Alegiance                   string                         `json:"allegiance"`
	StateId                     int                            `json:"state_id"`
	State                       string                         `json:"state"`
	SecurityId                  int                            `json:"security_id"`
	Security                    string                         `json:"security"`
	PrimaryEconomyId            int                            `json:"primary_economy_id"`
	PrimaryEconomy              string                         `json:"primary_economy"`
	Power                       string                         `json:"power"`
	PowerState                  string                         `json:"power_state"`
	PowerStateId                int                            `json:"power_state_id"`
	NeedPermit                  bool                           `json:"needs_permit"`
	Updated                     int64                          `json:"updated_at"`
	SimbadRef                   string                         `json:"simbad_ref"`
	ControllingMinofFactionId   int                            `json:"controlling_minor_faction_id"`
	ControllingMinorFactionName string                         `json:"controlling_minor_faction"`
	ReserveTypeId               int                            `json:"reserve_type_id"`
	ReserveType                 string                         `json:"reserve_type"`
	FactionPresences            []MinorFactionPresenceRecordV5 `json:"minor_faction_presences"`
}

type StationRecordV5 struct {
	Id                      int      `json:"id"`
	Name                    string   `json:"name"`
	SystemId                int      `json:"system_id"`
	Updated                 int64    `json:"updated_at"`
	MaxLandingPad           string   `json:"max_landing_pad_size"`
	DistanceToStar          float64  `json:"distance_to_star"`
	Governmantid            int      `json:"government_id"`
	Government              string   `json:"government"`
	AllegianceId            int      `json:"allegiance_id"`
	Alegiance               string   `json:"allegiance"`
	StateId                 int      `json:"state_id"`
	State                   string   `json:"state"`
	TypeId                  int      `json:"type_id"`
	Type                    string   `json:"type"`
	HasBlackmarket          bool     `json:"has_blackmarket"`
	HasMarket               bool     `json:"has_market"`
	HasRefuel               bool     `json:"has_refuel"`
	HasRepair               bool     `json:"has_repair"`
	HasRearm                bool     `json:"has_rearm"`
	HasOutfitting           bool     `json:"has_outfitting"`
	HasShipyard             bool     `json:"has_shipyard"`
	HasDocking              bool     `json:"has_docking"`
	HasCommodities          bool     `json:"has_commodities"`
	ImportCommodities       []string `json:"import_commodities"`
	ExportCommodities       []string `json:"export_commodities"`
	ProhibitedCommodities   []string `json:"prohibited_commodities"`
	Economies               []string `json:"economies"`
	ShipyardUpdated         int64    `json:"shipyard_updated_at"`
	OutfittingUpdated       int64    `json:"outfitting_updated_at"`
	MarketUpdated           int64    `json:"market_updated_at"`
	Planerary               bool     `json:"is_planetary"`
	SellingShips            []string `json:"selling_ships"`
	SellingModules          []int    `json:"selling_modules"`
	SettlementSizeId        int      `json:"settlement_size_id"`
	SettlementSize          string   `json:"settlement_size"`
	SettlementSecurityId    int      `json:"settlement_security_id"`
	SettlementSecurity      string   `json:"settlement_security"`
	BodyId                  int      `json:"body_id"`
	ControllingMinorFaction int      `json:"controlling_minor_faction_id"`
}

type FactionFileRecordV5 struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	UpdatedAt       int64  `json:"updated_at"`
	GovernmentID    int    `json:"government_id"`
	Government      string `json:"government"`
	AllegianceID    int    `json:"allegiance_id"`
	Allegiance      string `json:"allegiance"`
	StateID         int    `json:"state_id"`
	State           string `json:"state"`
	HomeSystemID    int    `json:"home_system_id"`
	IsPlayerFaction bool   `json:"is_player_faction"`
}

func (cat CommodityCategoryRecordV5) String() string {
	return fmt.Sprint("CommodityCategory(%d, '%s')", cat.Id, cat.Name)
}

func ReadCommoditiesFile(fn string) (*map[int]*CommodityRecordV5, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cmdts []CommodityRecordV5
	dec := json.NewDecoder(bufio.NewReader(f))

	err = dec.Decode(&cmdts)
	if err != nil {
		return nil, err
	}
	m := make(map[int]*CommodityRecordV5)
	for i := 0; i < len(cmdts); i++ {
		m[cmdts[i].Id] = &(cmdts[i])
	}
	return &m, nil
}

func ReadSystemsFile(fn string) (*map[int]*SystemRecordV5, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	m := make(map[int]*SystemRecordV5)
	for scanner.Scan() {
		var s SystemRecordV5
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			log.Printf("Error unmarshaling system: %v\n", err)
			continue
		}
		m[s.Id] = &s
	}

	return &m, nil
}

func ReadStationsFile(fn string) (*map[int]*StationRecordV5, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	m := make(map[int]*StationRecordV5)
	for scanner.Scan() {
		var s StationRecordV5
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			log.Printf("Error unmarshaling station: %v\n", err)
			continue
		}
		m[s.Id] = &s
	}

	return &m, nil
}

func atoiEmptyZero(a string) (int, error) {
	if len(a) > 0 {
		return strconv.Atoi(a)
	}
	return 0, nil
}

func BindStations(listing string, commodities *map[int]*CommodityRecordV5, stations *map[int]*StationRecordV5) error {
	f, err := os.Open(listing)
	if err != nil {
		return err
	}
	defer f.Close()

	title := true
	r := csv.NewReader(f)
	pos2name := make(map[int]string)
	linenum := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		linenum += 1
		if title {
			for i, nm := range record {
				pos2name[i] = nm
			}
			title = false
		} else {
			var l ListingRecordV5
			for i, v := range record {
				fld := pos2name[i]
				switch fld {
				case "id":
					l.Id, err = strconv.Atoi(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
				case "station_id":
					stid, err := strconv.Atoi(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
					l.Station = (*stations)[stid]
				case "commodity_id":
					id, err := strconv.Atoi(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
					l.Commodity = (*commodities)[id]
				case "supply":
					l.Supply, err = atoiEmptyZero(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
				case "supply_bracket":
					l.Supply_bracket, err = atoiEmptyZero(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
				case "buy_price":
					l.Buy_price, err = strconv.Atoi(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
				case "sell_price":
					l.Sell_price, err = strconv.Atoi(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
				case "demand":
					l.Demand, err = atoiEmptyZero(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
				case "demand_bracket":
					l.Demand_bracket, err = atoiEmptyZero(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
				case "collected_at":
					tmp, err := strconv.Atoi(v)
					if err != nil {
						log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
						return err
					}
					l.Collected_at = int64(tmp)
				default:
					log.Fatalf("Field error on line %d %s: %v", linenum, fld, err)
					panic("foo")
				}
			}
			if l.Commodity != nil {
				if l.Supply > 0 {
					if l.Commodity.Selling == nil {
						l.Commodity.Selling = make(map[int]*ListingRecordV5)
					}
					l.Commodity.Selling[l.Id] = &l
				}
				if l.Demand > 0 {
					if l.Commodity.Buying == nil {
						l.Commodity.Buying = make(map[int]*ListingRecordV5)
					}
					l.Commodity.Buying[l.Id] = &l
				}
			}
		}

	}
	return nil
}
