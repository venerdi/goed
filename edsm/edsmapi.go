package edsm

import (
	"encoding/json"
	"errors"
	"goed/edGalaxy"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

/*
 use https://mholt.github.io/json-to-go/
*/

func edsmSysInfo2galaxyBriefSystemInfo(si *EDSMSysInfo) *edGalaxy.BriefSystemInfo {
	if si == nil {
		return nil
	}
	return &edGalaxy.BriefSystemInfo{
		Allegiance:   si.Allegiance,
		Government:   si.Government,
		Faction:      si.Faction,
		FactionState: si.FactionState,
		Population:   si.Population,
		Reserve:      si.Reserve,
		Security:     si.Security,
		Economy:      si.Economy,
	}
}

func edsmSystem2GalaxySummary(eds *EDSMSystemV1) *edGalaxy.SystemSummary {
	if eds == nil {
		return nil
	}
	return &edGalaxy.SystemSummary{
		Name:      eds.Name,
		EDSMid:    eds.EDSMid,
		EDSMid64:  eds.EDSMid64,
		EDDBid:    0,
		Coords:    eds.Coords,
		BriefInfo: edsmSysInfo2galaxyBriefSystemInfo(eds.SystemInfo),
		PrimaryStar: &edGalaxy.StarInfo{
			Name:        eds.PrimaryStar.Name,
			Type:        eds.PrimaryStar.Type,
			IsScoopable: eds.PrimaryStar.IsScoopable,
		},
	}
}

type cachedEDSMSystemV1 struct {
	system    *EDSMSystemV1
	timestamp time.Time
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

func (c *EDSMConnector) SystemSummaryByName(systemName string, rplChannel edGalaxy.SystemSummaryReplyChan) {
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

	//	sbody := string(body)
	//	if idx := strings.Index(sbody, `"information":[]`); idx != -1 {
	//		log.Printf("Stripping empty information from: \n%s\n", sbody)
	//		body = append(body[:idx], body[idx+17:]...)
	//	}

	if len(body) < 10 {
		log.Printf("Failed parse system %s (data too short): %s\n", systemName, string(body))
		if string(body) == "[]" {
			return nil, errors.New("Unknown system.")
		}
		return nil, errors.New("System is not known of EDSM failure")
	}

	var si EDSMSystemV1_T
	if err = json.Unmarshal(body, &si); err != nil {
		log.Printf("Failed parse system %s data %s: %v\n", systemName, string(body), err)
		return nil, err
	}

	log.Println(string(body))
	log.Printf("Checking interface: its a %v\n", si.SI)
	return temp2publicSysteinfo(&si), nil
}

func temp2publicSysteinfo(t *EDSMSystemV1_T) *EDSMSystemV1 {
	rv := &EDSMSystemV1{
		Name:         t.Name,
		EDSMid:       t.EDSMid,
		EDSMid64:     t.EDSMid64,
		Coords:       t.Coords.Clone(),
		CoordsLocked: t.CoordsLocked,
		PrimaryStar:  &t.PrimaryStar,
	}

	var si EDSMSysInfo

	if err := json.Unmarshal(t.SI, &si); err == nil {
		rv.SystemInfo = &si
	} else {
		rv.SystemInfo = nil
	}

	return rv
}

func (c *EDSMConnector) GetSystemInfo(systemName string, rplChannel chan *FetchEDSMSystemReply) {

	usn := normalizeSystemName(systemName)

	c.mtx.RLock()
	cs, here := c.systemsCache[usn]
	mayAskEDSM := c.aliveRequests < c.maxRequests
	c.mtx.RUnlock() // must NOT defer

	if here {
		if time.Now().Sub(cs.timestamp).Hours() < 1 {
			log.Printf("Returning cached system %s\n", systemName)
			rplChannel <- &FetchEDSMSystemReply{systemName, cs.system, nil}
			return
		}
		log.Printf("System %s is expired in the cache\n", systemName)
		if !mayAskEDSM {
			log.Printf("Returning EXPIRED cached system %s (no free slots)\n", systemName)
			rplChannel <- &FetchEDSMSystemReply{systemName, cs.system, nil}
			return
		}
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
		c.systemsCache[usn] = &cachedEDSMSystemV1{s, time.Now()}
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
