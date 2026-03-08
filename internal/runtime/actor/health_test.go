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

package actor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tochemey/goakt-mcp/mcp"
)

func TestHealthActor(t *testing.T) {
	ctx := context.Background()

	t.Run("starts and stops cleanly", func(t *testing.T) {
		system, stop := testActorSystem(t)
		defer stop()

		pid, err := system.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(nil, 0))
		require.NoError(t, err)
		require.NotNil(t, pid)
		assert.Equal(t, mcp.ActorNameHealth, pid.Name())

		waitForActors()
	})

	t.Run("unhandles non-PostStart messages", func(t *testing.T) {
		kit, ctx := newTestKit(t)

		kit.Spawn(ctx, mcp.ActorNameHealth, newHealthChecker(nil, 0))
		waitForActors()

		pid, err := kit.ActorSystem().ActorOf(ctx, mcp.ActorNameHealth)
		require.NoError(t, err)
		require.NotNil(t, pid)
		require.NoError(t, pid.Tell(ctx, pid, "unknown-message"))
		waitForActors()
	})
}
