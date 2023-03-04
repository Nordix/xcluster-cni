/*
  SPDX-License-Identifier: Apache-2.0
  Copyright (c) 2019-2023 Nordix Foundation
*/

package util

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/go-logr/logr"
)

// Route Defines a route. The json format is a narroved version of the
// "ip -j" command
type Route struct {
	Dst      string `json:"dst"`
	Protocol string `json:"protocol"`
	Gateway  string `json:"gateway"`
}

type RouteHandler interface {
	// Set Define a route. If the route exists, it will be replaced
	Set(ctx context.Context, route *Route) error
	// Delete Delete a route
	Delete(ctx context.Context, route *Route) error
	// GetRoutes Returns current routes
	GetRoutes(ctx context.Context) ([]Route, error)
}

type ipRoute struct {
	protocol string
	ip       string
}

// Equal Returns true if the routes are equal
func RoutesEqual(r1, r2 *Route) bool {
	if r1 == nil || r2 == nil {
		return (r1 == nil) == (r2 == nil)
	}
	if r1.Protocol != r2.Protocol {
		return false
	}
	// Convert addresses to ensure canonical representation
	_, dst1, err := net.ParseCIDR(r1.Dst)
	if err != nil {
		return false
	}
	_, dst2, err := net.ParseCIDR(r2.Dst)
	if err != nil {
		return false
	}
	// (I am sure there is a faster way)
	if dst1.String() != dst2.String() {
		return false
	}
	gw1 := net.ParseIP(r1.Gateway)
	if gw1 == nil {
		return false
	}
	return gw1.Equal(net.ParseIP(r2.Gateway))
}

// NewRouteHandler Create a RouteHandler for the specified protocol.
// The protocol *may* be a name, but more common a number
func NewRouteHandler(
	ctx context.Context, protocol string) (RouteHandler, error) {
	if protocol == "" {
		return nil, fmt.Errorf("Empty protocol")
	}
	ipPath, err := exec.LookPath("ip")
	if err != nil {
		return nil, err
	}
	r := ipRoute{
		protocol: protocol,
		ip:       ipPath,
	}
	return &r, nil
}

func family(cidr string) string {
	if _, ipNet, err := net.ParseCIDR(cidr); err == nil {
		if ipNet.IP.To4() != nil {
			return "-4"
		}
	}
	return "-6"
}

func (r *ipRoute) Set(ctx context.Context, route *Route) error {
	logger := logr.FromContextOrDiscard(ctx).V(1)
	logger.Info("Set Route", "route", route)
	if route.Gateway == "" {
		return fmt.Errorf("No gateway")
	}
	toctx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()
	cmd := exec.CommandContext(
		toctx, r.ip, family(route.Dst), "-j", "route", "replace",
		"protocol", r.protocol, route.Dst, "via", route.Gateway)
	return cmd.Run()
}

func (r *ipRoute) Delete(ctx context.Context, route *Route) error {
	logger := logr.FromContextOrDiscard(ctx).V(1)
	logger.Info("Delete Route", "route", route)
	toctx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()
	cmd := exec.CommandContext(
		toctx, r.ip, family(route.Dst), "-j", "route", "del",
		"protocol", r.protocol, route.Dst)
	return cmd.Run()
}

func (r *ipRoute) GetRoutes(ctx context.Context) ([]Route, error) {
	toctx, cancel := context.WithTimeout(ctx, time.Second*8)
	defer cancel()

	// Get IPv4 routes
	routes4, err := r.execIp(toctx, "-4")
	if err != nil {
		return nil, err
	}
	// Get IPv6 routes
	routes6, err := r.execIp(toctx, "-6")
	if err != nil {
		return nil, err
	}
	return append(routes4, routes6...), nil
}

func (r *ipRoute) execIp(ctx context.Context, family string) ([]Route, error) {
	cmd := exec.CommandContext(
		ctx, r.ip, family, "-j", "route", "show", "protocol", r.protocol)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	routes := []Route{}
	if err = json.Unmarshal(out, &routes); err != nil {
		return nil, err
	}
	for i := range routes {
		routes[i].Protocol = r.protocol // When requesting a protocol the field is suspressed
	}
	return routes, nil
}
