package edgic

import (
	"errors"
	empty "github.com/golang/protobuf/ptypes/empty"
	pb "goed/api/protobuf-spec"
	"goed/edGalaxy"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"log"
	"time"
)

type EDInfoCenterClient struct {
	addr string
}

func NewEDInfoCenterClient(addr string) *EDInfoCenterClient {
	return &EDInfoCenterClient{addr: addr}
}

type rpccallproc func(pb.EDInfoCenterClient, context.Context)

func callRpc(addr string, rpcCall rpccallproc) error {
	if len(addr) < 5 {
		return errors.New("Galaxy information server is not configured")
	}
	log.Printf("Dialing info center '%s'\n", addr)
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		log.Printf("did not connect: %v", err)
		return errors.New("Galaxy information server is not available")
	}
	defer conn.Close()
	c := pb.NewEDInfoCenterClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rpcCall(c, ctx)

	return nil
}

func (cc *EDInfoCenterClient) GetDistance(name1 string, name2 string) (float64, error) {
	var rpl *pb.SystemsDistanceReply
	var cerr error = nil

	distanceCall := func(c pb.EDInfoCenterClient, ctx context.Context) {
		rpl, cerr = c.GetDistance(ctx, &pb.SystemsDistanceRequest{Name1: name1, Name2: name2})
	}

	err := callRpc(cc.addr, distanceCall)

	if err != nil {
		return 0, err
	}

	if cerr != nil {
		log.Printf("Could not get distance: %v", err)
		return 0, errors.New("Galaxy information server malfunction")
	}

	if len(rpl.Error) != 0 {
		return 0, errors.New(rpl.Error)
	}

	return rpl.GetDistance(), nil
}

func (cc *EDInfoCenterClient) GetHumanWorldStat() (*edGalaxy.HumanWorldStat, error) {
	var stat *pb.HumanWorldStat
	var cerr error = nil

	call := func(c pb.EDInfoCenterClient, ctx context.Context) {
		stat, cerr = c.GetHumanWorldStat(ctx, &empty.Empty{})
	}

	err := callRpc(cc.addr, call)

	if err != nil {
		return nil, err
	}

	if cerr != nil {
		log.Printf("Could not get system summary: %v", err)
		return nil, errors.New("Galaxy information server malfunction")
	}

	if stat == nil {
		return nil, errors.New("Galaxy information server is broken")
	}

	return &edGalaxy.HumanWorldStat{
		Systems:       stat.GetSystems(),
		Stations:      stat.GetStations(),
		Factions:      stat.GetFactions(),
		HumanFactions: stat.GetHumanFactions(),
		Population:    stat.GetPopulation()}, nil
}

func (cc *EDInfoCenterClient) GetMostVisitedSystems(systemName string, maxDistance float64, limit int) ([]*edGalaxy.SystemVisitsStatCalculated, int64, error) {
	var rpl *pb.MostVisitedSystemsReply
	var cerr error = nil

	statcall := func(c pb.EDInfoCenterClient, ctx context.Context) {
		rpl, cerr = c.GetMostVisitedSystems(ctx, &pb.MostVisitedSystemsRequest{
			Origin:      systemName,
			MaxDistance: maxDistance, Limit: int64(limit)})
	}
	err := callRpc(cc.addr, statcall)

	if err != nil {
		return nil, 0, err
	}

	if cerr != nil {
		log.Printf("Could not get most visited systems: %v", err)
		return nil, 0, errors.New("Galaxy information server malfunction")
	}
	return pbSystemVisitsStat2galaxySystemVisitsStatCalculated(rpl.GetSystemVisitStat()), rpl.GetTotalCount(), nil
}

func (cc *EDInfoCenterClient) GetSystemSummary(name string) (*edGalaxy.SystemSummary, error) {
	var rpl *pb.SystemSummaryReply
	var cerr error = nil

	sumcall := func(c pb.EDInfoCenterClient, ctx context.Context) {
		rpl, cerr = c.GetSystemSummary(ctx, &pb.SystemByNameRequest{Name: name})
	}

	err := callRpc(cc.addr, sumcall)

	if err != nil {
		return nil, err
	}

	if cerr != nil {
		log.Printf("Could not get system summary: %v", err)
		return nil, errors.New("Galaxy information server malfunction")
	}

	if len(rpl.Error) != 0 {
		return nil, errors.New(rpl.Error)
	}

	return pbSystemSummary2galaxy(rpl.GetSummary()), nil
}

func (cc *EDInfoCenterClient) GetDockableStations(name string) ([]*edGalaxy.DockableStationShortInfo, error, []string) {
	var rpl *pb.DockableStationsReply
	var cerr error = nil

	call := func(c pb.EDInfoCenterClient, ctx context.Context) {
		rpl, cerr = c.GetDockableStations(ctx, &pb.SystemByNameRequest{Name: name})
	}

	err := callRpc(cc.addr, call)

	if err != nil {
		return nil, err, nil
	}

	if cerr != nil {
		log.Printf("Could not get system dockables: %v", err)
		return nil, errors.New("Galaxy information server malfunction"), nil
	}

	if len(rpl.Error) != 0 {
		return nil, errors.New(rpl.Error), rpl.GetSuggestedSystems()
	}

	pbStations := rpl.GetStations()
	sz := len(pbStations)
	stations := make([]*edGalaxy.DockableStationShortInfo, sz)
	for i := 0; i < sz; i++ {
		stations[i] = pb2galaxyDockableStationShortInfo(pbStations[i])
	}
	return stations, nil, nil
}

func pbPoint3D2galaxy(p *pb.Point3D) *edGalaxy.Point3D {
	if p == nil {
		return nil
	}
	return &edGalaxy.Point3D{X: p.X, Y: p.Y, Z: p.Z}
}

func pbSystemVisitsStat2galaxySystemVisitsStatCalculated(pbStat []*pb.SystemVisitsStat) []*edGalaxy.SystemVisitsStatCalculated {
	if pbStat == nil {
		return nil
	}
	stat := make([]*edGalaxy.SystemVisitsStatCalculated, len(pbStat))
	for i, s := range pbStat {
		stat[i] = &edGalaxy.SystemVisitsStatCalculated{
			Name:     s.GetName(),
			Count:    s.GetCount(),
			Distance: s.GetDistance()}
	}
	return stat
}

func pmPopSystemBriefInfo2galaxy(i *pb.PopulatedSystemBriefInfo) *edGalaxy.BriefSystemInfo {
	if i == nil {
		return nil
	}
	return &edGalaxy.BriefSystemInfo{
		Allegiance:   i.GetAllegiance(),
		Government:   i.GetGovernment(),
		Faction:      i.GetFaction(),
		FactionState: i.GetFactionState(),
		Population:   i.GetPopulation(),
		Reserve:      i.GetReserve(),
		Security:     i.GetSecurity(),
		Economy:      i.GetEconomy()}
}

func pb2galaxyDockableStationShortInfo(s *pb.DockableStationShortInfo) *edGalaxy.DockableStationShortInfo {
	if s == nil {
		return nil
	}
	return &edGalaxy.DockableStationShortInfo{
		Name:       s.Name,
		LandingPad: s.LandingPad,
		Distance:   s.Distance,
		Planetary:  s.Planetary}
}

func pbSystemSummary2galaxy(s *pb.SystemSummary) *edGalaxy.SystemSummary {
	if s == nil {
		return nil
	}
	return &edGalaxy.SystemSummary{
		Name:      s.GetName(),
		Coords:    pbPoint3D2galaxy(s.GetCoords()),
		BriefInfo: pmPopSystemBriefInfo2galaxy(s.GetPopSystemInfo())}
}
