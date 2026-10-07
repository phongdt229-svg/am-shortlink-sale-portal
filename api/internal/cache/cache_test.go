package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mem struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (c *mem) Get(_ context.Context, k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}
func (c *mem) Set(_ context.Context, k string, v []byte, _ time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
}
func (c *mem) Del(_ context.Context, ks ...string) {}

type report struct{ Clicks int64 }

func TestDoCachesAndHits(t *testing.T) {
	c := &mem{m: map[string][]byte{}}
	var calls atomic.Int32
	fn := func(context.Context) (report, error) { calls.Add(1); return report{Clicks: 42}, nil }

	k := Key("overview", "a", 1)
	v, err := Do(context.Background(), c, "overview", k, time.Minute, fn)
	require.NoError(t, err)
	require.Equal(t, int64(42), v.Clicks)
	v, err = Do(context.Background(), c, "overview", k, time.Minute, fn)
	require.NoError(t, err)
	require.Equal(t, int64(42), v.Clicks)
	require.Equal(t, int32(1), calls.Load())
}

func TestDoDoesNotCacheErrors(t *testing.T) {
	c := &mem{m: map[string][]byte{}}
	_, err := Do(context.Background(), c, "x", "k-err", time.Minute, func(context.Context) (report, error) { return report{}, errors.New("boom") })
	require.Error(t, err)
	_, ok := c.Get(context.Background(), "k-err")
	require.False(t, ok)
}

func TestKeyDependsOnScope(t *testing.T) {
	require.NotEqual(t, Key("r", []string{"a"}), Key("r", []string{"b"}))
	require.Equal(t, Key("r", []string{"a"}, 1), Key("r", []string{"a"}, 1))
}

func TestNoopAndZeroTTLBypass(t *testing.T) {
	var calls int
	fn := func(context.Context) (int, error) { calls++; return calls, nil }
	_, _ = Do(context.Background(), Noop{}, "x", "k", time.Minute, fn)
	_, _ = Do(context.Background(), Noop{}, "x", "k", time.Minute, fn)
	_, _ = Do[int](context.Background(), &mem{m: map[string][]byte{}}, "x", "k", 0, fn)
	require.Equal(t, 3, calls)
}
