package model

import (
	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
)

// Links is a map of URL links from this object to others. Keys are
// strings like "self" or "group" or "service", and values are URL
// strings.
type Links map[string]string

// Account represents an account on the wire.
type Account struct {
	Links   Links         `json:"link"`
	Account egwv1.Account `json:"account"`
}

// NewAccount configures a new Account instance.
func NewAccount() Account {
	return Account{
		Links:   Links{},
		Account: egwv1.Account{},
	}
}

// Group represents a Service Group on the wire.
type Group struct {
	Links Links              `json:"link"`
	Group egwv1.ServiceGroup `json:"group"`
}

// NewGroup configures a new Group instance.
func NewGroup() Group {
	return Group{
		Links: Links{},
		Group: egwv1.ServiceGroup{},
	}
}

// Service represents a load balancer service on the wire.
type Service struct {
	Links   Links              `json:"link"`
	Service egwv1.LoadBalancer `json:"service"`
}

// NewService configures a new Service instance.
func NewService() Service {
	return Service{
		Links:   Links{},
		Service: egwv1.LoadBalancer{},
	}
}

// Cluster represents an LB upstream cluster on the wire.
type Cluster struct {
	Links Links `json:"link"`
}

// NewCluster configures a new Cluster instance.
func NewCluster() Cluster {
	return Cluster{
		Links: Links{},
	}
}

// Endpoint represents a load balancer endpoint on the wire.
type Endpoint struct {
	Links    Links                `json:"link"`
	Endpoint egwv1.RemoteEndpoint `json:"endpoint"`
}

// NewEndpoint configures a new Endpoint instance.
func NewEndpoint() Endpoint {
	return Endpoint{
		Links:    Links{},
		Endpoint: egwv1.RemoteEndpoint{},
	}
}
