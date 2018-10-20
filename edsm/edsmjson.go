package edsm

import (
	"encoding/json"
	"goed/edGalaxy"
)

/*
 use https://mholt.github.io/json-to-go/
*/

type EDSMSysInfo struct {
	Allegiance    string `json:"allegiance"`
	Government    string `json:"government"`
	Faction       string `json:"faction"`
	FactionState  string `json:"factionState"`
	Population    int64  `json:"population"`
	Reserve       string `json:"reserve"`
	Security      string `json:"security"`
	Economy       string `json:"economy"`
	SecondEconomy string `json:"secondEconomy"`
}

type EDSMStarInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	IsScoopable bool   `json:"isScoopable"`
}

type EDSMSystemV1_T struct {
	Name         string           `json:"name"`
	EDSMid       int64            `json:"id"`
	EDSMid64     int64            `json:"id64"`
	Coords       edGalaxy.Point3D `json:"coords"`
	CoordsLocked bool             `json:"coordsLocked"`
	SI           json.RawMessage  `json:"information"`
	PrimaryStar  EDSMStarInfo     `json:"primaryStar"`
}

type EDSMSystemV1 struct {
	Name         string
	EDSMid       int64
	EDSMid64     int64
	Coords       *edGalaxy.Point3D
	CoordsLocked bool
	SystemInfo   *EDSMSysInfo
	PrimaryStar  *EDSMStarInfo
}

type EDSMStationInfo struct {
	ID                 int             `json:"id"`
	MarketID           int64           `json:"marketId"`
	Type               string          `json:"type"`
	Name               string          `json:"name"`
	DistanceToArrival  float64         `json:"distanceToArrival"`
	Allegiance         string          `json:"allegiance"`
	Government         string          `json:"government"`
	Economy            string          `json:"economy"`
	SecondEconomy      string			`json:"secondEconomy"`
	HasMarket          bool            `json:"haveMarket"`
	HasShipyard        bool            `json:"haveShipyard"`
	HasOutfitting      bool            `json:"haveOutfitting"`
	OtherServices      []string        `json:"otherServices"`
	ControllingFaction struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"controllingFaction"`
	UpdateTime struct {
		Information string `json:"information"`
		Market      string `json:"market"`
		Shipyard    string `json:"shipyard"`
		Outfitting  string `json:"outfitting"`
	} `json:"updateTime"`
}

type EDMSStationsInfoV1 struct {
	SystemID   int               `json:"id"`
	SystemID64 int64             `json:"id64"`
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	Stations   []EDSMStationInfo `json:"stations"`
}
