// MIT License
//
// Copyright (c) 2026 GoAkt Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package testdata

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// EchoServer implements TestServiceServer for testing.
type EchoServer struct {
	UnimplementedTestServiceServer
}

// Echo returns the input message unchanged.
func (s *EchoServer) Echo(_ context.Context, req *EchoRequest) (*EchoResponse, error) {
	return &EchoResponse{
		Message:  req.GetMessage(),
		Sequence: req.GetCount(),
	}, nil
}

// StreamEcho returns the input message count times as a server stream.
func (s *EchoServer) StreamEcho(req *EchoRequest, stream grpc.ServerStreamingServer[EchoResponse]) error {
	count := int(req.GetCount())
	if count <= 0 {
		count = 1
	}
	for i := 1; i <= count; i++ {
		if err := stream.Send(&EchoResponse{
			Message:  req.GetMessage(),
			Sequence: int32(i),
		}); err != nil {
			return err
		}
	}
	return nil
}

// RichServer implements RichServiceServer for testing.
type RichServer struct {
	UnimplementedRichServiceServer
}

// Process returns a response with the input name.
func (s *RichServer) Process(_ context.Context, req *RichRequest) (*RichResponse, error) {
	return &RichResponse{Result: "processed: " + req.GetName()}, nil
}

// SelfRefServer implements SelfRefServiceServer for testing.
type SelfRefServer struct {
	UnimplementedSelfRefServiceServer
}

// Traverse returns the input node unchanged.
func (s *SelfRefServer) Traverse(_ context.Context, req *TreeNode) (*TreeNode, error) {
	return req, nil
}

// StartTestServer starts a gRPC server with all test services registered on a
// random port. It returns the server address and a cleanup function. When
// withReflection is true, gRPC server reflection is also registered.
func StartTestServer(withReflection bool) (addr string, cleanup func(), err error) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("listen: %w", err)
	}

	srv := grpc.NewServer()
	RegisterTestServiceServer(srv, &EchoServer{})
	RegisterRichServiceServer(srv, &RichServer{})
	RegisterSelfRefServiceServer(srv, &SelfRefServer{})

	if withReflection {
		reflection.Register(srv)
	}

	go srv.Serve(lis) //nolint:errcheck

	return lis.Addr().String(), func() {
		srv.GracefulStop()
		lis.Close() //nolint:errcheck
	}, nil
}
