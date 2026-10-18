// Package pool provides a typed pool of reusable objects.
package pool

import "sync"

// Pool stores objects that can reset their state before reuse.
// A Pool is safe for concurrent use and must not be copied after first use.
// Objects may be removed automatically at any time; Pool is not durable storage.
// Use New to initialize a Pool.
type Pool[T interface{ Reset() }] struct {
	pool sync.Pool
}

// New creates a pool using factory when no reusable object is available.
// The factory must be non-nil, safe for concurrent calls, and return an object
// ready for use (not a nil pointer).
func New[T interface{ Reset() }](factory func() T) *Pool[T] {
	if factory == nil {
		panic("pool: nil factory")
	}
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any { return factory() },
		},
	}
}

// Get returns a pooled object or creates one using the factory.
// The caller owns the object until it is returned with Put.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put resets an object and returns it to the pool.
// The object must be valid for Reset. The caller must not use it after Put
// or return the same object more than once without obtaining it again.
func (p *Pool[T]) Put(object T) {
	object.Reset()
	p.pool.Put(object)
}
