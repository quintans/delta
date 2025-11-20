package repository

import (
	"fmt"

	"github.com/quintans/delta"
	"github.com/quintans/delta/example/domain"

	"github.com/google/uuid"
)

type PersonRecord struct {
	version int
	name    string
	age     int
	photo   []byte
}

type CarRecord struct {
	make    string
	kms     int
	ownerID uuid.UUID
}

type Repository struct {
	people map[uuid.UUID]*PersonRecord
	cars   map[uuid.UUID]*CarRecord
}

func NewRepository() *Repository {
	return &Repository{
		people: make(map[uuid.UUID]*PersonRecord),
		cars:   make(map[uuid.UUID]*CarRecord),
	}
}
func (r *Repository) GetByID(id uuid.UUID) (*domain.Person, error) {
	record, exists := r.people[id]
	if !exists {
		return nil, fmt.Errorf("person not found")
	}
	photoLazy := delta.NewLazy(func() ([]byte, error) {
		fmt.Println("*** Lazy-loading photo")
		return record.photo, nil
	})
	carLazy := delta.NewLazySlice(func(id uuid.UUID) ([]*domain.Car, error) {
		// if id is uuid.Nil, load all cars for the owner
		if id == uuid.Nil {
			fmt.Println("*** Lazy-loading cars")
			var cars []*domain.Car
			for carID, carRecord := range r.cars {
				if carRecord.ownerID == id {
					car := domain.HydrateCar(carID, carRecord.make, carRecord.kms)
					cars = append(cars, car)
				}
			}
			return cars, nil
		}

		carRecord, exists := r.cars[id]
		if !exists {
			return []*domain.Car{}, nil
		}
		car := domain.HydrateCar(id, carRecord.make, carRecord.kms)
		return []*domain.Car{car}, nil
	})
	person := domain.HydratePerson(id, record.version, record.name, record.age, photoLazy, carLazy)
	return person, nil
}

// Create creates a new person and its cars.
func (r *Repository) Create(p *domain.Person) error {
	if _, exists := r.people[p.ID()]; exists {
		return fmt.Errorf("person already exists")
	}

	photo, err := p.Photo()
	if err != nil {
		return fmt.Errorf("failed to get photo: %w", err)
	}
	r.people[p.ID()] = &PersonRecord{
		version: 1,
		name:    p.Name(),
		age:     p.Age(),
		photo:   photo,
	}

	cars, err := p.Cars()
	if err != nil {
		return fmt.Errorf("failed to get cars: %w", err)
	}
	for _, car := range cars {
		if err := r.saveCar(p.ID(), car, true); err != nil {
			return fmt.Errorf("failed to save car: %w", err)
		}
	}

	return nil
}

// Update updates a person and its cars. It uses optimistic locking to prevent concurrent updates.
//
// This should be the only way to update a persisted person.
func (r *Repository) Update(p *domain.Person) error {
	// optimistic locking check
	record, exists := r.people[p.ID()]
	if !exists && record.version != p.Version() {
		return fmt.Errorf("concurrency conflict")
	}
	record.version++

	// some fields are always saved regardless of delta
	record.name = p.Name()
	record.age = p.Age()

	changes := p.Delta()
	if changes != nil {
		// only save fields that have changed
		if changes.Photo != nil {
			record.photo = changes.Photo.Value
			fmt.Println("*** photo changed to:", string(record.photo))
		}
		if changes.Cars.Reset {
			fmt.Println("*** cars reset")
			// remove all existing cars
			for carID, carRecord := range r.cars {
				if carRecord.ownerID == p.ID() {
					delete(r.cars, carID)
				}
			}
		}
		for item := range changes.Cars.Items {
			car := item.Value
			switch item.Status {
			case delta.Removed:
				fmt.Println("*** car removed:", car.ID())
				delete(r.cars, car.ID())
			case delta.Added, delta.Modified:
				fmt.Printf("*** car added/modified: %s, added?: %t\n", car.ID(), item.Status == delta.Added)
				if err := r.saveCar(p.ID(), car, item.Status == delta.Added); err != nil {
					return fmt.Errorf("failed to save car: %w", err)
				}
			}
		}

	}
	return nil
}

func (r *Repository) Delete(id uuid.UUID) error {
	if _, exists := r.people[id]; !exists {
		return fmt.Errorf("person not found")
	}
	delete(r.people, id)

	// also delete all cars owned by this person
	for carID, carRecord := range r.cars {
		if carRecord.ownerID == id {
			delete(r.cars, carID)
		}
	}

	return nil
}

func (r *Repository) saveCar(ownerID uuid.UUID, car *domain.Car, isNew bool) error {
	if isNew {
		// new car
		r.cars[car.ID()] = &CarRecord{make: car.Make(), kms: car.Kms(), ownerID: ownerID}
	} else {
		record, exists := r.cars[car.ID()]
		if !exists {
			return fmt.Errorf("car not found")
		}

		// some fields are always saved regardless of delta
		record.make = car.Make()
		delta := car.Delta()
		if delta != nil {
			// only save fields that have changed
			if delta.Kms != nil {
				record.kms = delta.Kms.Value
			}
		}
	}
	return nil
}
