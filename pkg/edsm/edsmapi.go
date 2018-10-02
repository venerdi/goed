package edsm

import (
	"encoding/json"
	"errors"
	"goed/pkg/galaxy"
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
	Population    string `json:"population"`
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
	Name         string         `json:"name"`
	EDSMid       int64          `json:"id"`
	EDSMid64     int64          `json:"id64"`
	Coords       galaxy.Point3D `json:"coords"`
	CoordsLocked bool           `json:"coordsLocked"`
	PrimaryStar  EDSMStarInfo   `json:"primaryStar"`
}

type cachedEDSMSystemV1 struct {
	system *EDSMSystemV1
	expire int64
}

type FetchEDSMSystemReply struct {
	System *EDSMSystemV1
	Err    error
}

type system_fetch_request struct {
	systemName string
	rplChannel chan *FetchEDSMSystemReply
}

type EDSMConnector struct {
	tr           *http.Transport
	mtx          sync.RWMutex
	systemsCache map[string]*cachedEDSMSystemV1
	systemsFetch  chan *system_fetch_request
}

func normalizeSystemName(name string) string {
	return strings.ToUpper(name)
}

func NewEDSMConnector() *EDSMConnector {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	rv := &EDSMConnector{
		tr:           tr,
		systemsCache: make(map[string]*cachedEDSMSystemV1),
		systemsFetch:  make(chan *system_fetch_request, 2),
	}
	
	go rv.systemsFetcher()
	go rv.systemsFetcher()
	
	return rv
}

func (c *EDSMConnector) Close() {
	close(c.systemsFetch)
}

func (c *EDSMConnector) systemsFetcher() {
	log.Printf("systemsFetcher started...\n")
	for {
		rq, more := <- c.systemsFetch
		if more {
			c.fetchSystem(rq)
		} else {
			break
		}
	}
	log.Println("systemsFetcher finished")
}

func (c *EDSMConnector) fetchSystem(rq *system_fetch_request) {
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
	formData.Add("systemName", rq.systemName)

	log.Printf("Getting info on %s\n", rq.systemName)

	resp, err := client.PostForm("https://www.edsm.net/api-v1/system", formData)
	if err != nil {
		log.Fatalf("Failed post system %s : %v\n", rq.systemName, err)
		rq.rplChannel <- &FetchEDSMSystemReply{ nil, err }
		return
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed read system %s : %v\n", rq.systemName, err)
		rq.rplChannel <- &FetchEDSMSystemReply{ nil, err }
		return
	}
	resp.Body.Close()
	var si EDSMSystemV1

	if err = json.Unmarshal(body, &si); err != nil {
		log.Fatalf("Failed parse system %s data %s: %v\n", rq.systemName, string(body), err)
		rq.rplChannel <- &FetchEDSMSystemReply{ nil, err }
		return
	}
	
	
	usn := normalizeSystemName(si.Name)
	
	if len(usn) > 1 {
		c.mtx.Lock()
		c.systemsCache[usn] = &cachedEDSMSystemV1 {&si, 1 }
		c.mtx.Unlock()
		rq.rplChannel <- &FetchEDSMSystemReply{ &si, nil }
	}else{
		log.Fatalf("Strange data system %s data %s\n", rq.systemName, string(body))
	}
	
}

func (c *EDSMConnector) GetSystemInfo(systemName string, rplChannel chan *FetchEDSMSystemReply) {

	usn := normalizeSystemName(systemName)

	c.mtx.RLock()
	cs, here := c.systemsCache[usn]
	c.mtx.RUnlock() // must NOT defer
	
	if here {
		log.Printf("Returning cached system %s\n", systemName)
		rplChannel <- &FetchEDSMSystemReply { cs.system, nil }
		return
	}
	
	rq := &system_fetch_request{systemName, rplChannel }
	select {
	case c.systemsFetch <- rq:
	default:
		log.Println("Fetchers are busy for system", systemName)
		rq.rplChannel <- &FetchEDSMSystemReply{ nil, errors.New("all retchers are busy") }
	}
}
