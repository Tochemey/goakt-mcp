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

package grpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	ingressgrpc "github.com/tochemey/goakt-mcp/internal/ingress/grpc"
	pb "github.com/tochemey/goakt-mcp/internal/ingress/grpc/pb"
	"github.com/tochemey/goakt-mcp/mcp"
)

// bufconnBufSize is the buffer size for the in-process gRPC listener.
const bufconnBufSize = 1024 * 1024

// --- test doubles ------------------------------------------------------------

// fixedGRPCResolver always returns the configured identity.
type fixedGRPCResolver struct {
	tenantID mcp.TenantID
	clientID mcp.ClientID
}

func (r *fixedGRPCResolver) ResolveGRPCIdentity(_ context.Context) (mcp.TenantID, mcp.ClientID, error) {
	return r.tenantID, r.clientID, nil
}

// errGRPCResolver always returns an error, causing request rejection.
type errGRPCResolver struct{}

func (r *errGRPCResolver) ResolveGRPCIdentity(_ context.Context) (mcp.TenantID, mcp.ClientID, error) {
	return "", "", errors.New("unauthorized")
}

// fakeStreamInvoker implements [pkg.StreamInvoker] for tests.
type fakeStreamInvoker struct {
	tools        []mcp.Tool
	result       *mcp.ExecutionResult
	err          error
	listErr      error
	streamResult *mcp.StreamingResult
	streamErr    error
}

func (f *fakeStreamInvoker) Invoke(_ context.Context, _ *mcp.Invocation) (*mcp.ExecutionResult, error) {
	return f.result, f.err
}

func (f *fakeStreamInvoker) ListTools(_ context.Context) ([]mcp.Tool, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.tools, nil
}

func (f *fakeStreamInvoker) InvokeStream(_ context.Context, _ *mcp.Invocation) (*mcp.StreamingResult, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return f.streamResult, nil
}

// --- helpers -----------------------------------------------------------------

// newTestClient spins up an in-process gRPC server via bufconn and returns a
// connected MCPToolService client. It registers t.Cleanup to close both.
func newTestClient(
	t *testing.T,
	gw *fakeStreamInvoker,
	resolver mcp.GRPCIdentityResolver,
) pb.MCPToolServiceClient {
	t.Helper()

	lis := bufconn.Listen(bufconnBufSize)

	srv := grpc.NewServer()
	svc, err := ingressgrpc.NewServer(gw, mcp.GRPCIngressConfig{
		IdentityResolver: resolver,
	})
	require.NoError(t, err)
	pb.RegisterMCPToolServiceServer(srv, svc)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Server stopped; expected during cleanup.
			return
		}
	}()
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient(
		"passthrough://bufconn",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return pb.NewMCPToolServiceClient(conn)
}

// --- tests -------------------------------------------------------------------

func TestNewServer_NilGateway(t *testing.T) {
	_, err := ingressgrpc.NewServer(nil, mcp.GRPCIngressConfig{
		IdentityResolver: &fixedGRPCResolver{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway must not be nil")
}

func TestNewServer_NilIdentityResolver(t *testing.T) {
	_, err := ingressgrpc.NewServer(&fakeStreamInvoker{}, mcp.GRPCIngressConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IdentityResolver must not be nil")
}

func TestListTools_Success(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{
			{ID: "echo"},
			{
				ID: "multi-tool",
				Schemas: []mcp.ToolSchema{
					{Name: "read_file", Description: "Read a file", InputSchema: []byte(`{"type":"object"}`)},
					{Name: "write_file", Description: "Write a file", InputSchema: []byte(`{"type":"object"}`)},
				},
			},
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	resp, err := client.ListTools(context.Background(), &pb.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetTools(), 2)

	// First tool: simple, no schemas.
	assert.Equal(t, "echo", resp.GetTools()[0].GetId())
	assert.Empty(t, resp.GetTools()[0].GetSchemas())

	// Second tool: multi-schema.
	assert.Equal(t, "multi-tool", resp.GetTools()[1].GetId())
	require.Len(t, resp.GetTools()[1].GetSchemas(), 2)
	assert.Equal(t, "read_file", resp.GetTools()[1].GetSchemas()[0].GetName())
	assert.Equal(t, "write_file", resp.GetTools()[1].GetSchemas()[1].GetName())
}

func TestListTools_Error(t *testing.T) {
	gw := &fakeStreamInvoker{listErr: errors.New("registry unavailable")}
	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	_, err := client.ListTools(context.Background(), &pb.ListToolsRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCallTool_Success(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "echo"}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "pong"},
				},
			},
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	args, _ := json.Marshal(map[string]any{"msg": "ping"})
	resp, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName:  "echo",
		Arguments: args,
	})
	require.NoError(t, err)
	assert.False(t, resp.GetIsError())
	require.Len(t, resp.GetContent(), 1)
	assert.Equal(t, "text", resp.GetContent()[0].GetType())
	assert.Equal(t, "pong", resp.GetContent()[0].GetText())
	assert.NotEmpty(t, resp.GetStructuredContent())
}

func TestCallTool_Error(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "fail-tool"}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusFailure,
			Err:    mcp.NewRuntimeError(mcp.ErrCodeInternal, "backend error"),
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	resp, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "fail-tool",
	})
	require.NoError(t, err)
	assert.True(t, resp.GetIsError())
	require.Len(t, resp.GetContent(), 1)
	assert.Contains(t, resp.GetContent()[0].GetText(), "backend error")
}

func TestCallTool_IdentityResolutionFailure(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{{ID: "echo"}}}
	client := newTestClient(t, gw, &errGRPCResolver{})

	_, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "echo",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCallTool_ToolNotFound(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{{ID: "echo"}}}
	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	_, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "nonexistent",
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestCallTool_InvalidArguments(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{{ID: "echo"}}}
	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	_, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName:  "echo",
		Arguments: []byte(`{invalid json`),
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCallTool_MultiSchemaToolResolvesSubTool(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{
			ID: "multi-tool",
			Schemas: []mcp.ToolSchema{
				{Name: "read_file", Description: "Read a file"},
				{Name: "write_file", Description: "Write a file"},
			},
		}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusSuccess,
			Output: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "file contents"},
				},
			},
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	args, _ := json.Marshal(map[string]any{"path": "/tmp/test"})
	resp, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName:  "read_file",
		Arguments: args,
	})
	require.NoError(t, err)
	assert.False(t, resp.GetIsError())
	require.Len(t, resp.GetContent(), 1)
	assert.Equal(t, "file contents", resp.GetContent()[0].GetText())
}

func TestCallToolStream_Success(t *testing.T) {
	progressCh := make(chan mcp.ProgressEvent, 2)
	finalCh := make(chan *mcp.ExecutionResult, 1)

	// Send two progress events and a final result.
	progressCh <- mcp.ProgressEvent{Message: "step 1", Progress: 1, Total: 3}
	progressCh <- mcp.ProgressEvent{Message: "step 2", Progress: 2, Total: 3}
	close(progressCh)

	finalCh <- &mcp.ExecutionResult{
		Status: mcp.ExecutionStatusSuccess,
		Output: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "done"},
			},
		},
	}
	close(finalCh)

	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "stream-tool"}},
		streamResult: &mcp.StreamingResult{
			Progress: progressCh,
			Final:    finalCh,
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	stream, err := client.CallToolStream(context.Background(), &pb.CallToolStreamRequest{
		ToolName: "stream-tool",
	})
	require.NoError(t, err)

	// Collect all stream messages.
	var progressEvents []*pb.ProgressEvent
	var finalResult *pb.CallToolResponse

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		switch p := msg.GetPayload().(type) {
		case *pb.CallToolStreamResponse_Progress:
			progressEvents = append(progressEvents, p.Progress)
		case *pb.CallToolStreamResponse_Result:
			finalResult = p.Result
		}
	}

	// Verify progress events.
	require.Len(t, progressEvents, 2)
	assert.Equal(t, "step 1", progressEvents[0].GetMessage())
	assert.Equal(t, float64(1), progressEvents[0].GetProgress())
	assert.Equal(t, float64(3), progressEvents[0].GetTotal())
	assert.Equal(t, "step 2", progressEvents[1].GetMessage())
	assert.Equal(t, float64(2), progressEvents[1].GetProgress())

	// Verify final result.
	require.NotNil(t, finalResult)
	assert.False(t, finalResult.GetIsError())
	require.Len(t, finalResult.GetContent(), 1)
	assert.Equal(t, "done", finalResult.GetContent()[0].GetText())
}

func TestCallToolStream_IdentityResolutionFailure(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{{ID: "stream-tool"}}}
	client := newTestClient(t, gw, &errGRPCResolver{})

	stream, err := client.CallToolStream(context.Background(), &pb.CallToolStreamRequest{
		ToolName: "stream-tool",
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCallToolStream_ToolNotFound(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{{ID: "echo"}}}
	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	stream, err := client.CallToolStream(context.Background(), &pb.CallToolStreamRequest{
		ToolName: "nonexistent",
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestCallToolStream_InvokeStreamError(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools:     []mcp.Tool{{ID: "stream-tool"}},
		streamErr: errors.New("stream failed"),
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	stream, err := client.CallToolStream(context.Background(), &pb.CallToolStreamRequest{
		ToolName: "stream-tool",
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCallToolStream_NoProgressEvents(t *testing.T) {
	// When the executor does not support streaming, the progress channel is
	// immediately closed and only the final result is delivered.
	progressCh := make(chan mcp.ProgressEvent)
	close(progressCh)

	finalCh := make(chan *mcp.ExecutionResult, 1)
	finalCh <- &mcp.ExecutionResult{
		Status: mcp.ExecutionStatusSuccess,
		Output: map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "immediate result"},
			},
		},
	}
	close(finalCh)

	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "sync-tool"}},
		streamResult: &mcp.StreamingResult{
			Progress: progressCh,
			Final:    finalCh,
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	stream, err := client.CallToolStream(context.Background(), &pb.CallToolStreamRequest{
		ToolName: "sync-tool",
	})
	require.NoError(t, err)

	// Should receive exactly one message: the final result.
	msg, err := stream.Recv()
	require.NoError(t, err)

	result, ok := msg.GetPayload().(*pb.CallToolStreamResponse_Result)
	require.True(t, ok)
	assert.False(t, result.Result.GetIsError())
	assert.Equal(t, "immediate result", result.Result.GetContent()[0].GetText())

	// Stream should be exhausted.
	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)
}

func TestCallTool_NilResult(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools:  []mcp.Tool{{ID: "echo"}},
		result: nil,
		err:    errors.New("gateway error"),
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	_, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "echo",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestCallTool_EmptyTools(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{}}
	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	resp, err := client.ListTools(context.Background(), &pb.ListToolsRequest{})
	require.NoError(t, err)
	assert.Empty(t, resp.GetTools())
}

func TestCallTool_ToolErrorWithStatus(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "echo"}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusTimeout,
			Err:    mcp.NewRuntimeError(mcp.ErrCodeInvocationTimeout, "timed out"),
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	resp, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "echo",
	})
	require.NoError(t, err)
	assert.True(t, resp.GetIsError())
	assert.Contains(t, resp.GetContent()[0].GetText(), "timed out")
}

func TestCallTool_DeniedResult(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "echo"}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusDenied,
			Err:    mcp.NewRuntimeError(mcp.ErrCodePolicyDenied, "access denied"),
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	resp, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "echo",
	})
	require.NoError(t, err)
	assert.True(t, resp.GetIsError())
	assert.Contains(t, resp.GetContent()[0].GetText(), "access denied")
}

func TestCallTool_ThrottledResult(t *testing.T) {
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "echo"}},
		result: &mcp.ExecutionResult{
			Status: mcp.ExecutionStatusThrottled,
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	resp, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "echo",
	})
	require.NoError(t, err)
	assert.True(t, resp.GetIsError())
	assert.Contains(t, resp.GetContent()[0].GetText(), "throttled")
}

func TestCallTool_EmptyToolName(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{{ID: "echo"}}}
	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	_, err := client.CallTool(context.Background(), &pb.CallToolRequest{
		ToolName: "",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "tool_name must not be empty")
}

func TestCallToolStream_EmptyToolName(t *testing.T) {
	gw := &fakeStreamInvoker{tools: []mcp.Tool{{ID: "echo"}}}
	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	stream, err := client.CallToolStream(context.Background(), &pb.CallToolStreamRequest{
		ToolName: "",
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListTools_InputSchemaRoundTrip(t *testing.T) {
	inputSchema := []byte(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)
	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{
			ID: "fs-tool",
			Schemas: []mcp.ToolSchema{
				{Name: "read_file", Description: "Read a file", InputSchema: inputSchema},
			},
		}},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	resp, err := client.ListTools(context.Background(), &pb.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetTools(), 1)
	require.Len(t, resp.GetTools()[0].GetSchemas(), 1)

	schema := resp.GetTools()[0].GetSchemas()[0]
	assert.Equal(t, "read_file", schema.GetName())
	assert.Equal(t, "Read a file", schema.GetDescription())
	assert.JSONEq(t, string(inputSchema), string(schema.GetInputSchema()))
}

func TestCallToolStream_ErrorResult(t *testing.T) {
	progressCh := make(chan mcp.ProgressEvent)
	close(progressCh)

	finalCh := make(chan *mcp.ExecutionResult, 1)
	finalCh <- &mcp.ExecutionResult{
		Status: mcp.ExecutionStatusFailure,
		Err:    mcp.NewRuntimeError(mcp.ErrCodeInternal, "stream backend error"),
	}
	close(finalCh)

	gw := &fakeStreamInvoker{
		tools: []mcp.Tool{{ID: "stream-tool"}},
		streamResult: &mcp.StreamingResult{
			Progress: progressCh,
			Final:    finalCh,
		},
	}

	client := newTestClient(t, gw, &fixedGRPCResolver{tenantID: "acme", clientID: "c1"})

	stream, err := client.CallToolStream(context.Background(), &pb.CallToolStreamRequest{
		ToolName: "stream-tool",
	})
	require.NoError(t, err)

	msg, err := stream.Recv()
	require.NoError(t, err)

	result, ok := msg.GetPayload().(*pb.CallToolStreamResponse_Result)
	require.True(t, ok)
	assert.True(t, result.Result.GetIsError())
	assert.Contains(t, result.Result.GetContent()[0].GetText(), "stream backend error")
}
