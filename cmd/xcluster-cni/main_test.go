package main

import (
	"context"
	"os"
	"testing"

	"github.com/Nordix/xcluster-cni/pkg/log"
	"github.com/Nordix/xcluster-cni/pkg/util"
	"github.com/go-logr/logr"
	k8s "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
	   Testing is simple in principle: define the node array and use a
	   fake route-handler. However to define input and expected output is
	   ... extensive.

	   Trace can be enabled per test-case. Run;

		  go test | grep '^{' | jq

	   Addresses in the route-handler ("before" and "after") MUST be in
	   canonical format. Addresses and CIDRs in annotations and K8s fields
	   MAY not.
*/
func TestRouteSync(t *testing.T) {
	const (
		cidrAnnotation    = "cidr.nordix.org/eth2"
		addressAnnotation = "addr.nordix.org/eth2"
		ownNode           = "myself"
	)

	annotationHandler := syncHandler{
		cidrAnnotation:    cidrAnnotation,
		addressAnnotation: addressAnnotation,
	}

	tcases := []struct {
		name        string
		syncHandler *syncHandler
		nodes       []k8s.Node
		before      []util.Route
		after       []util.Route
		trace       bool
	}{
		{
			name:        "Get nothing, do nothing",
			syncHandler: &syncHandler{},
		},
		{
			name:        "No nodes, delete all routes",
			syncHandler: &annotationHandler,
			before: []util.Route{
				{
					Dst:     "10.0.0.0/24",
					Gateway: "11.0.0.1",
				},
				{
					Dst:     "fd00::/112",
					Gateway: "fd00:1000::1",
				},
			},
		},
		{
			name:        "One peer node",
			syncHandler: &annotationHandler,
			nodes: []k8s.Node{
				{
					ObjectMeta: meta.ObjectMeta{
						Name: "peer",
						Annotations: map[string]string{
							cidrAnnotation:    "20.0.0.0/24,fd00:1000::0.0.0.0/96",
							addressAnnotation: "192.168.1.1,fd00:1::192.168.1.1",
						},
					},
				},
			},
			before: []util.Route{
				{
					Dst:     "10.0.0.0/24",
					Gateway: "11.0.0.1",
				},
				{
					Dst:     "fd00::/112",
					Gateway: "fd00:1000::1",
				},
			},
			after: []util.Route{
				{
					Dst:     "20.0.0.0/24",
					Gateway: "192.168.1.1",
				},
				{
					Dst:     "fd00:1000::/96",
					Gateway: "fd00:1::c0a8:101",
				},
			},
		},
		{
			name:        "IPv6 encoded IPv4 addresses",
			syncHandler: &annotationHandler,
			nodes: []k8s.Node{
				{
					ObjectMeta: meta.ObjectMeta{
						Name: "peer",
						Annotations: map[string]string{
							cidrAnnotation:    "::ffff:20.0.0.0/120,fd00:1000::0.0.0.0/96",
							addressAnnotation: "::ffff:192.168.1.1,fd00:1::192.168.1.1",
						},
					},
				},
			},
			after: []util.Route{
				{
					Dst:     "20.0.0.0/24",
					Gateway: "192.168.1.1",
				},
				{
					Dst:     "fd00:1000::/96",
					Gateway: "fd00:1::c0a8:101",
				},
			},
		},
		{
			name:        "Only the own node",
			syncHandler: &annotationHandler,
			nodes: []k8s.Node{
				{
					ObjectMeta: meta.ObjectMeta{
						Name: ownNode,
						Annotations: map[string]string{
							cidrAnnotation:    "20.0.0.0/24,fd00:1000::0.0.0.0/96",
							addressAnnotation: "192.168.1.1,fd00:1::192.168.1.1",
						},
					},
				},
			},
			before: []util.Route{
				{
					Dst:     "10.0.0.0/24",
					Gateway: "11.0.0.1",
				},
				{
					Dst:     "fd00::/112",
					Gateway: "fd00:1000::1",
				},
			},
		},
		{
			name:        "K8s Node",
			syncHandler: &syncHandler{},
			nodes: []k8s.Node{
				{
					ObjectMeta: meta.ObjectMeta{
						Name: "peer",
					},
					Status: k8s.NodeStatus{
						Addresses: []k8s.NodeAddress{
							{
								Type:    k8s.NodeInternalIP,
								Address: "192.168.1.1",
							},
							{
								Type:    k8s.NodeInternalIP,
								Address: "fd00:1::192.168.1.1",
							},
						},
					},
					Spec: k8s.NodeSpec{
						PodCIDRs: []string{
							"20.0.0.0/24",
							"fd00:1000::0.0.0.0/96",
						},
					},
				},
			},
			before: []util.Route{
				{
					Dst:     "20.0.0.0/24",
					Gateway: "11.0.0.1",
				},
				{
					Dst:     "fd00:1000::/96",
					Gateway: "fd00:1000::1",
				},
			},
			after: []util.Route{
				{
					Dst:     "20.0.0.0/24",
					Gateway: "192.168.1.1",
				},
				{
					Dst:     "fd00:1000::/96",
					Gateway: "fd00:1::c0a8:101",
				},
			},
		},
		{
			name: "Annotations has precedence in K8s Nodes",
			syncHandler: &syncHandler{
				addressAnnotation: addressAnnotation,
			},
			nodes: []k8s.Node{
				{
					ObjectMeta: meta.ObjectMeta{
						Name: "peer",
						Annotations: map[string]string{
							addressAnnotation: "172.20.1.1,fd00:1::172.20.1.1",
						},
					},
					Status: k8s.NodeStatus{
						Addresses: []k8s.NodeAddress{
							{
								Type:    k8s.NodeInternalIP,
								Address: "192.168.1.1",
							},
							{
								Type:    k8s.NodeInternalIP,
								Address: "fd00:1::192.168.1.1",
							},
						},
					},
					Spec: k8s.NodeSpec{
						PodCIDRs: []string{
							"20.0.0.0/24",
							"fd00:1000::0.0.0.0/96",
						},
					},
				},
			},
			before: []util.Route{
				{
					Dst:     "20.0.0.0/24",
					Gateway: "172.20.1.1",
				},
				{
					Dst:     "fd00:1000::/96",
					Gateway: "fd00:1::ac14:101",
				},
			},
			after: []util.Route{
				{
					Dst:     "20.0.0.0/24",
					Gateway: "172.20.1.1",
				},
				{
					Dst:     "fd00:1000::/96",
					Gateway: "fd00:1::ac14:101",
				},
			},
		},
	}

	_ = os.Setenv("NODE_NAME", ownNode)
	for _, tc := range tcases {
		rh := newTestRouteHandler(t, tc.before)
		h := tc.syncHandler
		h.rh = rh
		ctx := context.TODO()
		if tc.trace {
			z, _ := log.ZapLogger("stderr", "trace")
			ctx = log.NewContext(ctx, z)
		}
		if err := tc.syncHandler.syncRoutes(ctx, tc.nodes); err != nil {
			t.Errorf("%s: Unexpected error %v", tc.name, err)
		} else {
			if !rh.sameRoutes(tc.after) {
				routes, _ := h.rh.GetRoutes(ctx)
				t.Errorf("%s: Invalid routes after sync: %v", tc.name, routes)
			}
		}
	}
}

type testRouteHandler struct {
	routes map[string]util.Route
	t      *testing.T
}

func newTestRouteHandler(t *testing.T, routes []util.Route) *testRouteHandler {
	rh := testRouteHandler{
		routes: make(map[string]util.Route, len(routes)),
		t:      t,
	}
	for _, r := range routes {
		//t.Logf("Init route %s", r.Dst)
		rh.routes[r.Dst] = r
	}
	return &rh
}

func (t *testRouteHandler) Set(
	ctx context.Context, route *util.Route) error {
	logger := logr.FromContextOrDiscard(ctx).V(2)
	logger.Info("Route Set", "route", route)
	t.routes[route.Dst] = *route
	return nil
}

func (t *testRouteHandler) Delete(
	ctx context.Context, route *util.Route) error {
	logger := logr.FromContextOrDiscard(ctx).V(2)
	logger.Info("Route Delete", "route", route)
	delete(t.routes, route.Dst)
	return nil
}

func (t *testRouteHandler) GetRoutes(
	ctx context.Context) ([]util.Route, error) {
	routes := make([]util.Route, len(t.routes))
	i := 0
	for _, v := range t.routes {
		routes[i] = v
		i++
	}
	return routes, nil
}

func (t *testRouteHandler) sameRoutes(routes []util.Route) bool {
	if len(routes) != len(t.routes) {
		return false
	}
	for _, r1 := range routes {
		if r2, ok := t.routes[r1.Dst]; ok {
			if !util.RoutesEqual(&r1, &r2) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
