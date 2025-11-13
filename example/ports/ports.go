package ports

import (
	"github.com/quintans/delta/example/domain"

	"github.com/google/uuid"
)

type Repository interface {
	GetByID(id string) (*PersonDTO, error)
	Create(p *domain.Person) error
	Update(p *domain.Person) error
}

// PersonDTO is a data transfer object representing a person and should have any lazy fields resolved.
type PersonDTO struct {
	ID    uuid.UUID
	Name  string
	Age   int
	Photo []byte
	Cars  []*DTO
}

type DTO struct {
	ID   uuid.UUID
	Make string
	Kms  int
}
