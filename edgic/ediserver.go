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
	"sync/atomic"
)

type GrpcServerConf struct {
	Port    string
	Enabled bool
}

type GIServer struct {
	eddbInfo atomic.Value
	cfg      GrpcServerConf
	s        *grpc.Server
}

type grpcProcessor struct {
	gi *GIServer
}

func NewGIServer(cfg GrpcServerConf) *GIServer {
	return &GIServer{cfg: cfg}
}

func (s *GIServer) SetEDDBData(data *eddb.EDDBInfo) {
	s.eddbInfo.Store(data)
}

func (s *GIServer) getSystemCoords(systemName string) (*edGalaxy.Point3D, bool) {
	eddbInfo := s.eddbInfo.Load().(*eddb.EDDBInfo)
	if eddbInfo != nil {
		c, ok := eddbInfo.GetSystemCoordsByName(systemName)
		if ok {
			return c, true
		}
	}
	return nil, false
}

func (p *grpcProcessor) GetDistance(ctx context.Context, in *pb.SystemsDistanceRequest) (*pb.SystemsDistanceReply, error) {
	nm := in.GetName1()
	c1, known := p.gi.getSystemCoords(nm)
	if !known {
		return &pb.SystemsDistanceReply{Error: fmt.Sprintf("System '%s' is not known to me", nm)}, nil
	}
	nm = in.GetName2()
	c2, known := p.gi.getSystemCoords(nm)
	if !known {
		return &pb.SystemsDistanceReply{Error: fmt.Sprintf("System '%s' is not known to me", nm)}, nil
	}
	return &pb.SystemsDistanceReply{Distance: c1.Distance(c2)}, nil
}

func (p *grpcProcessor) GetSystemSummary(ctx context.Context, in *pb.SystemByNameRequest) (*pb.SystemSummaryReply, error) {
	return &pb.SystemSummaryReply{Error: "Not implemented"}, nil
}

func (p *grpcProcessor) GetDockableStations(ctx context.Context, in *pb.SystemByNameRequest) (*pb.DockableStationsReply, error) {
	eddbInfo := s.eddbInfo.Load().(*eddb.EDDBInfo)
	if eddbInfo == nil {
		return &pb.DockableStationsReply{Error: "EDDB processor is not (yet) available"}, nil
	}
	return &pb.DockableStationsReply{Error: "Not implemented"}, nil
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

	log.Printf("GIServer grps: serving on %s\n", s.cfg.Port)
	if err := s.s.Serve(lis); err != nil {
		log.Printf("failed to serve: %v", err)
	}
	return err
}
