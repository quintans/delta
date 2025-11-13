package domain

import (
	"fmt"
	"iter"
	"slices"

	lazy "github.com/quintans/delta"

	"github.com/google/uuid"
)

type Person struct {
	id    uuid.UUID
	name  string
	age   int
	photo *lazy.Lazy[[]byte]           // lazy-loaded photo
	cars  *lazy.Slice[*Car, uuid.UUID] // lazy-loaded cars
}

func NewPerson(name string, age int, photo []byte) *Person {
	photoLazy := lazy.NewEager(photo)
	carsLazy := lazy.NewEagerSlice[*Car, uuid.UUID]([]*Car{})
	return &Person{
		id:    uuid.New(),
		name:  name,
		age:   age,
		photo: &photoLazy.Lazy,
		cars:  &carsLazy.Slice,
	}
}

func HydratePerson(id uuid.UUID, name string, age int, photo *lazy.Lazy[[]byte], cars *lazy.Slice[*Car, uuid.UUID]) *Person {
	return &Person{
		id:    id,
		name:  name,
		age:   age,
		photo: photo,
		cars:  cars,
	}
}

func (p *Person) ID() uuid.UUID {
	return p.id
}

func (p *Person) Name() string {
	return p.name
}

func (p *Person) Age() int {
	return p.age
}

func (p *Person) Photo() ([]byte, error) {
	return p.photo.Get()
}

func (p *Person) SetPhoto(photo []byte) {
	p.photo.Set(photo)
}

func (p *Person) HappyBirthday() {
	p.age++
}

func (p *Person) Cars() ([]*Car, error) {
	it, err := p.cars.GetAll() // load cars if not already loaded
	if err != nil {
		return nil, err
	}
	return slices.Collect(it), nil
}

func (p *Person) BuyCar(car *Car) {
	p.cars.Set(car)
}

func (p *Person) SellCar(carID uuid.UUID) {
	p.cars.Remove(carID)
}

func (p *Person) DriveCar(carID uuid.UUID, kms int) error {
	cars, err := p.cars.GetAll() // ensure cars are loaded
	if err != nil {
		return err
	}
	for car := range cars {
		if car.ID() == carID {
			car.drive(kms)
			p.cars.Set(car)
			return nil
		}
	}
	return fmt.Errorf("car with ID %s not found", carID)
}

func (p *Person) Greet() string {
	return fmt.Sprintf("Hello, my name is %s and I am %d years old.", p.name, p.age)
}

type PersonDelta struct {
	Photo *lazy.Change[[]byte]
	Cars  iter.Seq[lazy.SliceChange[uuid.UUID, *Car]]
}

func (p *Person) Delta() *PersonDelta {
	return &PersonDelta{
		Photo: p.photo.Change(),
		Cars:  p.cars.Changes(),
	}
}
