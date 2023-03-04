package main

import (
	"context"
	"os"
	"time"

	"github.com/Nordix/xcluster-cni/pkg/util"
	"github.com/go-logr/logr"
	k8s "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	klog "k8s.io/klog/v2"
)

/*
   cmdDaemon The routing daemon watches the K8s node objects for node
   addresses and POD network CIDRs. Addresses and CIDR can be read
   from annotations or the K8s fields. The routing daemon configures
   routes for POD network CIDRs to node addresses for IPv4 and IPv6.
 */
func cmdDaemon(ctx context.Context, args []string) int {
	logger := logr.FromContextOrDiscard(ctx)
	logger.Info("Xcluster-cni daemon started", "pid", os.Getpid())
	klog.SetLogger(logger) // Use our logger for K8s logging

	// Create a syncer
	protocol := os.Getenv("PROTOCOL")
	if protocol == "" {
		protocol = "202"
	}
	rh, err := util.NewRouteHandler(ctx, protocol)
	if err != nil {
		logger.Error(err, "NewRouteHandler")
		return 1
	}
	sh := syncHandler{
		protocol: protocol,
		cidrAnnotation: os.Getenv("CIDR_ANNOTATION"),
		addressAnnotation: os.Getenv("ADDRESS_ANNOTATION"),
		rh: rh,
	}
	syncer := syncer{
		sh: &sh,
		// The capacity is just one to make sure the channel is
		// drained on each sync. Non-blocking sending is used
		ch: make(chan struct{}, 1),
		lastSync: time.Now(),
	}
	go syncer.run(ctx)			// Start the syncing go function

	// Start watching Nodes
	clientset, err := util.GetClientset()
	if err != nil {
		logger.Error(err, "GetClientset")
		return 1
	}
	funcs := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			syncer.trig(ctx)
		},
		DeleteFunc: func(obj interface{}) {
			syncer.trig(ctx)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			syncer.trig(ctx)
		},
	}
	h, err := util.CreateNodeHandler(ctx, clientset, &funcs)
	if err != nil {
		logger.Error(err, "CreateNodeHandler")
		return 1
	}
	syncer.h = h

	<-ctx.Done()
	logger.Error(ctx.Err(), "Xcluster-cni daemon terminating")
	return 0
}

// syncer The syncer has a trig() function that is called when any K8s
// node update occurs. When trig() is called a signal is sent to a go
// routine that reads all node objects and sets up or update routes.
// Consecutive syncs are at least 'minSyncInterval' apart.
type syncer struct {
	h     util.Handler
	ch    chan struct{}
	sh    *syncHandler
	lastSync time.Time
}
const minSyncInterval = time.Second * 5

// trig Trigs a route sync. Called on any K8s node object update
func (s *syncer) trig(ctx context.Context) {
	logger := logr.FromContextOrDiscard(ctx).V(2)
	// The capacity is just one to make sure the channel is
	// drained on each sync. Non-blocking sending is used
	//logger.Info("Called trig", "cap", cap(s.ch), "len", len(s.ch))
	select {
	case s.ch <- struct{}{}:
		logger.Info("Sync trigged")
	default:
		logger.Info("Sync already trigged")
	}
}

// run A go routine to run the sync
func (s *syncer) run(ctx context.Context) {
	for {
		select {
		case <-s.ch:
			// Since the capacity of the channel is 1 (one) it is
			// drained by this read
			s.sync(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// sync 
func (s *syncer) sync(ctx context.Context) {
	logger := logr.FromContextOrDiscard(ctx)
	// Delay if necessary to ensure syncs are more than
	// 'minSyncInterval' apart.  We can't use time.Sleep() since it
	// can't be interrupted.
	interval := time.Since(s.lastSync)
	if interval < minSyncInterval {
		d := minSyncInterval - interval
		if d < time.Second {
			d = time.Second
		}
		logger.V(1).Info("Delaying sync", "duration", d)
		t := time.NewTimer(d)
		select {
		case <-t.C:
		case <-ctx.Done():
			if !t.Stop() {
				<-t.C
			}
			return // interrupted
		}
	}

	start := time.Now()
	logger.Info("Syncing routes start")
	nodeList := s.h.List()
	nodes := make([]k8s.Node, len(nodeList))
	for i := range nodeList {
		np := nodeList[i].(*k8s.Node)
		nodes[i] = *np
	}
	err := s.sh.syncRoutes(ctx, nodes)
	if err != nil {
		logger.Error(err, "Sync routes")
	}
	s.lastSync = time.Now()
	logger.Info("Syncing routes finish", "duration", s.lastSync.Sub(start))
}
