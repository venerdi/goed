package edsm

import (
	"encoding/json"
	"errors"
	"goed/pkg/edGalaxy"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

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

type EDSMSystemV1 struct {
	Name         string           `json:"name"`
	EDSMid       int64            `json:"id"`
	EDSMid64     int64            `json:"id64"`
	Coords       edGalaxy.Point3D `json:"coords"`
	CoordsLocked bool             `json:"coordsLocked"`
	SystemInfo   EDSMSysInfo      `json:"information,omitempty"`
	PrimaryStar  EDSMStarInfo     `json:"primaryStar"`
}

func edsmSystem2GalaxySummary(eds *EDSMSystemV1) *edGalaxy.SystemSummary {
	if eds == nil {
		return nil
	}
	return &edGalaxy.SystemSummary{
		Name:     eds.Name,
		EDSMid:   eds.EDSMid,
		EDSMid64: eds.EDSMid64,
		EDDBid:   0,
		Coords:   &eds.Coords,
		BriefInfo: &edGalaxy.BriefSystemInfo{
			Allegiance:   eds.SystemInfo.Allegiance,
			Government:   eds.SystemInfo.Government,
			Faction:      eds.SystemInfo.Faction,
			FactionState: eds.SystemInfo.FactionState,
			Population:   eds.SystemInfo.Population,
			Reserve:      eds.SystemInfo.Reserve,
			Security:     eds.SystemInfo.Security,
			Economy:      eds.SystemInfo.Economy,
		},
		PrimaryStar: &edGalaxy.StarInfo{
			Name:        eds.PrimaryStar.Name,
			Type:        eds.PrimaryStar.Type,
			IsScoopable: eds.PrimaryStar.IsScoopable,
		},
	}
}

type cachedEDSMSystemV1 struct {
	system *EDSMSystemV1
	expire int64
}

type FetchEDSMSystemReply struct {
	RequestedSystemName string
	System              *EDSMSystemV1
	Err                 error
}

const (
	max_concurrent_edsm_requests = 10
)

type EDSMConnector struct {
	tr            *http.Transport
	mtx           sync.RWMutex
	systemsCache  map[string]*cachedEDSMSystemV1
	aliveRequests int
	maxRequests   int
}

func normalizeSystemName(name string) string {
	return strings.ToUpper(name)
}

func NewEDSMConnector(maxConnections int) *EDSMConnector {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	if maxConnections < 1 {
		maxConnections = 1
	}
	if maxConnections > max_concurrent_edsm_requests {
		maxConnections = max_concurrent_edsm_requests
	}
	rv := &EDSMConnector{
		tr:            tr,
		systemsCache:  make(map[string]*cachedEDSMSystemV1),
		aliveRequests: 0,
		maxRequests:   maxConnections,
	}

	return rv
}

func (c *EDSMConnector) Close() {
	c.tr.CloseIdleConnections()
}

func (c *EDSMConnector) SystemSymmaryByName(systemName string, rplChannel edGalaxy.SystemSummaryReplyChan) {
	rplC := make(chan *FetchEDSMSystemReply)
	go c.GetSystemInfo(systemName, rplC)
	rpl := <-rplC
	rplChannel <- &edGalaxy.SystemSummaryReply{
		RequestedSystemName: rpl.RequestedSystemName,
		System:              edsmSystem2GalaxySummary(rpl.System),
		Err:                 rpl.Err,
	}
}

func (c *EDSMConnector) fetchSystem(systemName string) (*EDSMSystemV1, error) {
	client := &http.Client{
		Transport: c.tr,
		Timeout:   20 * time.Second,
	}

	formData := url.Values{
		"showId":          {"1"},
		"showCoordinates": {"1"},
		"showInformation": {"1"},
		"showPrimaryStar": {"1"},
	}
	formData.Add("systemName", systemName)

	log.Printf("Getting info on %s\n", systemName)

	resp, err := client.PostForm("https://www.edsm.net/api-v1/system", formData)
	if err != nil {
		log.Printf("Failed post system %s : %v\n", systemName, err)
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed read system %s : %v\n", systemName, err)
		return nil, err
	}
	resp.Body.Close()

	sbody := string(body)
	if idx := strings.Index(sbody, `"information":[]`); idx != -1 {
		log.Printf("Stripping empty information from: \n%s\n", sbody)
		body = append(body[:idx], body[idx+17:]...)
	}

	if len(body) < 10 {
		log.Printf("Failed parse system %s (data too short): %s\n", systemName, string(body))
		if string(body) == "[]" {
			return nil, errors.New("Unknown system.")
		}
		return nil, errors.New("System is not known of EDSM failure")
	}

	var si EDSMSystemV1
	if err = json.Unmarshal(body, &si); err != nil {
		log.Printf("Failed parse system %s data %s: %v\n", systemName, string(body), err)
		return nil, err
	}

	log.Println(string(body))
	return &si, nil
}

func (c *EDSMConnector) GetSystemInfo(systemName string, rplChannel chan *FetchEDSMSystemReply) {

	usn := normalizeSystemName(systemName)

	c.mtx.RLock()
	cs, here := c.systemsCache[usn]
	mayAskEDSM := c.aliveRequests < c.maxRequests
	c.mtx.RUnlock() // must NOT defer

	if here {
		log.Printf("Returning cached system %s\n", systemName)
		rplChannel <- &FetchEDSMSystemReply{systemName, cs.system, nil}
		return
	}
	if !mayAskEDSM {
		rplChannel <- &FetchEDSMSystemReply{systemName, nil, errors.New("all fetchers are busy")}
		return
	}

	c.incAliveRequestsCount()
	s, err := c.fetchSystem(systemName)
	c.decAliveRequestsCount()

	if err != nil {
		rplChannel <- &FetchEDSMSystemReply{systemName, nil, err}
		return
	}
	usn = normalizeSystemName(s.Name)

	if len(usn) > 1 {
		c.mtx.Lock()
		c.systemsCache[usn] = &cachedEDSMSystemV1{s, 1}
		c.mtx.Unlock()
		rplChannel <- &FetchEDSMSystemReply{systemName, s, nil}
		return
	} else {
		log.Printf("Strange data for system %s: %s\n", usn)
		rplChannel <- &FetchEDSMSystemReply{systemName, nil, errors.New("Inconsistent data")}
		return
	}

}

func (c *EDSMConnector) incAliveRequestsCount() {
	c.mtx.Lock()
	c.aliveRequests++
	c.mtx.Unlock()
}
func (c *EDSMConnector) decAliveRequestsCount() {
	c.mtx.Lock()
	c.aliveRequests--
	c.mtx.Unlock()
}
