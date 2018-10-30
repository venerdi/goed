package eddb

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-zeromq/zmq4"
	"goed/edGalaxy"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const (
	SCHEMA_KEY = "$schemaRef"
	RELAY      = "tcp://eddn.edcd.io:9500"

	cmd_exit          = 0
	cmd_backup        = 1
	cmd_restore       = 2
	cmd_getSystemStat = 10
)

type EDDNMessage struct {
	SchemaRef string `json:"$schemaRef"`
	Header    struct {
		GatewayTimestamp time.Time `json:"gatewayTimestamp"`
		SoftwareName     string    `json:"softwareName"`
		SoftwareVersion  string    `json:"softwareVersion"`
		UploaderID       string    `json:"uploaderID"`
	} `json:"header"`
	Message json.RawMessage `json:"message"`
}

type FSDJumpMessage struct {
	Factions []struct {
		Allegiance    string  `json:"Allegiance"`
		FactionState  string  `json:"FactionState"`
		Government    string  `json:"Government"`
		Influence     float64 `json:"Influence"`
		Name          string  `json:"Name"`
		PendingStates []struct {
			State string `json:"State"`
			Trend int    `json:"Trend"`
		} `json:"PendingStates,omitempty"`
		RecoveringStates []struct {
			State string `json:"State"`
			Trend int    `json:"Trend"`
		} `json:"RecoveringStates,omitempty"`
	} `json:"Factions,omitempty"`
	Population          int64     `json:"Population"`
	PowerplayState      string    `json:"PowerplayState,omitempty"`
	Powers              []string  `json:"Powers,omitempty"`
	StarPos             []float64 `json:"StarPos"`
	StarSystem          string    `json:"StarSystem"`
	SystemAddress       int64     `json:"SystemAddress"`
	SystemAllegiance    string    `json:"SystemAllegiance"`
	SystemEconomy       string    `json:"SystemEconomy"`
	SystemFaction       string    `json:"SystemFaction"`
	SystemGovernment    string    `json:"SystemGovernment"`
	SystemSecondEconomy string    `json:"SystemSecondEconomy"`
	SystemSecurity      string    `json:"SystemSecurity"`
	Event               string    `json:"event"`
	Timestamp           time.Time `json:"timestamp"`
}

type DockedMessage struct {
	Body              string    `json:"Body"`
	BodyType          string    `json:"BodyType"`
	DistFromStarLS    float64   `json:"DistFromStarLS"`
	MarketID          int       `json:"MarketID"`
	StarPos           []float64 `json:"StarPos"`
	StarSystem        string    `json:"StarSystem"`
	StationAllegiance string    `json:"StationAllegiance"`
	StationEconomies  []struct {
		Name       string  `json:"Name"`
		Proportion float64 `json:"Proportion"`
	} `json:"StationEconomies"`
	StationEconomy    string    `json:"StationEconomy"`
	StationFaction    string    `json:"StationFaction"`
	StationGovernment string    `json:"StationGovernment"`
	StationName       string    `json:"StationName"`
	StationServices   []string  `json:"StationServices"`
	StationType       string    `json:"StationType"`
	SystemAddress     int64     `json:"SystemAddress"`
	Event             string    `json:"event"`
	Timestamp         time.Time `json:"timestamp"`
}

type shipStatCollector_controlMessage struct {
	command int
	params  interface{}
	result  chan interface{}
}

type shipStatCollector_getSystemVisitStatRequest struct {
	coords      *edGalaxy.Point3D
	maxDistance float64
}

type shipStatCollector_getSystemVisitStatReply struct {
	stat         []*edGalaxy.SystemVisitsStat
	inRangeCount int64
}

type SystemShipStat struct {
	Name          string                                      `json:"Name"`
	Coords        edGalaxy.Point3D                            `json:"Coords"`
	SystemVisits  *edGalaxy.TimeVisitStatCollector            `json:"systems_visits"`
	StationVisits map[string]*edGalaxy.TimeVisitStatCollector `json:"stations_visits"`
}

type ShipStatCollector struct {
	fsdJump chan *EDDNMessage
	docked  chan *EDDNMessage
	control chan shipStatCollector_controlMessage
	/*
		0 - idle
		1 - running
		2 - shutdown requested
	*/
	listenLoopStatus int32
	systemsStat      map[string]*SystemShipStat
}

func NewShipStatCollector() *ShipStatCollector {
	c := &ShipStatCollector{
		fsdJump:          make(chan *EDDNMessage, 10),
		docked:           make(chan *EDDNMessage, 10),
		control:          make(chan shipStatCollector_controlMessage, 10),
		listenLoopStatus: 0,
		systemsStat:      make(map[string]*SystemShipStat)}
	go c.processMessages()
	return c
}

func (c *ShipStatCollector) NoteFSDJump(m *EDDNMessage) error {
	select {
	case c.fsdJump <- m:
	default:
		return errors.New("FSD channel is busy")
	}
	return nil
}

func (c *ShipStatCollector) NoteDocked(m *EDDNMessage) error {
	select {
	case c.docked <- m:
	default:
		return errors.New("Docked channel is busy")
	}
	return nil
}

func (c *ShipStatCollector) processMessages() {
	for {
		select {
		case m := <-c.fsdJump:
			{
				c.handleFSDJump(m)
			}
		case m := <-c.docked:
			{
				c.handleDocked(m)
			}
		case cm := <-c.control:
			{
				if finish := c.handleControlMessage(&cm); finish {
					log.Println("ShipStatCollector::processMessages: exiting")
					return
				}
			}
		}
	}
}

func (c *ShipStatCollector) handleControlMessage(m *shipStatCollector_controlMessage) bool {
	switch m.command {
	case cmd_exit:
		{
			m.result <- 0
			return true
		}
	case cmd_backup:
		{
			errcode := c.performBackup(m.params.(string))
			m.result <- errcode
			return false
		}
	case cmd_restore:
		{
			errcode := c.performRestore(m.params.(string))
			m.result <- errcode
			return false
		}
	case cmd_getSystemStat:
		{
			m.result <- c.getSystemVisitStat(m.params.(shipStatCollector_getSystemVisitStatRequest))
			return false
		}
	default:
		{
			log.Printf("Unhandled command %d\n", m.command)
			return false
		}
	}
	return false
}
func (c *ShipStatCollector) getSystemVisitStat(rq shipStatCollector_getSystemVisitStatRequest) shipStatCollector_getSystemVisitStatReply {
	var totalCount int64 = 0
	stat := make([]*edGalaxy.SystemVisitsStat,0)
	for _, st := range c.systemsStat {
		if rq.coords.Distance( & st.Coords ) <= rq.maxDistance  {
			var systemCount int64 = 0
			for _, timeVisits := range st.SystemVisits.Visits {
				systemCount += timeVisits.VisitCount
			}
			totalCount += systemCount
			stat = append(stat,&edGalaxy.SystemVisitsStat{Name: st.Name, Coords: & st.Coords, Count: systemCount} )
		}
	}
	return shipStatCollector_getSystemVisitStatReply{stat: stat, inRangeCount: totalCount}
}

func (c *ShipStatCollector) performBackup(fileName string) int {
	/*
	   TODO: rename to .bak old file
	*/
	f, err := os.Create(fileName)
	if err != nil {
		log.Printf("Backup to %s failed: %v\n", fileName, err)
		return 1
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, st := range c.systemsStat {
		err = enc.Encode(st)
		if err != nil {
			log.Printf("Backup encode to %s failed: %v\n", fileName, err)
			return 1
		}
	}
	log.Printf("Backup to %s succeeded\n", fileName)
	return 0
}

func (c *ShipStatCollector) performRestore(fileName string) int {
	f, err := os.Open(fileName)
	if err != nil {
		log.Printf("Restore from %s failed: %v\n", fileName, err)
		return 1
	}
	defer f.Close()

	c.systemsStat = make(map[string]*SystemShipStat, 1000)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var s SystemShipStat
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			log.Printf("Error unmarshaling stat: %v\n", err)
			continue
		}
		c.systemsStat[strings.ToUpper(s.Name)] = &s
	}

	log.Printf("Restore from %s succeeded: got %d stats\n", fileName, len(c.systemsStat))
	return 0
}

func (c *ShipStatCollector) handleDocked(m *EDDNMessage) {
	var docked DockedMessage
	err := json.Unmarshal(m.Message, &docked)
	if err != nil {
		log.Printf("json Docked message failed:\n%s\n", string(m.Message))
		return
	}
	nm := strings.ToUpper(docked.StarSystem)
	systemStat, exists := c.systemsStat[nm]
	if !exists {
		if len(docked.StarPos) != 3 {
			log.Printf("Docked at %s to %s                          ---- %s %s %s ignoder - strange coords\n", docked.StarSystem, docked.StationName, m.Header.SoftwareName, m.Header.UploaderID, docked.Timestamp.Format(time.StampMilli))
		}
		systemStat = &SystemShipStat{Name: docked.StarSystem,
			Coords:       edGalaxy.Point3D{X: docked.StarPos[0], Y: docked.StarPos[1], Z: docked.StarPos[2]},
			SystemVisits: edGalaxy.NewTimeVisitStatCollector(7*24, 3600),
		}
		c.systemsStat[nm] = systemStat
		systemStat.SystemVisits.NoteVisit(docked.Timestamp) // it's me here
	}

	if systemStat.StationVisits == nil {
		systemStat.StationVisits = make(map[string]*edGalaxy.TimeVisitStatCollector)
	}
	nm = strings.ToUpper(docked.StationName)
	collector, exists := systemStat.StationVisits[nm]
	if !exists {
		collector = edGalaxy.NewTimeVisitStatCollector(7*24, 3600)
		systemStat.StationVisits[nm] = collector
	}
	collector.NoteVisit(docked.Timestamp)
	log.Printf("Docked at %s to %s\n", docked.StarSystem, docked.StationName)
	//	log.Printf("Docked at %s to %s                          ---- %s %s %s\n", docked.StarSystem, docked.StationName, m.Header.SoftwareName, m.Header.UploaderID, docked.Timestamp.Format(time.StampMilli))
}

func (c *ShipStatCollector) handleFSDJump(m *EDDNMessage) {
	var jump FSDJumpMessage
	err := json.Unmarshal(m.Message, &jump)
	if err != nil {
		log.Printf("json FSDJump message failed:\n%s\n", string(m.Message))
		return
	}
	if len(jump.StarPos) != 3 {
		log.Printf("FSDJump to %s              ---- %s %s %s\n ignored - strange coordinates", jump.StarSystem, m.Header.SoftwareName, m.Header.UploaderID, jump.Timestamp.Format(time.StampMilli))
	}
	nm := strings.ToUpper(jump.StarSystem)
	systemStat, exists := c.systemsStat[nm]
	if !exists {
		systemStat = &SystemShipStat{Name: jump.StarSystem,
			Coords:       edGalaxy.Point3D{X: jump.StarPos[0], Y: jump.StarPos[1], Z: jump.StarPos[2]},
			SystemVisits: edGalaxy.NewTimeVisitStatCollector(7*24, 3600),
		}
		c.systemsStat[nm] = systemStat
	}
	systemStat.SystemVisits.NoteVisit(jump.Timestamp)
	log.Printf("FSDJump to %s\n", jump.StarSystem)
	//	log.Printf("FSDJump to %s              ---- %s %s %s\n", jump.StarSystem, m.Header.SoftwareName, m.Header.UploaderID, jump.Timestamp.Format(time.StampMilli))
}

func (c *ShipStatCollector) Shutdown() {
	m := shipStatCollector_controlMessage{
		command: cmd_exit,
		result:  make(chan interface{})}
	atomic.StoreInt32(&c.listenLoopStatus, 2)
	c.control <- m
	<-m.result
}

func (c *ShipStatCollector) Backup(fileName string) bool {
	m := shipStatCollector_controlMessage{
		command: cmd_backup,
		params:  fileName,
		result:  make(chan interface{})}
	c.control <- m
	res := <-m.result
	return res == 0
}

func (c *ShipStatCollector) Restore(fileName string) bool {
	m := shipStatCollector_controlMessage{
		command: cmd_restore,
		params:  fileName,
		result:  make(chan interface{})}
	c.control <- m
	res := <-m.result
	return res == 0
}

func (c *ShipStatCollector) GetSystemVisitsStat(coords *edGalaxy.Point3D, maxDistance float64, limit int) ([]*edGalaxy.SystemVisitsStat, int64, error) {
	m := shipStatCollector_controlMessage{
		command: cmd_getSystemStat,
		params:  shipStatCollector_getSystemVisitStatRequest{coords: coords, maxDistance: maxDistance},
		result:  make(chan interface{})}
	c.control <- m
	r := <-m.result

	rpl := r.(shipStatCollector_getSystemVisitStatReply)
	
	sort.Slice(rpl.stat, func(i, j int) bool {
		return rpl.stat[i].Count > rpl.stat[j].Count // reverse
	})
	if len(rpl.stat) > limit {
		rpl.stat = rpl.stat[:limit]
	}
	return rpl.stat, rpl.inRangeCount, nil
}


func (c *ShipStatCollector) listenLoop() {
	needDeal := true
	var req zmq4.Socket = nil

	for {
		if atomic.LoadInt32(&c.listenLoopStatus) == 2 {
			if req != nil {
				req.Close()
				return
			}
		}
		if req == nil {
			req = zmq4.NewSub(context.Background())
			req.SetOption(zmq4.OptionSubscribe, "")
			needDeal = true
		}
		if needDeal {
			log.Printf("Dialing %s", RELAY)
			err := req.Dial(RELAY)
			if err != nil {
				log.Printf("could not dial: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			needDeal = false
		}
		msg, err := req.Recv()
		if err != nil {
			log.Printf("could not recv: %v", err)
			if req != nil {
				req.Close()
				req = nil
			}
			time.Sleep(5 * time.Second)
			continue
		}
		b := bytes.NewReader(msg.Bytes())
		r, err := zlib.NewReader(b)
		if err != nil {
			log.Printf("decompress failed: %v", err)
			continue
		}

		dec := json.NewDecoder(r)
		var m EDDNMessage
		if err := dec.Decode(&m); err == io.EOF {
			log.Println("EOF")
		} else if err != nil {
			r.Close()
			log.Println("json decode failed")
			continue
		}
		r.Close()

		var objmap map[string]*json.RawMessage
		err = json.Unmarshal(m.Message, &objmap)
		if err != nil {
			log.Println("json decode message failed")
			continue
		}

		evt, exists := objmap["event"]
		if !exists {
			//			log.Println("event does not exists.")
			continue
		}

		var evtName string
		if err = json.Unmarshal(*evt, &evtName); err != nil {
			log.Printf("Failed to decode event name")
			continue
		}

		switch evtName {
		case "Scan", "Location":
			{
			}
		case "FSDJump":
			{
				c.NoteFSDJump(&m)
			}
		case "Docked":
			{
				c.NoteDocked(&m)
			}
		default:
			{
				log.Printf("Evt: %s Ref %s from %s\n", evtName, m.SchemaRef, m.Header.SoftwareName)

			}
		}
	}
}

func (c *ShipStatCollector) StartListen() {
	run := atomic.CompareAndSwapInt32(&c.listenLoopStatus, 0, 1)
	if run {
		go c.listenLoop()
	} else {
		log.Printf("StartListen at state now %d ignored\n", atomic.LoadInt32(&c.listenLoopStatus))
	}
}
