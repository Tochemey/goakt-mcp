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

// ResourceSchema holds resource metadata discovered from a backend MCP server
// via the resources/list method.
type ResourceSchema struct {
	// URI is the resource identifier as defined by the MCP specification.
	URI string
	// Name is the human-readable name of the resource.
	Name string
	// Description is the human-readable description of the resource.
	Description string
	// MIMEType is the MIME type of the resource content.
	MIMEType string
}

// ResourceTemplateSchema holds resource template metadata discovered from a
// backend MCP server via the resources/templates/list method.
type ResourceTemplateSchema struct {
	// URITemplate is the URI template as defined by RFC 6570.
	URITemplate string
	// Name is the human-readable name of the resource template.
	Name string
	// Description is the human-readable description of the resource template.
	Description string
	// MIMEType is the MIME type of the resource content.
	MIMEType string
}

