package delta

import (
	"errors"
	"iter"

	"github.com/quintans/ds/collections/linkedmap"
)

// ============ Scalar ======================

type LazyScalar[T any] struct {
	isSet   bool
	value   T
	fn      func() (T, error)
	isDirty bool
}

func NewLazy[T any](fn func() (T, error)) *LazyScalar[T] {
	return &LazyScalar[T]{isSet: false, fn: fn}
}

func (v *LazyScalar[T]) Get() (T, error) {
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

func (v *LazyScalar[T]) Set(value T) {
	v.value = value
	v.isSet = true
	v.isDirty = true
}

type Change[T any] struct {
	Value T
}

func (v *LazyScalar[T]) Change() *Change[T] {
	if v.isDirty {
		return &Change[T]{Value: v.value}
	}
	return nil
}

type Scalar[T any] struct {
	LazyScalar[T]
}

func New[T any](value T) *Scalar[T] {
	return &Scalar[T]{
		LazyScalar: LazyScalar[T]{
			isSet: true,
			value: value,
		},
	}
}

func (e *Scalar[T]) Get() T {
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

type LazySlice[T Identifiable[I], I comparable] struct {
	isSet   bool
	isReset bool
	fetched *linkedmap.Map[I, Item[T, I]]
	fn      func(I) ([]T, error) // function to load items by ID. If ID is zero value, load all items.
}

func NewLazySlice[T Identifiable[I], I comparable](fn func(I) ([]T, error)) *LazySlice[T, I] {
	return &LazySlice[T, I]{
		isSet:   false,
		fn:      fn,
		fetched: linkedmap.New[I, Item[T, I]](),
	}
}

func (s *LazySlice[T, I]) GetAll() (iter.Seq[T], error) {
	if s.isSet {
		return filterRemoved(s.fetched.Values()), nil
	}
	// load all items when zero value is passed
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

func (s *LazySlice[T, I]) Get(id I) (T, error) {
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

func (s *LazySlice[T, I]) SetAll(value []T) {
	s.isReset = true
	s.isSet = true
	s.fetched = linkedmap.New(linkedmap.WithCapacity[I, Item[T, I]](len(value)))
	for _, v := range value {
		s.fetched.Put(v.ID(), Item[T, I]{value: v, status: Added})
	}
}

func (s *LazySlice[T, I]) Set(value T) {
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

func (s *LazySlice[T, I]) Clear() {
	s.isSet = true
	s.isReset = true
	s.fetched.Clear()
}

func (s *LazySlice[T, I]) Remove(id I) bool {
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

func (s *LazySlice[T, I]) IsReset() bool {
	return s.isReset
}

type Changes[T Identifiable[I], I comparable] struct {
	Reset bool
	Items iter.Seq[SliceChange[I, T]]
}

type SliceChange[I comparable, T any] struct {
	ID     I
	Value  T
	Status Status
}

func (s *LazySlice[T, I]) Changes() Changes[T, I] {
	return Changes[T, I]{
		Reset: s.isReset,
		Items: s.changesIterator(),
	}
}

func (s *LazySlice[T, I]) changesIterator() iter.Seq[SliceChange[I, T]] {
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

type Slice[T Identifiable[I], I comparable] struct {
	LazySlice[T, I]
}

func NewSlice[T Identifiable[I], I comparable](value []T) *Slice[T, I] {
	fetched := linkedmap.New(linkedmap.WithCapacity[I, Item[T, I]](len(value)))
	for _, v := range value {
		fetched.Put(v.ID(), Item[T, I]{value: v, status: Unchanged})
	}
	return &Slice[T, I]{
		LazySlice: LazySlice[T, I]{
			isSet:   true,
			fetched: fetched,
		},
	}
}

func (e *Slice[T, I]) GetAll() iter.Seq[T] {
	return filterRemoved(e.fetched.Values())
}

func (e *Slice[T, I]) Get(id I) T {
	item, exists := e.fetched.Get(id)
	if exists {
		return item.value
	}
	var zero T
	return zero
}
