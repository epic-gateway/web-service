package model

import (
	epicv1 "epic-gateway.org/resource-model/api/v1"
)

// Links is a map of URL links from this object to others. Keys are
// strings like "self" or "group" or "service", and values are URL
// strings.
type Links map[string]string

// Account represents an account on the wire.
type Account struct {
	Links   Links          `json:"link"`
	Account epicv1.Account `json:"account"`
}

// NewAccount configures a new Account instance.
func NewAccount() Account {
	return Account{
		Links:   Links{},
		Account: epicv1.Account{},
	}
}

// Group represents a Service Group on the wire.
type Group struct {
	Links Links                 `json:"link"`
	Group epicv1.LBServiceGroup `json:"group"`
}

// NewGroup configures a new Group instance.
func NewGroup() Group {
	return Group{
		Links: Links{},
		Group: epicv1.LBServiceGroup{},
	}
}

// Service represents a load balancer service on the wire.
type Service struct {
	Links   Links               `json:"link"`
	Service epicv1.LoadBalancer `json:"service"`
}

// NewService configures a new Service instance.
func NewService() Service {
	return Service{
		Links:   Links{},
		Service: epicv1.LoadBalancer{},
	}
}

// Proxy represents a load balancer service on the wire.
type Proxy struct {
	Links Links          `json:"link"`
	Proxy epicv1.GWProxy `json:"proxy"`
}

// NewProxy configures a new Proxy instance.
func NewProxy() Proxy {
	return Proxy{
		Links: Links{},
		Proxy: epicv1.GWProxy{},
	}
}

// Slice represents an EndpointSlice on the wire.
type Slice struct {
	Links Links                  `json:"link"`
	Slice epicv1.GWEndpointSlice `json:"slice"`
}

// NewSlice configures a new Slice instance.
func NewSlice() Slice {
	return Slice{
		Links: Links{},
		Slice: epicv1.GWEndpointSlice{},
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
	Links    Links                 `json:"link"`
	Endpoint epicv1.RemoteEndpoint `json:"endpoint"`
}

// NewEndpoint configures a new Endpoint instance.
func NewEndpoint() Endpoint {
	return Endpoint{
		Links:    Links{},
		Endpoint: epicv1.RemoteEndpoint{},
	}
}

// Route represents a GWRoute on the wire.
type Route struct {
	Links Links          `json:"link"`
	Route epicv1.GWRoute `json:"route"`
}

// NewRoute configures a new Route instance.
func NewRoute() Route {
	return Route{
		Links: Links{},
		Route: epicv1.GWRoute{},
	}
}
