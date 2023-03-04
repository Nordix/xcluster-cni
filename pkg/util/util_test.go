package util

import (
	"bytes"
	"testing"
	//"net"
)

func TestCreateCIDR(t *testing.T) {
	tcases := []struct {
		name        string
		dcidr       string
		nodeNo      uint
		expected    string
		expectedErr bool
	}{
		{
			name:     "Basic IPv6",
			dcidr:    "fd00:1000::/96/112",
			nodeNo:   5,
			expected: "fd00:1000::5:0/112",
		},
		{
			name:     "Basic IPv4",
			dcidr:    "192.168.0.0/22/25",
			nodeNo:   5,
			expected: "192.168.2.128/25",
		},
		{
			name:     "Basic IPv6/64",
			dcidr:    "fd00:1000::/48/64",
			nodeNo:   5,
			expected: "fd00:1000:0:5::/64",
		},
		{
			name:        "Invalid IPv6 adress",
			dcidr:       "fd00::1000::/96/112",
			expectedErr: true,
		},
		{
			name:        "Bits1 invalid",
			dcidr:       "fd00:1000::/0x3/112",
			expectedErr: true,
		},
		{
			name:        "Bits2 invalid",
			dcidr:       "fd00:1000::/96/0x64",
			expectedErr: true,
		},
		{
			name:        "Bits1 > bits1",
			dcidr:       "fd00:1000::/96/64",
			expectedErr: true,
		},
		{
			name:        "Bits1 too high IPv4",
			dcidr:       "10.0.0.0/34/36",
			expectedErr: true,
		},
		{
			name:        "Bits1 too low",
			dcidr:       "fd00:1000::/0/112",
			expectedErr: true,
		},
	}
	for _, tc := range tcases {
		cidr, err := CreateCIDR(tc.dcidr, tc.nodeNo)
		if tc.expectedErr {
			if err == nil {
				t.Errorf("%s: expected err, got %s", tc.name, cidr)
			}
			t.Logf("%s: %v", tc.name, err)
		} else {
			if err != nil {
				t.Errorf("%s: unexpected err %v", tc.name, err)
			} else {
				t.Logf("%s: %s", tc.name, cidr)
				if cidr != tc.expected {
					t.Errorf(
						"%s: expected %s, got %s", tc.name, tc.expected, cidr)
				}
			}
		}
	}
}

func TestShiftOr(t *testing.T) {
	tcases := []struct {
		name     string
		b        []byte
		n        uint64
		shift    int
		expected []byte
	}{
		{
			name:     "One byte",
			b:        []byte{0},
			n:        0xffff,
			shift:    4,
			expected: []byte{0xf0},
		},
		{
			name:     "All 32-bit set",
			b:        []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			n:        0xffffffff,
			shift:    64,
			expected: []byte{0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:     "Non-aligned 7-bit value",
			b:        []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			n:        0x7f,
			shift:    14,
			expected: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x1f, 0xc0, 0},
		},
	}
	for _, tc := range tcases {
		shiftOr(tc.b, tc.n, tc.shift)
		t.Logf("%s: %v", tc.name, tc.b)
		if !bytes.Equal(tc.b, tc.expected) {
			t.Errorf("%s: Expected %v, got %v", tc.name, tc.expected, tc.b)
		}
	}
}

func TestRouteEqual(t *testing.T) {
	tcases := []struct {
		name  string
		r1    *Route
		r2    *Route
		equal bool
	}{
		{
			name:  "Both nil",
			equal: true,
		},
		{
			name:  "One nil",
			r1:    &Route{},
			equal: false,
		},
		{
			name:  "The other nil",
			r2:    &Route{},
			equal: false,
		},
		{
			name: "IPv6 encoded IPv4",
			r1: &Route{
				Dst:     "10.0.0.0/24",
				Gateway: "::ffff:192.168.1.1",
			},
			r2: &Route{
				Dst:     "::ffff:10.0.0.1/120",
				Gateway: "192.168.1.1",
			},
			equal: true,
		},
		{
			name: "Non-canonical IPv6",
			r1: &Route{
				Dst:     "fd00:0:0:0:0:0:a00:0000/120",
				Gateway: "fd00:0:0:0:0:0:a00:1",
			},
			r2: &Route{
				Dst:     "fd00::10.0.0.0/120",
				Gateway: "fd00::10.0.0.1",
			},
			equal: true,
		},
	}
	for _, tc := range tcases {
		if RoutesEqual(tc.r1, tc.r2) != tc.equal {
			t.Errorf("%s: failed", tc.name)
		}
	}
}
