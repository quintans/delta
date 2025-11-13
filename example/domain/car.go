package domain

import (
	"github.com/google/uuid"
	lazy "github.com/quintans/delta"
)

// Car belongs to the person aggregate and therefore does not have its own repository nor versioning.
type Car struct {
	id   uuid.UUID
	make string
	kms  *lazy.Eager[int]
}

func NewCar(make string, kms int) *Car {
	return &Car{
		id:   uuid.New(),
		make: make,
		kms:  lazy.NewEager(kms),
	}
}

func HydrateCar(id uuid.UUID, make string, kms int) *Car {
	return &Car{
		id:   id,
		make: make,
		kms:  lazy.NewEager(kms),
	}
}

func (c *Car) ID() uuid.UUID {
	return c.id
}

func (c *Car) Make() string {
	return c.make
}

func (c *Car) drive(kms int) {
	currentKms := c.kms.Get()
	c.kms.Set(currentKms + kms)
}

func (c *Car) Kms() int {
	return c.kms.Get()
}

type CarDelta struct {
	Kms *lazy.Change[int]
}

func (c *Car) Delta() *CarDelta {
	return &CarDelta{
		Kms: c.kms.Change(),
	}
}
