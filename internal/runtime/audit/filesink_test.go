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

package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSink_WriteAndClose(t *testing.T) {
	dir := t.TempDir()
	sink, err := NewFileSink(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sink.Close() })

	ev := &Event{
		Type:     EventTypeInvocationComplete,
		TenantID: "tenant-1",
		ToolID:   "tool-1",
		Outcome:  "success",
	}
	require.NoError(t, sink.Write(ev))
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(filepath.Join(dir, "audit.log"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "tenant-1")
	assert.Contains(t, string(data), "tool-1")
	assert.Contains(t, string(data), "invocation_complete")
}

func TestFileSink_NewFileSink_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "audit")
	sink, err := NewFileSink(dir)
	require.NoError(t, err)
	require.NoError(t, sink.Close())
	assert.DirExists(t, dir)
}
