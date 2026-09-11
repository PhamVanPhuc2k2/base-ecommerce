package domain

import (
	"time"

	"github.com/google/uuid"
)

type Brand struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
