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
	"net"
	"testing"

	ptu "github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	egwv1 "gitlab.com/acnodal/egw-resource-model/api/v1"
)

var (
	tcp23 = corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 23}
)

func TestAssignment(t *testing.T) {
	alloc := NewAllocator()
	alloc.pools = map[string]Pool{
		"test0": mustLocalPool(t, "1.2.3.4/31"),
		"test1": mustLocalPool(t, "1000::4/127"),
		"test2": mustLocalPool(t, "1.2.4.0/24"),
		"test3": mustLocalPool(t, "1000::4:0/120"),
	}

	tests := []struct {
		desc       string
		svc        string
		ip         string
		ports      []corev1.ServicePort
		sharingKey string
		wantErr    bool
	}{
		{
			desc: "assign s1",
			svc:  "s1",
			ip:   "1.2.3.4",
		},
		{
			desc: "s1 idempotent reassign",
			svc:  "s1",
			ip:   "1.2.3.4",
		},
		{
			desc:    "s2 can't grab s1's IP",
			svc:     "s2",
			ip:      "1.2.3.4",
			wantErr: true,
		},
		{
			desc: "s2 can get the other IP",
			svc:  "s2",
			ip:   "1.2.3.5",
		},
		{
			desc:    "s1 now can't grab s2's IP",
			svc:     "s1",
			ip:      "1.2.3.5",
			wantErr: true,
		},
		{
			desc: "s1 frees its IP",
			svc:  "s1",
			ip:   "",
		},
		{
			desc: "s2 can grab s1's former IP",
			svc:  "s2",
			ip:   "1.2.3.4",
		},
		{
			desc: "s1 can now grab s2's former IP",
			svc:  "s1",
			ip:   "1.2.3.5",
		},
		{
			desc: "s3 can grab another IP in that pool",
			svc:  "s3",
			ip:   "1.2.4.254",
		},
		{
			desc:       "s4 takes an IP, with sharing",
			svc:        "s4",
			ip:         "1.2.4.3",
			ports:      []corev1.ServicePort{http},
			sharingKey: "sharing",
		},
		{
			desc:       "s4 changes its sharing key in place",
			svc:        "s4",
			ip:         "1.2.4.3",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
		},
		{
			desc:       "s3 can't share with s4 (port conflict)",
			svc:        "s3",
			ip:         "1.2.4.3",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
			wantErr:    true,
		},
		{
			desc:       "s3 can't share with s4 (wrong sharing key)",
			svc:        "s3",
			ip:         "1.2.4.3",
			ports:      []corev1.ServicePort{https},
			sharingKey: "othershare",
			wantErr:    true,
		},
		{
			desc:       "s3 takes the same IP as s4",
			svc:        "s3",
			ip:         "1.2.4.3",
			ports:      []corev1.ServicePort{https},
			sharingKey: "share",
		},
		{
			desc:       "s3 can change its ports while keeping the same IP",
			svc:        "s3",
			ip:         "1.2.4.3",
			ports:      []corev1.ServicePort{dns},
			sharingKey: "share",
		},
		{
			desc: "s4 takes s3's former IP",
			svc:  "s4",
			ip:   "1.2.4.254",
		},

		// IPv6 tests (same as ipv4 but with ipv6 addresses)
		{
			desc: "ipv6 assign s1",
			svc:  "s1",
			ip:   "1000::4",
		},
		{
			desc: "s1 idempotent reassign",
			svc:  "s1",
			ip:   "1000::4",
		},
		{
			desc:    "s2 can't grab s1's IP",
			svc:     "s2",
			ip:      "1000::4",
			wantErr: true,
		},
		{
			desc: "s2 can get the other IP",
			svc:  "s2",
			ip:   "1000::4:5",
		},
		{
			desc:    "s1 now can't grab s2's IP",
			svc:     "s1",
			ip:      "1000::4:5",
			wantErr: true,
		},
		{
			desc: "s1 frees its IP",
			svc:  "s1",
			ip:   "",
		},
		{
			desc: "s2 can grab s1's former IP",
			svc:  "s2",
			ip:   "1000::4",
		},
		{
			desc: "s1 can now grab s2's former IP",
			svc:  "s1",
			ip:   "1000::4:5",
		},
		{
			desc: "s3 can grab another IP in that pool",
			svc:  "s3",
			ip:   "1000::4:ff",
		},
		{
			desc:       "s4 takes an IP, with sharing",
			svc:        "s4",
			ip:         "1000::4:3",
			ports:      []corev1.ServicePort{http},
			sharingKey: "sharing",
		},
		{
			desc:       "s4 changes its sharing key in place",
			svc:        "s4",
			ip:         "1000::4:3",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
		},
		{
			desc:       "s3 can't share with s4 (port conflict)",
			svc:        "s3",
			ip:         "1000::4:3",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
			wantErr:    true,
		},
		{
			desc:       "s3 can't share with s4 (wrong sharing key)",
			svc:        "s3",
			ip:         "1000::4:3",
			ports:      []corev1.ServicePort{https},
			sharingKey: "othershare",
			wantErr:    true,
		},
		{
			desc:       "s3 takes the same IP as s4",
			svc:        "s3",
			ip:         "1000::4:3",
			ports:      []corev1.ServicePort{https},
			sharingKey: "share",
		},
		{
			desc:       "s3 can change its ports while keeping the same IP",
			svc:        "s3",
			ip:         "1000::4:3",
			ports:      []corev1.ServicePort{dns},
			sharingKey: "share",
		},
		{
			desc:       "s3 can't change its sharing key while keeping the same IP",
			svc:        "s3",
			ip:         "1000::4:3",
			ports:      []corev1.ServicePort{https},
			sharingKey: "othershare",
			wantErr:    true,
		},
		{
			desc: "s4 takes s3's former IP",
			svc:  "s4",
			ip:   "1000::4:ff",
		},
	}

	for _, test := range tests {
		if test.ip == "" {
			alloc.Unassign(test.svc)
			continue
		}
		ip := net.ParseIP(test.ip)
		if ip == nil {
			t.Fatalf("invalid IP %q in test %q", test.ip, test.desc)
		}
		alreadyHasIP := assigned(alloc, test.svc) == test.ip
		_, err := alloc.Assign(test.svc, ip, test.ports, test.sharingKey)
		if test.wantErr {
			if err == nil {
				t.Errorf("%q should have caused an error, but did not", test.desc)
			} else if a := assigned(alloc, test.svc); !alreadyHasIP && a == test.ip {
				t.Errorf("%q: Assign(%q, %q) failed, but allocator did record allocation", test.desc, test.svc, test.ip)
			}

			continue
		}

		if err != nil {
			t.Errorf("%q: Assign(%q, %q): %s", test.desc, test.svc, test.ip, err)
		}
		if a := assigned(alloc, test.svc); a != test.ip {
			t.Errorf("%q: ran Assign(%q, %q), but allocator has recorded allocation of %q", test.desc, test.svc, test.ip, a)
		}
	}
}

func TestPoolAllocation(t *testing.T) {
	alloc := NewAllocator()
	// This test only allocates from the "test" and "testV6" pools, so
	// it will run out of IPs quickly even though there are tons
	// available in other pools.
	alloc.pools = map[string]Pool{
		"not_this_one": mustLocalPool(t, "192.168.0.0/16"),
		"test":         mustLocalPool(t, "1.2.3.4/30"),
		"testV6":       mustLocalPool(t, "1000::/126"),
		"test2":        mustLocalPool(t, "10.20.30.0/24"),
	}

	validIP4s := map[string]bool{
		"1.2.3.4": true,
		"1.2.3.5": true,
		"1.2.3.6": true,
		"1.2.3.7": true,
	}
	validIP6s := map[string]bool{
		"1000::":  true,
		"1000::1": true,
		"1000::2": true,
		"1000::3": true,
	}

	tests := []struct {
		desc       string
		svc        string
		ports      []corev1.ServicePort
		sharingKey string
		unassign   bool
		wantErr    bool
		isIPv6     bool
	}{
		{
			desc: "s1 gets an IP",
			svc:  "s1",
		},
		{
			desc: "s2 gets an IP",
			svc:  "s2",
		},
		{
			desc: "s3 gets an IP",
			svc:  "s3",
		},
		{
			desc: "s4 gets an IP",
			svc:  "s4",
		},
		{
			desc:    "s5 can't get an IP",
			svc:     "s5",
			wantErr: true,
		},
		{
			desc:    "s6 can't get an IP",
			svc:     "s6",
			wantErr: true,
		},
		{
			desc:     "s1 releases its IP",
			svc:      "s1",
			unassign: true,
		},
		{
			desc: "s5 can now grab s1's former IP",
			svc:  "s5",
		},
		{
			desc:    "s6 still can't get an IP",
			svc:     "s6",
			wantErr: true,
		},
		{
			desc:     "s5 unassigns in prep for enabling IP sharing",
			svc:      "s5",
			unassign: true,
		},
		{
			desc:       "s5 enables IP sharing",
			svc:        "s5",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
		},
		{
			desc:       "s6 can get an IP now, with sharing",
			svc:        "s6",
			ports:      []corev1.ServicePort{https},
			sharingKey: "share",
		},

		// Clear old ipv4 addresses
		{
			desc:     "s1 clear old ipv4 address",
			svc:      "s1",
			unassign: true,
		},
		{
			desc:     "s2 clear old ipv4 address",
			svc:      "s2",
			unassign: true,
		},
		{
			desc:     "s3 clear old ipv4 address",
			svc:      "s3",
			unassign: true,
		},
		{
			desc:     "s4 clear old ipv4 address",
			svc:      "s4",
			unassign: true,
		},
		{
			desc:     "s5 clear old ipv4 address",
			svc:      "s5",
			unassign: true,
		},
		{
			desc:     "s6 clear old ipv4 address",
			svc:      "s6",
			unassign: true,
		},

		// IPv6 tests.
		{
			desc:   "s1 gets an IP6",
			svc:    "s1",
			isIPv6: true,
		},
		{
			desc:   "s2 gets an IP6",
			svc:    "s2",
			isIPv6: true,
		},
		{
			desc:   "s3 gets an IP6",
			svc:    "s3",
			isIPv6: true,
		},
		{
			desc:   "s4 gets an IP6",
			svc:    "s4",
			isIPv6: true,
		},
		{
			desc:    "s5 can't get an IP6",
			svc:     "s5",
			isIPv6:  true,
			wantErr: true,
		},
		{
			desc:    "s6 can't get an IP6",
			svc:     "s6",
			isIPv6:  true,
			wantErr: true,
		},
		{
			desc:     "s1 releases its IP6",
			svc:      "s1",
			unassign: true,
		},
		{
			desc:   "s5 can now grab s1's former IP6",
			svc:    "s5",
			isIPv6: true,
		},
		{
			desc:    "s6 still can't get an IP6",
			svc:     "s6",
			isIPv6:  true,
			wantErr: true,
		},
		{
			desc:     "s5 unassigns in prep for enabling IP6 sharing",
			svc:      "s5",
			unassign: true,
		},
		{
			desc:       "s5 enables IP6 sharing",
			svc:        "s5",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
			isIPv6:     true,
		},
		{
			desc:       "s6 can get an IP6 now, with sharing",
			svc:        "s6",
			ports:      []corev1.ServicePort{https},
			sharingKey: "share",
			isIPv6:     true,
		},
	}

	for _, test := range tests {
		if test.unassign {
			alloc.Unassign(test.svc)
			continue
		}
		pool := "test"
		if test.isIPv6 {
			pool = "testV6"
		}
		ip, err := alloc.AllocateFromPool(test.svc, pool, test.ports, test.sharingKey)
		if test.wantErr {
			if err == nil {
				t.Errorf("%s: should have caused an error, but did not", test.desc)

			}
			continue
		}
		if err != nil {
			t.Errorf("%s: AllocateFromPool(%q, \"test\"): %s", test.desc, test.svc, err)
		}
		validIPs := validIP4s
		if test.isIPv6 {
			validIPs = validIP6s
		}
		if !validIPs[ip.String()] {
			t.Errorf("%s: allocated unexpected IP %q", test.desc, ip)
		}
	}

	alloc.Unassign("s5")
	if _, err := alloc.AllocateFromPool("s5", "nonexistentpool", nil, ""); err == nil {
		t.Error("Allocating from non-existent pool succeeded")
	}
}

func TestAllocation(t *testing.T) {
	alloc := NewAllocator()
	alloc.pools = map[string]Pool{
		"default": mustLocalPool(t, "1.2.3.4/30"),
		"test1V6": mustLocalPool(t, "1000::4/127"),
	}

	validIPs := map[string]bool{
		"1.2.3.4": true,
		"1.2.3.5": true,
		"1.2.3.6": true,
		"1.2.3.7": true,
		"1000::4": true,
		"1000::5": true,
	}

	tests := []struct {
		desc       string
		svc        string
		ports      []corev1.ServicePort
		sharingKey string
		unassign   bool
		wantErr    bool
	}{
		{
			desc: "s1 gets an IP",
			svc:  "s1",
		},
		{
			desc: "s2 gets an IP",
			svc:  "s2",
		},
		{
			desc: "s3 gets an IP",
			svc:  "s3",
		},
		{
			desc: "s4 gets an IP",
			svc:  "s4",
		},
		{
			desc:    "s5 can't get an IP",
			svc:     "s5",
			wantErr: true,
		},
		{
			desc:    "s6 can't get an IP",
			svc:     "s6",
			wantErr: true,
		},
		{
			desc:     "s1 gives up its IP",
			svc:      "s1",
			unassign: true,
		},
		{
			desc:       "s5 can now get an IP",
			svc:        "s5",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
		},
		{
			desc:    "s6 still can't get an IP",
			svc:     "s6",
			wantErr: true,
		},
		{
			desc:       "s6 can get an IP with sharing",
			svc:        "s6",
			ports:      []corev1.ServicePort{https},
			sharingKey: "share",
		},

		// Clear addresses
		{
			desc:     "s1 clear old ipv4 address",
			svc:      "s1",
			unassign: true,
		},
		{
			desc:     "s2 clear old ipv4 address",
			svc:      "s2",
			unassign: true,
		},
		{
			desc:     "s3 clear old ipv4 address",
			svc:      "s3",
			unassign: true,
		},
		{
			desc:     "s4 clear old ipv4 address",
			svc:      "s4",
			unassign: true,
		},
		{
			desc:     "s5 clear old ipv4 address",
			svc:      "s5",
			unassign: true,
		},
		{
			desc:     "s6 clear old ipv4 address",
			svc:      "s6",
			unassign: true,
		},

		{
			desc: "s1 gets an IP",
			svc:  "s1",
		},
		{
			desc: "s2 gets an IP",
			svc:  "s2",
		},
		{
			desc: "s3 gets an IP",
			svc:  "s3",
		},
		{
			desc: "s4 gets an IP",
			svc:  "s4",
		},
		{
			desc:    "s5 can't get an IP",
			svc:     "s5",
			wantErr: true,
		},
		{
			desc:    "s6 can't get an IP",
			svc:     "s6",
			wantErr: true,
		},
		{
			desc:     "s1 gives up its IP",
			svc:      "s1",
			unassign: true,
		},
		{
			desc:       "s5 can now get an IP",
			svc:        "s5",
			ports:      []corev1.ServicePort{http},
			sharingKey: "share",
		},
		{
			desc:    "s6 still can't get an IP",
			svc:     "s6",
			wantErr: true,
		},
		{
			desc:       "s6 can get an IP with sharing",
			svc:        "s6",
			ports:      []corev1.ServicePort{https},
			sharingKey: "share",
		},
	}

	for _, test := range tests {
		if test.unassign {
			alloc.Unassign(test.svc)
			continue
		}
		_, ip, err := alloc.Allocate(test.svc, test.ports, test.sharingKey)
		if test.wantErr {
			if err == nil {
				t.Errorf("%s: should have caused an error, but did not", test.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: Allocate(%q, \"test\"): %s", test.desc, test.svc, err)
		}
		if !validIPs[ip.String()] {
			t.Errorf("%s allocated unexpected IP %q", test.desc, ip)
		}
	}
}

func TestPoolMetrics(t *testing.T) {
	alloc := NewAllocator()
	alloc.pools = map[string]Pool{
		"test": mustLocalPool(t, "1.2.3.4/30"),
	}

	tests := []struct {
		desc       string
		svc        string
		ip         string
		ports      []corev1.ServicePort
		sharingKey string
		ipsInUse   float64
	}{
		{
			desc:     "assign s1",
			svc:      "s1",
			ip:       "1.2.3.4",
			ipsInUse: 1,
		},
		{
			desc:     "assign s2",
			svc:      "s2",
			ip:       "1.2.3.5",
			ipsInUse: 2,
		},
		{
			desc:     "unassign s1",
			svc:      "s1",
			ipsInUse: 1,
		},
		{
			desc:     "unassign s2",
			svc:      "s2",
			ipsInUse: 0,
		},
		{
			desc:       "assign s1 shared",
			svc:        "s1",
			ip:         "1.2.3.4",
			sharingKey: "key",
			ports:      []corev1.ServicePort{http},
			ipsInUse:   1,
		},
		{
			desc:       "assign s2 shared",
			svc:        "s2",
			ip:         "1.2.3.4",
			sharingKey: "key",
			ports:      []corev1.ServicePort{https},
			ipsInUse:   1,
		},
		{
			desc:       "assign s3 shared",
			svc:        "s3",
			ip:         "1.2.3.4",
			sharingKey: "key",
			ports:      []corev1.ServicePort{tcp23},
			ipsInUse:   1,
		},
		{
			desc:     "unassign s1 shared",
			svc:      "s1",
			ports:    []corev1.ServicePort{http},
			ipsInUse: 1,
		},
		{
			desc:     "unassign s2 shared",
			svc:      "s2",
			ports:    []corev1.ServicePort{https},
			ipsInUse: 1,
		},
		{
			desc:     "unassign s3 shared",
			svc:      "s3",
			ports:    []corev1.ServicePort{tcp23},
			ipsInUse: 0,
		},
	}

	// The "test" pool contains one range: 1.2.3.4/30
	value := ptu.ToFloat64(poolCapacity.WithLabelValues("test"))
	if int(value) != 4 {
		t.Errorf("stats.poolCapacity invalid %f. Expected 4", value)
	}

	for _, test := range tests {
		if test.ip == "" {
			alloc.Unassign(test.svc)
			value := ptu.ToFloat64(poolActive.WithLabelValues("test"))
			if value != test.ipsInUse {
				t.Errorf("%v; in-use %v. Expected %v", test.desc, value, test.ipsInUse)
			}
			continue
		}

		ip := net.ParseIP(test.ip)
		if ip == nil {
			t.Fatalf("invalid IP %q in test %q", test.ip, test.desc)
		}
		_, err := alloc.Assign(test.svc, ip, test.ports, test.sharingKey)

		if err != nil {
			t.Errorf("%q: Assign(%q, %q): %v", test.desc, test.svc, test.ip, err)
		}
		if a := assigned(alloc, test.svc); a != test.ip {
			t.Errorf("%q: ran Assign(%q, %q), but allocator has recorded allocation of %q", test.desc, test.svc, test.ip, a)
		}
		value := ptu.ToFloat64(poolActive.WithLabelValues("test"))
		if value != test.ipsInUse {
			t.Errorf("%v; in-use %v. Expected %v", test.desc, value, test.ipsInUse)
		}
	}
}

// Some helpers

func assigned(a *Allocator, svc string) string {
	ip := a.IP(svc)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func mustLocalPool(t *testing.T, r string) LocalPool {
	p, err := NewLocalPool(r, "", "")
	if err != nil {
		panic(err)
	}
	return *p
}

func servicePrefix(name string, spec egwv1.ServicePrefixSpec) *egwv1.ServicePrefix {
	return &egwv1.ServicePrefix{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}
}
