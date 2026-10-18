package pool_test

import (
	"sync"
	"testing"

	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/pkg/pool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type object struct {
	value  string
	resets int
}

func (o *object) Reset() {
	o.value = ""
	o.resets++
}

func TestNew(t *testing.T) {
	calls := 0
	p := pool.New(func() *object {
		calls++
		return &object{value: "new"}
	})
	require.NotNil(t, p)
	assert.Zero(t, calls, "factory should be called lazily")

	first := p.Get()
	second := p.Get()
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotSame(t, first, second)
	assert.Equal(t, "new", first.value)
	assert.Equal(t, 2, calls)
}

func TestNewNilFactory(t *testing.T) {
	assert.PanicsWithValue(t, "pool: nil factory", func() {
		pool.New[*object](nil)
	})
}

func TestPutResetsObject(t *testing.T) {
	// Observe Reset through a separate value, without accessing the object
	// after ownership has been transferred to the pool.
	resets := 0
	p := pool.New(func() *observedObject {
		return &observedObject{resets: &resets}
	})
	o := p.Get()
	o.value = "dirty"
	p.Put(o)
	assert.Equal(t, 1, resets)

	// sync.Pool may discard entries, so pointer identity is not guaranteed.
	got := p.Get()
	require.NotNil(t, got)
	assert.Empty(t, got.value)
	p.Put(got)
	assert.Equal(t, 2, resets)
}

type observedObject struct {
	value  string
	resets *int
}

func (o *observedObject) Reset() {
	o.value = ""
	(*o.resets)++
}

func TestPoolWithShortLink(t *testing.T) {
	p := pool.New(func() *model.ShortLink { return &model.ShortLink{} })
	link := p.Get()
	*link = model.ShortLink{ID: "id", Link: "original", UserID: 42, Deleted: true}
	p.Put(link)

	got := p.Get()
	require.NotNil(t, got)
	assert.Equal(t, model.ShortLink{}, *got)
}

func TestPoolConcurrent(t *testing.T) {
	p := pool.New(func() *object { return &object{} })
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				o := p.Get()
				if o.value != "" {
					t.Error("Get returned an object that was not reset")
				}
				o.value = "in use"
				p.Put(o)
			}
		}()
	}
	wg.Wait()
}
