package edgic

import (
	"errors"
	"log"
	"time"

	pb "goed/api/protobuf-spec"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type EDInfoCenterClient struct {
	addr string
}

func NewEDInfoCenterClient(addr string) *EDInfoCenterClient {
	return &EDInfoCenterClient{addr: addr}
}

func (cc *EDInfoCenterClient) GetDistance(name1 string, name2 string) (float64, error) {
	if( len(cc.addr) < 5 ){
		return 0, errors.New("Galaxy information server is not configured")
	}
	log.Printf("Dialing info center '%s'\n", cc.addr)
	conn, err := grpc.Dial(cc.addr, grpc.WithInsecure())
	if err != nil {
		log.Printf("did not connect: %v", err)
		return 0, errors.New("Galaxy information server is not available")
	}
	defer conn.Close()
	c := pb.NewEDInfoCenterClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rpl, err := c.GetDistance(ctx, &pb.SystemsDistanceRequest{Name1: name1, Name2: name2})
	if err != nil {
		log.Printf("Could not get distance: %v", err)
		return 0, errors.New("Galaxy information server malfunction")
	}

	if len(rpl.Error) != 0 {
		return 0, errors.New(rpl.Error)
	}

	return rpl.GetDistance(), nil
}
