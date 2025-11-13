package delta

import (
	"errors"
	"iter"

	"github.com/quintans/ds/collections/linkedmap"
)

// ============ Lazy ======================

type Lazy[T any] struct {
	isSet   bool
	value   T
	fn      func() (T, error)
	isDirty bool
}

func NewLazy[T any](fn func() (T, error)) *Lazy[T] {
	return &Lazy[T]{isSet: false, fn: fn}
}

func (v *Lazy[T]) Get() (T, error) {
	if v.isSet {
		return v.value, nil
	}
	value, err := v.fn()
	if err != nil {
		var zero T
		return zero, err
	}
	v.value = value
	v.isSet = true
	return v.value, nil
}

func (v *Lazy[T]) Set(value T) {
	v.value = value
	v.isSet = true
	v.isDirty = true
}

type Change[T any] struct {
	Value T
}

func (v *Lazy[T]) Change() *Change[T] {
	if v.isDirty {
		return &Change[T]{Value: v.value}
	}
	return nil
}

type Eager[T any] struct {
	Lazy[T]
}

func NewEager[T any](value T) *Eager[T] {
	return &Eager[T]{
		Lazy: Lazy[T]{
			isSet: true,
			value: value,
		},
	}
}

func (e *Eager[T]) Get() T {
	return e.value
}

// ============ Lazy Slice ======================

type Status int

const (
	Unchanged Status = iota
	Added
	Removed
	Modified
	Absent
)

type Identifiable[T comparable] interface {
	ID() T
}

type Item[T Identifiable[I], I comparable] struct {
	value  T
	status Status
}

type Slice[T Identifiable[I], I comparable] struct {
	isSet   bool
	isReset bool
	fetched *linkedmap.Map[I, Item[T, I]]
	fn      func(I) ([]T, error)
}

func NewLazySlice[T Identifiable[I], I comparable](fn func(I) ([]T, error)) *Slice[T, I] {
	return &Slice[T, I]{
		isSet:   false,
		fn:      fn,
		fetched: linkedmap.New[I, Item[T, I]](),
	}
}

func (s *Slice[T, I]) GetAll() (iter.Seq[T], error) {
	if s.isSet {
		return filterRemoved(s.fetched.Values()), nil
	}
	var zero I
	values, err := s.fn(zero)
	if err != nil {
		return nil, err
	}

	for _, v := range values {
		item, ok := s.fetched.Get(v.ID())
		if !ok {
			s.fetched.Put(v.ID(), Item[T, I]{value: v, status: Unchanged})
		} else if item.status == Added {
			s.fetched.Put(v.ID(), Item[T, I]{value: item.value, status: Modified})
		}
	}

	s.isSet = true
	return filterRemoved(s.fetched.Values()), nil
}

func filterRemoved[T Identifiable[I], I comparable](it iter.Seq[Item[T, I]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range it {
			if v.status == Removed {
				continue
			}
			if !yield(v.value) {
				return
			}
		}
	}
}

var ErrNotFound = errors.New("item not found")

func (s *Slice[T, I]) Get(id I) (T, error) {
	item, exists := s.fetched.Get(id)
	if exists {
		if item.status == Absent || item.status == Removed {
			var zero T
			return zero, ErrNotFound
		}
		return item.value, nil
	}
	if s.isSet {
		var zero T
		return zero, ErrNotFound
	}

	values, err := s.fn(id)
	if err != nil {
		var zero T
		return zero, err
	}
	if len(values) == 0 {
		s.fetched.Put(id, Item[T, I]{status: Absent})
		var zero T
		return zero, ErrNotFound
	}
	s.fetched.Put(values[0].ID(), Item[T, I]{value: values[0], status: Unchanged})
	return values[0], nil
}

func (s *Slice[T, I]) SetAll(value []T) {
	s.isReset = true
	s.isSet = true
	s.fetched = linkedmap.New(linkedmap.WithCapacity[I, Item[T, I]](len(value)))
	for _, v := range value {
		s.fetched.Put(v.ID(), Item[T, I]{value: v, status: Added})
	}
}

func (s *Slice[T, I]) Set(value T) {
	item, exists := s.fetched.Get(value.ID())
	if exists {
		status := item.status
		switch status {
		case Absent, Added:
			status = Added
		case Removed, Unchanged:
			status = Modified
		}
		s.fetched.Put(value.ID(), Item[T, I]{value: value, status: status})
		return
	}
	s.fetched.Put(value.ID(), Item[T, I]{value: value, status: Added})
}

func (s *Slice[T, I]) Clear() {
	s.isSet = true
	s.isReset = true
	s.fetched.Clear()
}

func (s *Slice[T, I]) Remove(id I) bool {
	item, exists := s.fetched.Get(id)
	if exists {
		if item.status == Added {
			s.fetched.Delete(id)
			return true
		}
	}
	s.fetched.Put(id, Item[T, I]{status: Removed})
	return exists
}

func (s *Slice[T, I]) IsReset() bool {
	return s.isReset
}

type SliceChange[I comparable, T any] struct {
	ID     I
	Value  T
	Status Status
}

func (s *Slice[T, I]) Changes() iter.Seq[SliceChange[I, T]] {
	it := s.fetched.Entries()
	return func(yield func(SliceChange[I, T]) bool) {
		for k, v := range it {
			if v.status == Unchanged {
				continue
			}
			change := SliceChange[I, T]{
				ID:     k,
				Value:  v.value,
				Status: v.status,
			}
			if !yield(change) {
				return
			}
		}
	}
}

type EagerSlice[T Identifiable[I], I comparable] struct {
	Slice[T, I]
}

func NewEagerSlice[T Identifiable[I], I comparable](value []T) *EagerSlice[T, I] {
	fetched := linkedmap.New(linkedmap.WithCapacity[I, Item[T, I]](len(value)))
	for _, v := range value {
		fetched.Put(v.ID(), Item[T, I]{value: v, status: Unchanged})
	}
	return &EagerSlice[T, I]{
		Slice: Slice[T, I]{
			isSet:   true,
			fetched: fetched,
		},
	}
}

func (e *EagerSlice[T, I]) GetAll() iter.Seq[T] {
	return filterRemoved(e.fetched.Values())
}

func (e *EagerSlice[T, I]) Get(id I) T {
	item, exists := e.fetched.Get(id)
	if exists {
		return item.value
	}
	var zero T
	return zero
}
