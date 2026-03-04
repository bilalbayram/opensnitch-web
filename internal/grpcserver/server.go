package grpcserver

import (
	"fmt"
	"log"
	"net"

	pb "github.com/evilsocket/opensnitch-web/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"time"
)

type Server struct {
	grpcServer *grpc.Server
	service    *UIService
}

func New(service *UIService) *Server {
	kasp := keepalive.ServerParameters{
		Time:    10 * time.Second,
		Timeout: 20 * time.Second,
	}
	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second,
		PermitWithoutStream: true,
	}

	s := grpc.NewServer(
		grpc.KeepaliveParams(kasp),
		grpc.KeepaliveEnforcementPolicy(kaep),
	)

	pb.RegisterUIServer(s, service)

	return &Server{
		grpcServer: s,
		service:    service,
	}
}

func (s *Server) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("gRPC listen on %s: %w", addr, err)
	}
	log.Printf("[grpc] Listening on %s", addr)
	return s.grpcServer.Serve(lis)
}

func (s *Server) ListenUnix(path string) error {
	lis, err := net.Listen("unix", path)
	if err != nil {
		return fmt.Errorf("gRPC listen on unix:%s: %w", path, err)
	}
	log.Printf("[grpc] Listening on unix:%s", path)
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}
