// Copyright 2017 Google Inc.
// Copyright 2020 Acnodal Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package allocator

import (
	"fmt"
	"net"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
	corev1 "k8s.io/api/core/v1"
)

// An Allocator tracks IP address pools and allocates addresses from them.
type Allocator struct {
	pools     map[string]Pool
	allocated map[string]*alloc // svc -> alloc
}

type alloc struct {
	pool  string
	ip    net.IP
	ports []corev1.ServicePort
	Key
}

// NewAllocator returns an Allocator managing no pools.
func NewAllocator() *Allocator {
	return &Allocator{
		pools:     map[string]Pool{},
		allocated: map[string]*alloc{},
	}
}

// SetPools updates the set of address pools that the allocator owns.
func (a *Allocator) SetPools(groups []egwv1.ServicePrefix) error {
	pools, err := parseConfig(groups)
	if err != nil {
		return err
	}

	for n := range a.pools {
		if pools[n] == nil {
			poolCapacity.DeleteLabelValues(n)
			poolActive.DeleteLabelValues(n)
		}
	}

	a.pools = pools

	// Refresh or initiate stats
	for n, p := range a.pools {
		poolCapacity.WithLabelValues(n).Set(float64(p.Size()))
		poolActive.WithLabelValues(n).Set(float64(p.InUse()))
	}

	return nil
}

// assign unconditionally updates internal state to reflect svc's
// allocation of alloc. Caller must ensure that this call is safe.
func (a *Allocator) assign(svc string, alloc *alloc) {
	a.Unassign(svc)
	a.allocated[svc] = alloc

	pool := a.pools[alloc.pool]
	pool.Assign(alloc.ip, alloc.ports, svc, &alloc.Key)

	poolCapacity.WithLabelValues(alloc.pool).Set(float64(a.pools[alloc.pool].Size()))
	poolActive.WithLabelValues(alloc.pool).Set(float64(pool.InUse()))
}

// Assign assigns the requested ip to svc, if the assignment is
// permissible by sharingKey.
func (a *Allocator) Assign(svc string, ip net.IP, ports []corev1.ServicePort, sharingKey string) (string, error) {
	pool := poolFor(a.pools, ip)
	if pool == "" {
		return "", fmt.Errorf("%q is not allowed in config", ip)
	}
	sk := &Key{
		Sharing: sharingKey,
	}

	// Does the IP already have allocs? If so, needs to be the same
	// sharing key, and have non-overlapping ports. If not, the proposed
	// IP needs to be allowed by configuration.
	err := a.pools[pool].Available(ip, ports, svc, sk) // FIXME: this should Assign() here, not check Available.  Might need to iterate over pools rather than do poolFor
	if err != nil {
		return "", err
	}

	// Either the IP is entirely unused, or the requested use is
	// compatible with existing uses. Assign!
	alloc := &alloc{
		pool:  pool,
		ip:    ip,
		ports: make([]corev1.ServicePort, len(ports)),
		Key:   *sk,
	}
	for i, port := range ports {
		alloc.ports[i] = port
	}
	a.assign(svc, alloc)
	return pool, nil
}

// Unassign frees the IP associated with service, if any.
func (a *Allocator) Unassign(svc string) bool {
	if a.allocated[svc] == nil {
		return false
	}

	al := a.allocated[svc]

	// tell the pool that the address has been released. there might not
	// be a pool, e.g., in the case of a config change that move
	// addresses from one pool to another
	pool, tracked := a.pools[al.pool]
	if tracked {
		pool.Release(al.ip, svc)
		poolActive.WithLabelValues(al.pool).Set(float64(pool.InUse()))
	}

	delete(a.allocated, svc)

	return true
}

// AllocateFromPool assigns an available IP from pool to service.
func (a *Allocator) AllocateFromPool(svc string, poolName string, ports []corev1.ServicePort, sharingKey string) (net.IP, error) {
	var ip net.IP

	// if we have already allocated an address for this service then
	// return it
	if alloc := a.allocated[svc]; alloc != nil {
		return alloc.ip, nil
	}

	pool := a.pools[poolName]
	if pool == nil {
		fmt.Printf("known pools: %#v\n", a.pools)
		return nil, fmt.Errorf("unknown pool %q", poolName)
	}

	sk := &Key{
		Sharing: sharingKey,
	}
	ip, err := pool.AssignNext(svc, ports, sk)
	if err != nil {
		// Woops, no IPs :( Fail.
		return nil, err
	}

	alloc := &alloc{
		pool:  poolName,
		ip:    ip,
		ports: make([]corev1.ServicePort, len(ports)),
		Key:   *sk,
	}
	for i, port := range ports {
		alloc.ports[i] = port
	}
	a.assign(svc, alloc)

	return ip, nil
}

// Allocate assigns any available and assignable IP to service.
func (a *Allocator) Allocate(svc string, ports []corev1.ServicePort, sharingKey string) (string, net.IP, error) {
	var (
		err error
		ip  net.IP
	)

	fmt.Println("attempting to allocate")

	// it's either the "default" pool or nothing
	ip, err = a.AllocateFromPool(svc, "default", ports, sharingKey)
	if err == nil {
		return "default", ip, nil
	}
	return "", nil, err
}

// IP returns the IP address allocated to service, or nil if none are allocated.
func (a *Allocator) IP(svc string) net.IP {
	if alloc := a.allocated[svc]; alloc != nil {
		return alloc.ip
	}
	return nil
}

// Pool returns the pool from which service's IP was allocated. If
// service has no IP allocated, "" is returned.
func (a *Allocator) Pool(svc string) string {
	ip := a.IP(svc)
	if ip == nil {
		return ""
	}
	return poolFor(a.pools, ip)
}

// poolFor returns the pool that owns the requested IP, or "" if none.
func poolFor(pools map[string]Pool, ip net.IP) string {
	for pname, p := range pools {
		if p.Contains(ip) {
			return pname
		}
	}
	return ""
}

func parseConfig(groups []egwv1.ServicePrefix) (map[string]Pool, error) {
	pools := map[string]Pool{}

	for i, group := range groups {
		poolName := group.Name

		pool, err := parsePrefix(poolName, group.Spec)
		if err != nil {
			return nil, fmt.Errorf("parsing address pool #%d: %s", i+1, err)
		}

		// Check that the pool isn't already defined
		if pools[poolName] != nil {
			return nil, fmt.Errorf("duplicate definition of pool %q", poolName)
		}

		// Check that this pool doesn't overlap with any of the previous
		// ones
		for name, r := range pools {
			if pool.Overlaps(r) {
				return nil, fmt.Errorf("pool %q overlaps with already defined pool %q", poolName, name)
			}
		}

		pools[poolName] = pool
	}

	return pools, nil
}

func parsePrefix(name string, prefix egwv1.ServicePrefixSpec) (Pool, error) {
	ret, err := NewLocalPool(prefix.Pool, prefix.Subnet, prefix.Aggregation)
	if err != nil {
		return nil, err
	}
	return *ret, nil
}
