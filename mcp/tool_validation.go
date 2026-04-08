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

package mcp

// ValidateTool checks that a tool definition is valid before registration.
//
// Validation rules:
//   - Tool.ID must not be zero (empty)
//   - Transport must be TransportStdio, TransportHTTP, or TransportGRPC
//   - For stdio: Command must be non-empty
//   - For http: URL must be non-empty
//   - For grpc: Target and Service must be non-empty; exactly one of
//     DescriptorSet or Reflection must be set
func ValidateTool(tool Tool) error {
	if tool.ID.IsZero() {
		return NewRuntimeError(ErrCodeInvalidRequest, "tool ID is required")
	}

	switch tool.Transport {
	case TransportStdio:
		if tool.Stdio == nil || tool.Stdio.Command == "" {
			return NewRuntimeError(ErrCodeInvalidRequest, "stdio tool must have non-empty command")
		}
	case TransportHTTP:
		if tool.HTTP == nil || tool.HTTP.URL == "" {
			return NewRuntimeError(ErrCodeInvalidRequest, "http tool must have non-empty URL")
		}
	case TransportGRPC:
		if err := validateGRPCTool(tool); err != nil {
			return err
		}
	default:
		return NewRuntimeError(ErrCodeInvalidRequest, "transport must be stdio, http, or grpc")
	}
	return nil
}

// validateGRPCTool checks gRPC-specific tool configuration constraints.
func validateGRPCTool(tool Tool) error {
	if tool.GRPC == nil {
		return NewRuntimeError(ErrCodeInvalidRequest, "grpc tool must have non-nil GRPC config")
	}
	if tool.GRPC.Target == "" {
		return NewRuntimeError(ErrCodeInvalidRequest, "grpc tool must have non-empty target")
	}
	if tool.GRPC.Service == "" {
		return NewRuntimeError(ErrCodeInvalidRequest, "grpc tool must have non-empty service")
	}
	hasDescriptor := tool.GRPC.DescriptorSet != ""
	hasReflection := tool.GRPC.Reflection
	if hasDescriptor && hasReflection {
		return NewRuntimeError(ErrCodeInvalidRequest, "grpc tool must set DescriptorSet or Reflection, not both")
	}
	if !hasDescriptor && !hasReflection {
		return NewRuntimeError(ErrCodeInvalidRequest, "grpc tool must set either DescriptorSet or Reflection")
	}
	return nil
}
