package model

import (
	"github.com/google/uuid"
)

type Links map[string]string

type Group struct {
	ID    uuid.UUID `json:"id,omitempty"`
	Links Links     `json:"link"`
	Name  string    `json:"name"`
}

type Service struct {
	ID      uuid.UUID `json:"id,omitempty"`
	Links   Links     `json:"link"`
	Name    string    `json:"name"`
	Address string    `json:"address"`
	GroupID uuid.UUID `json:"-"`
}

type Endpoint struct {
	ID        uuid.UUID `json:"id,omitempty"`
	Links     Links     `json:"link"`
	Address   string    `json:"address"`
	ServiceID uuid.UUID `json:"-"`
}
