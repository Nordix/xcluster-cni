package main

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/Nordix/xcluster-cni/pkg/util"
	"github.com/go-logr/logr"
	k8s "k8s.io/api/core/v1"
)

// syncHandler The class for route-sync. If annotations are empty ("")
// the K8s fields in the Node object are used. The protocol MAY be a
// string if "/etc/iproute2/rt_protos" is updated, but more common
// (and safer) is to use a number.
type syncHandler struct {
	rh                util.RouteHandler
	protocol          string
	cidrAnnotation    string
	addressAnnotation string
}

// syncRoutes Ensure that routes defined by the nodes exists or are
// created. Delete superfluous routes. Only routes that matches the
// "protocol" are handled
func (h *syncHandler) syncRoutes(ctx context.Context, nodes []k8s.Node) error {
	// 1. Create a Route map from the nodes with the canonical Dst as key
	// 2. Create a map of the existing routes. Canonical Dst can be assumed
	// 3. Add all routes that doesn't exist or differs
	// 5. Run through the map of the existing routes and delete superfluous

	myself := getOwnNodeName(ctx, nodes)
	logger := logr.FromContextOrDiscard(ctx)
	want := make(map[string]util.Route, len(nodes))
	for _, n := range nodes {
		if n.ObjectMeta.Name == myself {
			continue
		}
		var nodeAddresses []string
		if h.addressAnnotation != "" {
			// Get the node addresses from the annotation
			if a, ok := n.ObjectMeta.Annotations[h.addressAnnotation]; ok {
				nodeAddresses = strings.Split(a, ",")
			}
		} else {
			// Get the node addresses from ".status.addresses"
			for _, a := range n.Status.Addresses {
				if a.Type == "InternalIP" {
					nodeAddresses = append(nodeAddresses, a.Address)
				}
			}
		}
		if len(nodeAddresses) == 0 {
			logger.Info("No node addresses", "node", n.ObjectMeta.Name)
			continue
		}

		var podCidrs []string
		if h.cidrAnnotation != "" {
			// Get the POD CIDRs from the annotation
			if a, ok := n.ObjectMeta.Annotations[h.cidrAnnotation]; ok {
				podCidrs = strings.Split(a, ",")
			}
		} else {
			// Get the POD CIDRs from .spec.podCIDRs
			podCidrs = n.Spec.PodCIDRs
		}
		if len(podCidrs) == 0 {
			logger.Info("No POD CIDRs", "node", n.ObjectMeta.Name)
			continue
		}
		logger.V(2).Info(
			"Node", "name", n.ObjectMeta.Name, "addresses", nodeAddresses,
			"POD CIDRs", podCidrs)

		// We have collected the node addresses and POD CIDRs for the
		// node.

		handledFamily := 0
		for i, c := range podCidrs {
			if i > 1 {
				logger.Info("POD CIDRs", "len", len(podCidrs))
				break
			}
			dst, family := canonicalCidr(c)
			if family == 0 {
				logger.Info("Parse failed", "CIDR", c)
				continue
			}
			if handledFamily == 0 {
				handledFamily = family
			} else {
				if handledFamily == family {
					logger.Info("Same family", "CIDRs", podCidrs)
					break
				}
			}
			gw := findAdr(family, nodeAddresses)
			if gw == "" {
				logger.Info("No Gateway", "family", family, "CIDR", c)
				continue
			}
			want[dst] = util.Route{
				Dst:      dst,
				Gateway:  gw,
				Protocol: h.protocol,
			}
		}
	}
	if traceLogger := logger.V(2); traceLogger.Enabled() {
		wantedRoutes := make([]util.Route, 0, len(want))
		for _, r := range want {
			wantedRoutes = append(wantedRoutes, r)
		}
		traceLogger.Info("Wanted routes", "routes", wantedRoutes)
	}

	present, err := h.rh.GetRoutes(ctx)
	if err != nil {
		return err
	}
	logger.V(2).Info("Existing routes", "routes", present)
	got := make(map[string]util.Route, len(present))
	for _, r := range present {
		got[r.Dst] = r
	}

	for k, v := range want {
		if c, ok := got[k]; ok {
			if util.RoutesEqual(&v, &c) {
				//logger.V(2).Info("Same route", "want", v, "got", c)
				continue
			}
		}
		h.rh.Set(ctx, &v)
	}
	for k, v := range got {
		if _, ok := want[k]; !ok {
			h.rh.Delete(ctx, &v)
		}
	}
	return nil
}

// canonicalCidr Returns the CIDR in canonical form and the
// family. The family is 4 or 6 or 0 in case of an error
func canonicalCidr(c string) (string, int) {
	_, ipNet, err := net.ParseCIDR(c)
	if err != nil {
		return "", 0
	}
	family := 4
	if ipNet.IP.To4() == nil {
		family = 6
	}
	return ipNet.String(), family
}

// findAdr Returns the first found address that matches the passed family
// The returned address is in canonical form
func findAdr(family int, addresses []string) string {
	for _, a := range addresses {
		ip := net.ParseIP(a)
		if ip != nil {
			if family == 4 && ip.To4() != nil {
				return ip.String()
			}
			if family == 6 && ip.To4() == nil {
				return ip.String()
			}
		}
	}
	return ""
}

// getOwnNodeName Returns the own node name
func getOwnNodeName(ctx context.Context, nodes []k8s.Node) string {
	if nodeName := os.Getenv("NODE_NAME"); nodeName != "" {
		return nodeName
	}
	if n := util.FindOwnNode(ctx, nodes); n != nil {
		return n.ObjectMeta.Name
	}
	return ""
}
