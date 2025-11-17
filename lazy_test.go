package delta_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/quintans/delta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test entity implementing Identifiable
type testEntity struct {
	id   string
	name string
}

func (e *testEntity) ID() string {
	return e.id
}

func TestDelta_Get_Eager(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
	}{
		{"string value", "hello", "hello"},
		{"int value", 42, 42},
		{"slice value", []int{1, 2, 3}, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eager := delta.New(tt.value)
			result := eager.Get()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDelta_Set_Eager(t *testing.T) {
	eager := delta.New(1)
	change := eager.Change()
	assert.Nil(t, change)

	eager.Set(2)
	change = eager.Change()
	require.NotNil(t, change)
	assert.Equal(t, 2, change.Value)
}

func TestDelta_Get_LazySuccess(t *testing.T) {
	callCount := 0
	expectedValue := "loaded value"

	scalar := delta.NewLazy(func() (string, error) {
		callCount++
		return expectedValue, nil
	})

	// First call should load the value
	result, err := scalar.Get()
	require.NoError(t, err)
	assert.Equal(t, expectedValue, result)
	assert.Equal(t, 1, callCount)

	// Second call should return cached value
	result, err = scalar.Get()
	require.NoError(t, err)
	assert.Equal(t, expectedValue, result)
	assert.Equal(t, 1, callCount)

	scalar.Set("new value")
	result, err = scalar.Get()
	require.NoError(t, err)
	assert.Equal(t, "new value", result)
	assert.Equal(t, 1, callCount) // still 1, no additional load
	change := scalar.Change()
	require.NotNil(t, change)
	assert.Equal(t, "new value", change.Value)
}

func TestDelta_Get_LazyError(t *testing.T) {
	expectedError := errors.New("loading failed")
	callCount := 0

	scalar := delta.NewLazy(func() (string, error) {
		callCount++
		return "", expectedError
	})

	result, err := scalar.Get()
	require.ErrorIs(t, err, expectedError)
	assert.Equal(t, "", result)
	assert.Equal(t, 1, callCount)

	// Second call should try to load again (not cached on error)
	result, err = scalar.Get()
	require.ErrorIs(t, err, expectedError)
	assert.Equal(t, "", result)
	assert.Equal(t, 2, callCount)
}

func TestDeltaSlice_GetAll_Eager(t *testing.T) {
	entities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	eager := delta.NewSlice(entities)
	seq := eager.GetAll()
	result := slices.Collect(seq)
	require.Len(t, result, 2)
	assert.Equal(t, "1", result[0].ID())
	assert.Equal(t, "2", result[1].ID())
}

func TestDeltaSlice_Get_Eager(t *testing.T) {
	entities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	eager := delta.NewSlice(entities)
	result := eager.Get("1")
	assert.Equal(t, "1", result.id)
	assert.Equal(t, "entity1", result.name)
}

func TestDeltaSlice_GetAll_LazySuccess(t *testing.T) {
	entities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}
	callCount := 0

	lazySlice := delta.NewLazySlice(func(id string) ([]*testEntity, error) {
		callCount++
		return entities, nil
	})

	// First call should load
	seq, err := lazySlice.GetAll()
	require.NoError(t, err)
	result := slices.Collect(seq)
	require.Len(t, result, 2)
	assert.Equal(t, "1", result[0].ID())
	assert.Equal(t, "2", result[1].ID())
	assert.Equal(t, 1, callCount)

	// Second call should return cached
	seq2, err2 := lazySlice.GetAll()
	require.NoError(t, err2)
	result2 := slices.Collect(seq2)
	require.Len(t, result2, 2)
	assert.Equal(t, "1", result2[0].ID())
	assert.Equal(t, "2", result2[1].ID())
	assert.Equal(t, 1, callCount) // still 1, no additional load
}

func TestDeltaSlice_Get_LazySuccess(t *testing.T) {
	entities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}
	callCount := 0
	var callId string

	lazySlice := delta.NewLazySlice(func(id string) ([]*testEntity, error) {
		callCount++
		callId = id
		return entities, nil
	})

	// First call should load
	result, err := lazySlice.Get("1")
	require.NoError(t, err)
	assert.Equal(t, "entity1", result.name)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, "1", callId)

	callId = ""
	// Second call should return cached
	result2, err2 := lazySlice.Get("1")
	require.NoError(t, err2)
	assert.Equal(t, "entity1", result2.name)
	assert.Equal(t, 1, callCount) // still 1, no additional load
	assert.Equal(t, "", callId)
}

func TestDeltaSlice_GetAll_LazyError(t *testing.T) {
	expectedError := errors.New("loading failed")
	callCount := 0

	lazySlice := delta.NewLazySlice(func(string) ([]*testEntity, error) {
		callCount++
		return nil, expectedError
	})

	seq, err := lazySlice.GetAll()
	require.ErrorIs(t, err, expectedError)
	assert.Nil(t, seq)
	assert.Equal(t, 1, callCount)
}

func TestDeltaSlice_Get_LazyError(t *testing.T) {
	expectedError := errors.New("loading failed")
	callCount := 0

	lazySlice := delta.NewLazySlice(func(string) ([]*testEntity, error) {
		callCount++
		return nil, expectedError
	})

	result, err := lazySlice.Get("1")
	require.ErrorIs(t, err, expectedError)
	assert.Nil(t, result)
	assert.Equal(t, 1, callCount)
}

func TestDeltaSlice_GetAll_WithPendingAdds(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Add entity before loading
	newEntity := &testEntity{id: "2", name: "entity2"}
	lazySlice.Set(newEntity)

	seq, err := lazySlice.GetAll()
	require.NoError(t, err)
	result := slices.Collect(seq)
	require.Len(t, result, 2)

	// Check that both original and added entities are present
	ids := make(map[string]bool)
	for _, entity := range result {
		ids[entity.ID()] = true
	}
	assert.True(t, ids["1"])
	assert.True(t, ids["2"])

	x := lazySlice.Changes()
	assert.False(t, x.Reset)
	changes := slices.Collect(x.Items)
	require.Len(t, changes, 1)
	assert.Equal(t, "2", changes[0].ID)
	assert.Equal(t, delta.Added, changes[0].Status)
}

func TestDeltaSlice_Get_WithPendingAdds(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Add entity before loading
	newEntity := &testEntity{id: "2", name: "entity2"}
	lazySlice.Set(newEntity)

	result, err := lazySlice.Get("1")
	require.NoError(t, err)
	assert.Equal(t, "entity1", result.name)

	// Get the newly added entity
	result2, err2 := lazySlice.Get("2")
	require.NoError(t, err2)
	assert.Equal(t, "entity2", result2.name)
}

func TestDeltaSlice_GetAll_WithPendingRemoves(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Remove entity before loading
	lazySlice.Remove("1")

	seq, err := lazySlice.GetAll()
	require.NoError(t, err)
	result := slices.Collect(seq)
	require.Len(t, result, 1)
	assert.Equal(t, "2", result[0].ID())

	x := lazySlice.Changes()
	assert.False(t, x.Reset)
	changes := slices.Collect(x.Items)
	require.Len(t, changes, 1)
	assert.Equal(t, "1", changes[0].ID)
	assert.Equal(t, delta.Removed, changes[0].Status)
}

func TestDeltaSlice_Get_WithPendingRemoves(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Remove entity before loading
	lazySlice.Remove("1")

	result, err := lazySlice.Get("1")
	require.ErrorIs(t, err, delta.ErrNotFound)
	assert.Nil(t, result)

	result2, err2 := lazySlice.Get("2")
	require.NoError(t, err2)
	assert.Equal(t, "entity2", result2.name)
}

func TestDeltaSlice_GetAll_WithPendingAddsAndRemoves(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Remove one, add one
	lazySlice.Remove("1")
	lazySlice.Set(&testEntity{id: "3", name: "entity3"})

	seq, err := lazySlice.GetAll()
	require.NoError(t, err)
	result := slices.Collect(seq)
	require.Len(t, result, 2)
	assert.Equal(t, "3", result[0].ID())
	assert.Equal(t, "2", result[1].ID())

	// Check correct entities present
	ids := make(map[string]bool)
	for _, entity := range result {
		ids[entity.ID()] = true
	}
	assert.False(t, ids["1"])
	assert.True(t, ids["2"])
	assert.True(t, ids["3"])

	x := lazySlice.Changes()
	assert.False(t, x.Reset)
	changes := slices.Collect(x.Items)
	require.Len(t, changes, 2)
	assert.Equal(t, "1", changes[0].ID)
	assert.Equal(t, delta.Removed, changes[0].Status)
	assert.Equal(t, "3", changes[1].ID)
	assert.Equal(t, delta.Added, changes[1].Status)
}

func TestDeltaSlice_Get_WithPendingAddsAndRemoves(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Remove one, add one
	lazySlice.Remove("1")
	lazySlice.Set(&testEntity{id: "3", name: "entity3"})

	result, err := lazySlice.Get("1")
	require.ErrorIs(t, err, delta.ErrNotFound)
	assert.Nil(t, result)

	result2, err2 := lazySlice.Get("2")
	require.NoError(t, err2)
	assert.Equal(t, "entity2", result2.name)

	result3, err3 := lazySlice.Get("3")
	require.NoError(t, err3)
	assert.Equal(t, "entity3", result3.name)
}

func TestDeltaSlice_GetAll_WithSamePendingAddsAndRemoves(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Add and remove the same entity before loading
	lazySlice.Set(&testEntity{id: "2", name: "entity2"})
	lazySlice.Remove("2")

	seq, err := lazySlice.GetAll()
	require.NoError(t, err)
	result := slices.Collect(seq)
	require.Len(t, result, 1)
	assert.Equal(t, "1", result[0].ID())

	x := lazySlice.Changes()
	assert.False(t, x.Reset)
	changes := slices.Collect(x.Items)
	require.Len(t, changes, 0)
}

func TestDeltaSlice_Get_WithSamePendingAddsAndRemoves(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Add and remove the same entity before loading
	lazySlice.Set(&testEntity{id: "2", name: "entity2"})
	lazySlice.Remove("2")

	result, err := lazySlice.Get("2")
	require.ErrorIs(t, err, delta.ErrNotFound)
	assert.Nil(t, result)

	result2, err2 := lazySlice.Get("1")
	require.NoError(t, err2)
	assert.Equal(t, "entity1", result2.name)
}

func TestDeltaSlice_GetAll_WithSamePendingRemovesAndAdds(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Remove and add the same entity before loading
	lazySlice.Remove("2")
	lazySlice.Set(&testEntity{id: "2", name: "entity2_new"})

	seq, err := lazySlice.GetAll()
	require.NoError(t, err)
	result := slices.Collect(seq)
	require.Len(t, result, 2)

	// Check that entity "2" is the newly added one
	var entity2 *testEntity
	for _, entity := range result {
		if entity.ID() == "2" {
			entity2 = entity
			break
		}
	}
	require.NotNil(t, entity2)
	assert.Equal(t, "entity2_new", entity2.name)

	x := lazySlice.Changes()
	assert.False(t, x.Reset)
	changes := slices.Collect(x.Items)
	require.Len(t, changes, 1)
	assert.Equal(t, "2", changes[0].ID)
	assert.Equal(t, delta.Modified, changes[0].Status)
}

func TestDeltaSlice_Get_WithSamePendingRemovesAndAdds(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	// Remove and add the same entity before loading
	lazySlice.Remove("2")
	lazySlice.Set(&testEntity{id: "2", name: "entity2_new"})

	result, err := lazySlice.Get("2")
	require.NoError(t, err)
	assert.Equal(t, "entity2_new", result.name)
}

func TestDeltaSlice_Set(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	lazySlice.Set(&testEntity{id: "2", name: "entity2_new"})

	result, err := lazySlice.Get("2")
	require.NoError(t, err)
	assert.Equal(t, "entity2_new", result.name)

	x := lazySlice.Changes()
	assert.False(t, x.Reset)
	changes := slices.Collect(x.Items)
	require.Len(t, changes, 1)
	assert.Equal(t, "2", changes[0].ID)
	assert.Equal(t, delta.Added, changes[0].Status)

	seq, err := lazySlice.GetAll()
	require.NoError(t, err)
	resultAll := slices.Collect(seq)
	require.Len(t, resultAll, 2)
	assert.Equal(t, "2", resultAll[0].ID())
	assert.Equal(t, "entity2_new", resultAll[0].name)
	assert.Equal(t, "1", resultAll[1].ID())
	assert.Equal(t, "entity1", resultAll[1].name)

	x = lazySlice.Changes()
	assert.False(t, x.Reset)
	changes = slices.Collect(x.Items)
	require.Len(t, changes, 1)
	assert.Equal(t, "2", changes[0].ID)
	assert.Equal(t, delta.Modified, changes[0].Status)
}

func TestDeltaSlice_SetAll(t *testing.T) {
	baseEntities := []*testEntity{
		{id: "1", name: "entity1"},
		{id: "2", name: "entity2"},
	}

	lazySlice := delta.NewLazySlice(fetcher(baseEntities))

	lazySlice.SetAll(baseEntities)
	assert.True(t, lazySlice.IsReset())

	result, err := lazySlice.Get("1")
	require.NoError(t, err)
	assert.Equal(t, "entity1", result.name)

	x := lazySlice.Changes()
	assert.True(t, x.Reset)
	changes := slices.Collect(x.Items)
	require.Len(t, changes, 2)
	assert.Equal(t, "1", changes[0].Value.ID())
	assert.Equal(t, delta.Added, changes[0].Status)
	assert.Equal(t, "2", changes[1].Value.ID())
	assert.Equal(t, delta.Added, changes[1].Status)
}

func fetcher(ents []*testEntity) func(id string) ([]*testEntity, error) {
	return func(id string) ([]*testEntity, error) {
		if id == "" {
			return ents, nil
		}
		for _, e := range ents {
			if e.id == id {
				return []*testEntity{e}, nil
			}
		}
		return nil, delta.ErrNotFound
	}
}
