package edgic

import (
	"fmt"
	"log"
	"net"

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
	eddbInfo atomic.Value
	edsmc    *edsm.EDSMConnector
	cfg      GrpcServerConf
	s        *grpc.Server
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

func (p *grpcProcessor) GetDockableStations(ctx context.Context, in *pb.SystemByNameRequest) (*pb.DockableStationsReply, error) {
	eddbInfo := p.gi.eddbInfo.Load().(*eddb.EDDBInfo)
	if eddbInfo == nil {
		return &pb.DockableStationsReply{Error: "EDDB processor is not (yet) available"}, nil
	}
	eddbStations, known := eddbInfo.GetDockableStations(in.GetName())
	if !known {
		return &pb.DockableStationsReply{Error: fmtNonHabitableSystem(in.GetName())}, nil
	}
	sz := len(eddbStations)
	pbStations := make([]*pb.DockableStationShortInfo, sz)
	for i := 0; i<sz; i++ {
		pbStations[i] = galaxyDockableStationShortInfo2pb(eddbStations[i])
	}
	return &pb.DockableStationsReply{Stations: pbStations}, nil
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
