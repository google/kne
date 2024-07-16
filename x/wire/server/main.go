package main

import (
	"flag"
	"fmt"
	"net"

	log "github.com/golang/glog"
	wpb "github.com/openconfig/kne/proto/wire"
	"github.com/openconfig/kne/x/wire"
	"google.golang.org/grpc"
)

var (
	port = flag.Int("port", 50058, "Wire server port")
)

type server struct {
	wpb.UnimplementedWireServer
	endpoints map[wire.PhysicalEndpoint]*wire.Wire
}

func newServer(endpoints map[wire.PhysicalEndpoint]*wire.Wire) *server {
	return &server{endpoints: endpoints}
}

func (s *server) Transmit(stream wpb.Wire_TransmitServer) error {
	pe, err := wire.ParsePhysicalEndpoint(stream.Context())
	if err != nil {
		return fmt.Errorf("unable to parse physical endpoint from incoming stream context: %v", err)
	}
	log.Infof("New Transmit stream started for endpoint %v", pe)
	w, ok := s.endpoints[*pe]
	if !ok {
		return fmt.Errorf("no endpoint found on server for request: %v", pe)
	}
	if err := w.Transmit(stream.Context(), stream); err != nil {
		return fmt.Errorf("transmit failed: %v", err)
	}
	return nil
}

func main() {
	flag.Parse()
	addr := fmt.Sprintf(":%d", *port)
	lis, err := net.Listen("tcp6", addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	frw1, err := wire.NewFileReadWriter("testdata/xx01sql17src.txt", "testdata/xx01sql17dst.txt")
	if err != nil {
		log.Fatalf("Failed to create file based read/writer: %v", err)
	}
	defer frw1.Close()
	frw2, err := wire.NewFileReadWriter("testdata/xx02sql17src.txt", "testdata/xx02sql17dst.txt")
	if err != nil {
		log.Fatalf("Failed to create file based read/writer: %v", err)
	}
	defer frw2.Close()
	endpoints := map[wire.PhysicalEndpoint]*wire.Wire{
		*wire.NewPhysicalEndpoint("xx01.sql17", "Ethernet0/0/0/0"): wire.NewWire(frw1),
		*wire.NewPhysicalEndpoint("xx02.sql17", "Ethernet0/0/0/1"): wire.NewWire(frw2),
	}
	wpb.RegisterWireServer(s, newServer(endpoints))
	log.Infof("Wire server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
