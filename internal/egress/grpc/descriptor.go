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

package grpc

import (
	"context"
	"fmt"
	"io"
	"os"

	"google.golang.org/grpc"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// LoadDescriptorSet reads a binary-encoded FileDescriptorSet from the given
// file path. The file is typically produced by:
//
//	protoc --descriptor_set_out=service.binpb --include_imports service.proto
//	buf build -o service.binpb
func LoadDescriptorSet(path string) (*descriptorpb.FileDescriptorSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read descriptor set %q: %w", path, err)
	}
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(data, fds); err != nil {
		return nil, fmt.Errorf("unmarshal descriptor set %q: %w", path, err)
	}
	return fds, nil
}

// FetchDescriptorSetViaReflection uses gRPC server reflection to obtain the
// file descriptors for the named service from a live backend. The returned
// FileDescriptorSet contains the service's own file descriptor and all of its
// transitive dependencies.
func FetchDescriptorSetViaReflection(ctx context.Context, conn *grpc.ClientConn, service string) (*descriptorpb.FileDescriptorSet, error) {
	client := rpb.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("open reflection stream: %w", err)
	}
	defer stream.CloseSend() //nolint:errcheck

	// Request the file containing the target service symbol.
	if err := stream.Send(&rpb.ServerReflectionRequest{
		MessageRequest: &rpb.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: service,
		},
	}); err != nil {
		return nil, fmt.Errorf("send reflection request: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv reflection response: %w", err)
	}

	fdResp := resp.GetFileDescriptorResponse()
	if fdResp == nil {
		if errResp := resp.GetErrorResponse(); errResp != nil {
			return nil, fmt.Errorf("reflection error: %s", errResp.GetErrorMessage())
		}
		return nil, fmt.Errorf("unexpected reflection response for service %q", service)
	}

	fds := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(fdResp.GetFileDescriptorProto())),
	}
	for _, raw := range fdResp.GetFileDescriptorProto() {
		fdp := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(raw, fdp); err != nil {
			return nil, fmt.Errorf("unmarshal reflected file descriptor: %w", err)
		}
		fds.File = append(fds.File, fdp)
	}

	// Fetch transitive dependencies that were not included in the initial response.
	if err := fetchDependencies(ctx, stream, fds); err != nil {
		return nil, err
	}

	return fds, nil
}

// fetchDependencies iteratively resolves any file descriptors referenced by
// imports but not yet present in fds. This ensures the FileDescriptorSet is
// self-contained and can be passed to protodesc.NewFiles.
func fetchDependencies(ctx context.Context, stream rpb.ServerReflection_ServerReflectionInfoClient, fds *descriptorpb.FileDescriptorSet) error {
	known := make(map[string]struct{}, len(fds.File))
	for _, f := range fds.File {
		known[f.GetName()] = struct{}{}
	}

	for {
		var missing []string
		for _, f := range fds.File {
			for _, dep := range f.GetDependency() {
				if _, ok := known[dep]; !ok {
					missing = append(missing, dep)
				}
			}
		}
		if len(missing) == 0 {
			return nil
		}

		for _, name := range missing {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("context cancelled during dependency resolution: %w", err)
			}
			if err := stream.Send(&rpb.ServerReflectionRequest{
				MessageRequest: &rpb.ServerReflectionRequest_FileByFilename{
					FileByFilename: name,
				},
			}); err != nil {
				return fmt.Errorf("send reflection request for %q: %w", name, err)
			}

			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return fmt.Errorf("reflection stream closed while resolving %q", name)
				}
				return fmt.Errorf("recv reflection response for %q: %w", name, err)
			}

			fdResp := resp.GetFileDescriptorResponse()
			if fdResp == nil {
				if errResp := resp.GetErrorResponse(); errResp != nil {
					return fmt.Errorf("reflection error for %q: %s", name, errResp.GetErrorMessage())
				}
				return fmt.Errorf("unexpected reflection response for %q", name)
			}

			for _, raw := range fdResp.GetFileDescriptorProto() {
				fdp := &descriptorpb.FileDescriptorProto{}
				if err := proto.Unmarshal(raw, fdp); err != nil {
					return fmt.Errorf("unmarshal reflected dependency %q: %w", name, err)
				}
				if _, ok := known[fdp.GetName()]; !ok {
					fds.File = append(fds.File, fdp)
					known[fdp.GetName()] = struct{}{}
				}
			}
		}
	}
}

// ResolveService locates a service by its fully-qualified name within the given
// FileDescriptorSet and returns its ServiceDescriptor. This is used when no
// specific method is configured to enumerate all RPCs in the service.
func ResolveService(fds *descriptorpb.FileDescriptorSet, service string) (protoreflect.ServiceDescriptor, error) {
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, fmt.Errorf("build file registry: %w", err)
	}
	desc, err := files.FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil, fmt.Errorf("service %q not found in descriptor set: %w", service, err)
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not a service descriptor", service)
	}
	return sd, nil
}

// ResolveMethod locates a specific RPC method within a service and returns the
// input and output message descriptors. The method parameter is the unqualified
// RPC name (e.g. "Charge", not "payments.v1.PaymentService/Charge").
func ResolveMethod(fds *descriptorpb.FileDescriptorSet, service, method string) (input, output protoreflect.MessageDescriptor, err error) {
	sd, err := ResolveService(fds, service)
	if err != nil {
		return nil, nil, err
	}
	md := sd.Methods().ByName(protoreflect.Name(method))
	if md == nil {
		return nil, nil, fmt.Errorf("method %q not found in service %q", method, service)
	}
	return md.Input(), md.Output(), nil
}
