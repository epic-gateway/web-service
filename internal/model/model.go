package model

import (
	"database/sql"

	"github.com/google/uuid"
)

type Links map[string]string

type Group struct {
	Links   Links  `json:"link"`
	Name    string `json:"name"`
	Created *Time  `json:"created"`
	Updated *Time  `json:"updated"`
}

func NewGroup() Group {
	return Group{
		// initialize times so we can Scan() from the db
		Created: NewTime(),
		Updated: NewTime(),
	}
}

type Service struct {
	Links   Links     `json:"link"`
	Name    string    `json:"name"`
	Address string    `json:"address"`
	GroupID uuid.UUID `json:"-"`
	Created *Time     `json:"created"`
	Updated *Time     `json:"updated"`
}

func NewService() Service {
	return Service{
		// initialize times so we can Scan() from the db
		Created: NewTime(),
		Updated: NewTime(),
	}
}

type Endpoint struct {
	Links     Links     `json:"link"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	ServiceID uuid.UUID `json:"-"`
	Created   *Time     `json:"created"`
	Updated   *Time     `json:"updated"`
}

func NewEndpoint() Endpoint {
	return Endpoint{
		// initialize times so we can Scan() from the db
		Created: NewTime(),
		Updated: NewTime(),
	}
}

type Callbacks interface {
	ServiceChanged(service Service, endpoints []Endpoint)
}

type Time struct {
	sql.NullTime
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t.Valid {
		return t.Time.MarshalJSON()
	}
	return []byte("null"), nil
}

func NewTime() *Time {
	return &Time{NullTime: sql.NullTime{}}
}
