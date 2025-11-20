package main

import (
	"fmt"

	"github.com/quintans/delta/example/domain"
	"github.com/quintans/delta/example/repository"
)

func main() {
	fmt.Println("Hello, Lazy Aggregate with Go!")

	repository := repository.NewRepository()
	// Create a new person
	person := domain.NewPerson("John Doe", 30, []byte("Photo data"))
	err := repository.Create(person)
	if err != nil {
		panic(err)
	}

	person, err = repository.GetByID(person.ID())
	if err != nil {
		panic(err)
	}

	car := domain.NewCar("bmw", 10000)
	person.BuyCar(car)

	err = repository.Update(person)
	if err != nil {
		panic(err)
	}

	// Retrieve the person
	retrievedPerson, err := repository.GetByID(person.ID())
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"Retrieved Person: ID=%s, Name=%s, Age=%d\n",
		retrievedPerson.ID(), retrievedPerson.Name(), retrievedPerson.Age(),
	)

	photo, err := retrievedPerson.Photo()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Retrieved Person photo: %s\n", photo)

	cars, err := retrievedPerson.Cars()
	if err != nil {
		panic(err)
	}
	fmt.Println("Retrieved Person Cars:")
	for _, car := range cars {
		fmt.Printf(" - ID=%s, Make=%s, Kms=%d\n", car.ID(), car.Make(), car.Kms())
	}

	// Update the person
	fmt.Println("Updating person and buying and driving a car...")
	person, err = repository.GetByID(person.ID())
	person.SetPhoto([]byte("New photo data"))
	car = domain.NewCar("Toyota", 2000)
	person.BuyCar(car)
	err = person.DriveCar(car.ID(), 30)
	if err != nil {
		panic(fmt.Errorf("failed to drive car: %w", err))
	}
	repository.Update(person)

	// Delete the person
	err = repository.Delete(person.ID())
	if err != nil {
		panic(err)
	}
	fmt.Println("Person deleted successfully")
}
