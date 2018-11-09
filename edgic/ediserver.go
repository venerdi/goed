package edgic

import (
	"errors"
	"fmt"
	"log"
	"net"

	empty "github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "goed/api/protobuf-spec"
	"goed/edGalaxy"
	"goed/eddb"
	"goed/edsm"
	"sync/atomic"
)

type GrpcServerConf struct {
	Port    string
	Enabled bool
}

type GIServer struct {
	eddbInfo           atomic.Value
	edsmc              *edsm.EDSMConnector
	visitsStatProvider edGalaxy.VisitsStatProvider
	cfg                GrpcServerConf
	s                  *grpc.Server
}

type grpcProcessor struct {
	gi *GIServer
}

func NewGIServer(cfg GrpcServerConf) *GIServer {
	return &GIServer{cfg: cfg, edsmc: edsm.NewEDSMConnector(3)}
}

func (s *GIServer) SetEDDBData(data *eddb.EDDBInfo) {
	s.eddbInfo.Store(data)
}

func (s *GIServer) SetVisitsStatProvider(prov edGalaxy.VisitsStatProvider) {
	s.visitsStatProvider = prov
}

func (s *GIServer) getSystemCoords(systemName string) (*edGalaxy.Point3D, bool) {
	ss, known := s.getSystemSummaryByName(systemName)
	if !known {
		return nil, false
	}
	return ss.Coords, true
}

func (s *GIServer) getSystemSummaryByName(systemName string) (*edGalaxy.SystemSummary, bool) {
	eddbInfo := s.eddbInfo.Load().(*eddb.EDDBInfo)
	if eddbInfo != nil {
		info, ok := eddbInfo.SystemSummaryByName(systemName)
		if ok {
			return info, true
		}
	}

	ch := make(edGalaxy.SystemSummaryReplyChan)
	go s.edsmc.SystemSummaryByName(systemName, ch)
	rpl := <-ch
	if rpl.Err != nil {
		log.Printf("EDSM request failed: %v", rpl.Err)
		return nil, false
	}
	return rpl.System, true
}

func fmtUnknownSystem(nm string) string {
	return fmt.Sprintf("System '%s' is not known to me", nm)
}

func fmtNonHabitableSystem(nm string) string {
	return fmt.Sprintf("System '%s' is not habitable", nm)
}

func galaxyPoint2pb(p *edGalaxy.Point3D) *pb.Point3D {
	if p == nil {
		return nil
	}
	return &pb.Point3D{X: p.X, Y: p.Y, Z: p.Z}
}

func galaxyBriefInfo2pbPopInfo(i *edGalaxy.BriefSystemInfo) *pb.PopulatedSystemBriefInfo {
	if i == nil {
		return nil
	}
	return &pb.PopulatedSystemBriefInfo{
		Allegiance:   i.Allegiance,
		Government:   i.Government,
		Faction:      i.Faction,
		FactionState: i.FactionState,
		Population:   i.Population,
		Reserve:      i.Reserve,
		Security:     i.Security,
		Economy:      i.Economy}
}

func galaxyShortFactionState2pb(s *edGalaxy.ShortFactionState) *pb.ShortFactionState {
	if s == nil {
		return nil
	}
	return &pb.ShortFactionState{Name: s.Name, State: s.State, Allegiance: s.Allegiance}
}
func galaxySystemVisitsStat2pb(coords *edGalaxy.Point3D, stat []*edGalaxy.SystemVisitsStat) []*pb.SystemVisitsStat {
	if stat == nil {
		return nil
	}
	rv := make([]*pb.SystemVisitsStat, len(stat))
	for i, s := range stat {
		rv[i] = &pb.SystemVisitsStat{
			Name:     s.Name,
			Count:    s.Count,
			Distance: coords.Distance(s.Coords)}
	}
	return rv
}

func galaxyActivityStatItem2pb(gstat []*edGalaxy.ActivityStatItem) ([]*pb.ActivityStatItem) {
	pbStat := make([]*pb.ActivityStatItem, len(gstat))
	for i, s := range gstat {
		pbStat[i] = &pb.ActivityStatItem { Timestamp: s.Timestamp, NumJumps: s.NumJumps, NumDocks: s.NumDocks }
	}
	return pbStat
}

func galaxyInterestingSystem4State2pb(s *edGalaxy.InterestingSystem4State) *pb.InterestingSystem4State {
	if s == nil {
		return nil
	}
	pbFactions := make([]*pb.ShortFactionState, len(s.Factions))
	for i, f := range s.Factions {
		pbFactions[i] = galaxyShortFactionState2pb(f)
	}
	return &pb.InterestingSystem4State{
		Name:          s.Name,
		Population:    s.Population,
		Coords:        galaxyPoint2pb(s.Coords),
		FactionStates: pbFactions}
}

func galaxyDockableStationShortInfo2pb(s *edGalaxy.DockableStationShortInfo) *pb.DockableStationShortInfo {
	if s == nil {
		return nil
	}
	return &pb.DockableStationShortInfo{
		Name:       s.Name,
		LandingPad: s.LandingPad,
		Distance:   s.Distance,
		Planetary:  s.Planetary}
}

func (p *grpcProcessor) GetDistance(ctx context.Context, in *pb.SystemsDistanceRequest) (*pb.SystemsDistanceReply, error) {
	nm := in.GetName1()
	c1, known := p.gi.getSystemCoords(nm)
	if !known {
		return &pb.SystemsDistanceReply{Error: fmtUnknownSystem(nm)}, nil
	}
	nm = in.GetName2()
	c2, known := p.gi.getSystemCoords(nm)
	if !known {
		return &pb.SystemsDistanceReply{Error: fmtUnknownSystem(nm)}, nil
	}
	return &pb.SystemsDistanceReply{Distance: c1.Distance(c2)}, nil
}

func (p *grpcProcessor) GetSystemSummary(ctx context.Context, in *pb.SystemByNameRequest) (*pb.SystemSummaryReply, error) {
	nm := in.GetName()
	ss, known := p.gi.getSystemSummaryByName(nm)
	if !known {
		return &pb.SystemSummaryReply{Error: fmtUnknownSystem(nm)}, nil
	}

	pbss := pb.SystemSummary{
		Name:          ss.Name,
		Coords:        galaxyPoint2pb(ss.Coords),
		PopSystemInfo: galaxyBriefInfo2pbPopInfo(ss.BriefInfo)}

	return &pb.SystemSummaryReply{Summary: &pbss}, nil
}

func (p *grpcProcessor) GetHumanWorldStat(ctx context.Context, _ *empty.Empty) (*pb.HumanWorldStat, error) {
	eddbInfo := p.gi.eddbInfo.Load().(*eddb.EDDBInfo)
	if eddbInfo == nil {
		return nil, errors.New("EDDB processor is not (yet) available")
	}

	ws := eddbInfo.GetHumanWorldStat()
	if ws == nil {
		log.Println("Unexpected nil stat\n")
		return &pb.HumanWorldStat{}, nil
	}
	return &pb.HumanWorldStat{
		Systems:       ws.Systems,
		Stations:      ws.Stations,
		Factions:      ws.Factions,
		HumanFactions: ws.HumanFactions,
		Population:    ws.Population}, nil
}

func (p *grpcProcessor) GetDockableStations(ctx context.Context, in *pb.SystemByNameRequest) (*pb.DockableStationsReply, error) {
	eddbInfo := p.gi.eddbInfo.Load().(*eddb.EDDBInfo)
	if eddbInfo == nil {
		return &pb.DockableStationsReply{Error: "EDDB processor is not (yet) available"}, nil
	}
	eddbStations, known := eddbInfo.GetDockableStations(in.GetName())
	if !known {
		suggested := eddbInfo.GetSimilarSystemNames(in.GetName())
		if len(suggested) > 0 {
			if len(suggested) > 10 {
				suggested = suggested[:10]
			}
		}
		return &pb.DockableStationsReply{Error: fmtNonHabitableSystem(in.GetName()), SuggestedSystems: suggested}, nil
	}
	sz := len(eddbStations)
	pbStations := make([]*pb.DockableStationShortInfo, sz)
	for i := 0; i < sz; i++ {
		pbStations[i] = galaxyDockableStationShortInfo2pb(eddbStations[i])
	}
	return &pb.DockableStationsReply{Stations: pbStations}, nil
}

func (p *grpcProcessor) GetInterestingSystem4State(ctx context.Context, in *pb.InterestingSystem4StateRequest) (*pb.InterestingSystem4StateReply, error) {
	eddbInfo := p.gi.eddbInfo.Load().(*eddb.EDDBInfo)
	if eddbInfo == nil {
		return &pb.InterestingSystem4StateReply{Error: "EDDB processor is not (yet) available"}, nil
	}
	nm := in.GetName()
	place, known := p.gi.getSystemCoords(nm)
	if !known {
		return &pb.InterestingSystem4StateReply{Error: fmtUnknownSystem(nm)}, nil
	}
	states := in.GetStates()
	if states == nil || len(states) == 0 {
		return &pb.InterestingSystem4StateReply{Error: "Empty states"}, nil
	}
	minPop := in.GetMinPop()
	if minPop < 1 {
		return &pb.InterestingSystem4StateReply{Error: "Zero population"}, nil
	}
	maxDistance := in.GetMaxDistance()
	res := eddbInfo.FindStates(states, place, minPop, maxDistance, 20)

	pbPlaces := make([]*pb.InterestingSystem4State, len(res))
	for i, r := range res {
		pbPlaces[i] = galaxyInterestingSystem4State2pb(r)
	}
	return &pb.InterestingSystem4StateReply{Systems: pbPlaces}, nil
}

func (p *grpcProcessor) GetMostVisitedSystems(ctx context.Context, in *pb.MostVisitedSystemsRequest) (*pb.MostVisitedSystemsReply, error) {
	nm := in.GetOrigin()
	coords, known := p.gi.getSystemCoords(nm)
	if !known {
		return &pb.MostVisitedSystemsReply{Error: fmtUnknownSystem(nm)}, nil
	}
	if p.gi.visitsStatProvider == nil {
		return &pb.MostVisitedSystemsReply{Error: "Stat collector is not set"}, nil
	}

	stat, total, err := p.gi.visitsStatProvider.GetSystemVisitsStat(coords, in.GetMaxDistance(), int(in.GetLimit()))
	if err != nil {
		log.Printf("GetSystemVisitsStat failed: %v", err)
		return &pb.MostVisitedSystemsReply{Error: "Stat collector eroor detected"}, nil
	}

	return &pb.MostVisitedSystemsReply{SystemVisitStat: galaxySystemVisitsStat2pb(coords, stat),
		TotalCount: total}, nil
}

func (p *grpcProcessor) GetGalaxyActivityStat(ctx context.Context, in *pb.ActivityStatRequest) (*pb.ActivityStatReply, error) {

	known := false
	coords := edGalaxy.Sol
	

	nm := in.GetOrigin()
	if len(nm) > 1 {
		if coords, known = p.gi.getSystemCoords(nm); !known {
			return &pb.ActivityStatReply{Error: fmtUnknownSystem(nm)}, nil
		}
	}

	if p.gi.visitsStatProvider == nil {
		return &pb.ActivityStatReply{Error: "Stat collector is not set"}, nil
	}

	stat := p.gi.visitsStatProvider.GetActivityStat(coords, in.GetMaxDistance())

	return &pb.ActivityStatReply{StatItems: galaxyActivityStatItem2pb(stat) }, nil
}

func (s *GIServer) Serve() error {
	lis, err := net.Listen("tcp", s.cfg.Port)
	if err != nil {
		log.Printf("failed to listen: %v", err)
		return err
	}
	s.s = grpc.NewServer()
	pb.RegisterEDInfoCenterServer(s.s, &grpcProcessor{gi: s})
	reflection.Register(s.s)

	log.Printf("GIServer grpc: serving on %s\n", s.cfg.Port)
	if err := s.s.Serve(lis); err != nil {
		log.Printf("failed to serve: %v", err)
	}
	return err
}
