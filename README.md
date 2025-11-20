# Delta

A Go library that provides lazy loading and change tracking utilities for Domain-Driven Design (DDD) aggregates. It helps optimize performance by deferring expensive data loading operations until needed, while efficiently tracking changes for persistence.

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Features

- **🚀 Lazy Loading**: Defer expensive operations until the data is actually accessed
- **📊 Change Tracking**: Efficiently track what has been modified for optimized persistence
- **🔄 Generic Types**: Fully generic implementation with type safety
- **📝 DDD Support**: Designed specifically for Domain-Driven Design patterns
- **⚡ Performance**: Minimize database queries and memory usage

## Quick Start

```go
import "github.com/quintans/delta"

// Lazy single value
photoLoader := func() ([]byte, error) {
    return loadPhotoFromDatabase()
}
photo := delta.NewLazy(photoLoader)

// Lazy slice with change tracking
// The function receives an ID parameter: if zero value, load all items; otherwise load specific item
carsLoader := func(id uuid.UUID) ([]*Car, error) {
    if id == uuid.Nil {
        return loadAllCarsFromDatabase()
    }
    return loadCarFromDatabase(id)
}
cars := delta.NewLazySlice(carsLoader)

// Access data (loads on first access)
photoData, err := photo.Get()
allCars, err := cars.GetAll()

// Modify and track changes
photo.Set(newPhotoData)
cars.Set(newCar)
cars.Remove(carId)

// Get only what changed
photoChange := photo.Change()
carChanges := cars.Changes()
```

## Core Components

### LazyScalar[T]

Lazy loading container for single values:

```go
// Create with loader function
lazy := delta.NewLazy(func() (string, error) {
    // Expensive operation
    return loadFromAPI(), nil
})

// Create with immediate value
eager := delta.New("immediate value")

// Access (loads once, caches result)
value, err := lazy.Get()

// Modify (marks as dirty)
lazy.Set("new value")

// Check for changes
if change := lazy.Change(); change != nil {
    fmt.Printf("Value changed to: %v", change.Value)
}
```

### LazySlice[T, I]

Lazy loading container for collections with change tracking:

```go
type Car struct {
    id   uuid.UUID
    make string
}

func (c *Car) ID() uuid.UUID { return c.id }

// Create lazy slice with loader function
// The loader receives an ID: if zero value (uuid.Nil), load all; otherwise load specific item
cars := delta.NewLazySlice(func(id uuid.UUID) ([]*Car, error) {
    if id == uuid.Nil {
        return repository.LoadAllCars()
    }
    car, err := repository.LoadCar(id)
    if err != nil {
        return nil, err
    }
    return []*Car{car}, nil
})

// Access all items (loads on first access)
allCars, err := cars.GetAll()

// Access specific item (lazy loads if not already loaded)
car, err := cars.Get(carId)

// Modifications
cars.Set(newCar)        // Add or update
cars.Remove(carId)      // Mark for removal
cars.Clear()           // Clear all
cars.SetAll(newCars)   // Replace all

// Track changes
changes := cars.Changes()
for change := range changes.Items {
    switch change.Status {
    case delta.Added:
        fmt.Printf("Added: %v", change.Value)
    case delta.Modified:
        fmt.Printf("Modified: %v", change.Value)
    case delta.Removed:
        fmt.Printf("Removed ID: %v", change.ID)
    }
}
```

## Usage Patterns

### DDD Aggregate Example

```go
type Person struct {
    id      uuid.UUID
    version int
    name    string
    age     int
    photo   *delta.LazyScalar[[]byte]
    cars    *delta.LazySlice[*Car, uuid.UUID]
}

// Constructor for new entities (eager loading)
func NewPerson(name string, age int, photo []byte) *Person {
    photoScalar := delta.New(photo)
    carsSlice := delta.NewSlice([]*Car{})
    return &Person{
        id:      uuid.New(),
        version: 0, // until is persisted is zero
        name:    name,
        age:     age,
        photo:   &photoScalar.LazyScalar,
        cars:    &carsSlice.LazySlice,
    }
}

// Hydration for persistence (lazy loading)
func HydratePerson(id uuid.UUID, version int, name string, age int, 
    photo *delta.LazyScalar[[]byte], cars *delta.LazySlice[*Car, uuid.UUID]) *Person {
    return &Person{
        id:      id,
        version: version,
        name:    name,
        age:     age,
        photo:   photo,
        cars:    cars,
    }
}

func (p *Person) Photo() ([]byte, error) {
    return p.photo.Get()
}

func (p *Person) BuyCar(car *Car) {
    p.cars.Set(car)
}

func (p *Person) SellCar(carID uuid.UUID) {
    p.cars.Remove(carID)
}

// Get delta for persistence
type PersonDelta struct {
    Photo *delta.Change[[]byte]
    Cars  delta.Changes[*Car, uuid.UUID]
}

func (p *Person) Delta() *PersonDelta {
    return &PersonDelta{
        Photo: p.photo.Change(),
        Cars:  p.cars.Changes(),
    }
}
```

### Repository Pattern

```go
func (r *Repository) GetByID(id uuid.UUID) (*Person, error) {
    record, exists := r.people[id]
    if !exists {
        return nil, fmt.Errorf("person not found")
    }
    
    // Create lazy loaders for expensive data
    photoLazy := delta.NewLazy(func() ([]byte, error) {
        return record.photo, nil
    })
    
    carsLazy := delta.NewLazySlice(func(id uuid.UUID) ([]*Car, error) {
        if id == uuid.Nil {
            return r.loadAllCarsForOwner(id)
        }
        return r.loadCar(id)
    })
    
    return HydratePerson(id, record.version, record.name, record.age, photoLazy, carsLazy), nil
}

func (r *Repository) Update(person *Person) error {
    // Optimistic locking check
    record := r.people[person.ID()]
    if record.version != person.Version() {
        return fmt.Errorf("concurrency conflict")
    }
    record.version++
    
    // Always save certain fields
    record.name = person.Name()
    record.age = person.Age()
    
    // Persist only changes
    delta := person.Delta()
    if delta.Photo != nil {
        record.photo = delta.Photo.Value
    }
    if delta.Cars.Reset {
        r.deleteAllCarsForOwner(person.ID())
    }
    for change := range delta.Cars.Items {
        r.persistCarChange(person.ID(), change)
    }
    
    return nil
}
```

## Best Practices

### ✅ Recommended Patterns

- **Aggregates**: Use lazy fields for expensive operations (photos, large collections)
- **DTOs**: Resolve all lazy fields eagerly for data transfer
- **Repository Create**: Eagerly instantiate all lazy fields with `New` and `NewSlice`
- **Repository Update**: Use delta tracking for efficient persistence
- **Repository Queries**: Return DTOs with resolved data

### ❌ Anti-patterns

- Don't access lazy fields in tight loops without caching
- Don't use lazy loading for small, frequently accessed data
- Don't forget to handle errors from lazy loading operations

## Change Tracking States

| Status | Description |
|--------|-------------|
| `Unchanged` | Item exists and hasn't been modified |
| `Added` | New item added to the collection |
| `Modified` | Existing item has been updated |
| `Removed` | Item marked for deletion |
| `Absent` | Item was requested but not found |

## Installation

```bash
go get github.com/quintans/delta
```

## Requirements

- Go 1.25.1 or higher
- Compatible with modern Go features (generics, iterators)

## Dependencies

- `github.com/google/uuid` - UUID generation and handling
- `github.com/quintans/ds` - Data structures (linkedmap for ordered collections)

## Examples

See the [example](./example) directory for a complete working example demonstrating:
- Person aggregate with lazy photo and cars collection
- Car entity with its own delta tracking (nested aggregates)
- Repository pattern with optimistic locking and delta persistence
- CRUD operations with efficient change tracking
- Lazy loading of expensive data (photos, collections)

Run the example:
```bash
cd example
go run main.go
```