package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/Nordix/xcluster-cni/pkg/util"
	"github.com/go-logr/logr"
	k8s "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	klog "k8s.io/klog/v2"
)

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
	syncer := syncer{sh: &sh}

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

type syncer struct {
	h     util.Handler
	timer *time.Timer
	mu    sync.Mutex
	sh    *syncHandler
}

// trig Trigs a route sync. The sync is delayed 5s to avoid
// sync-storms, for instance on start or re-start
func (s *syncer) trig(ctx context.Context) {
	s.mu.Lock()
	if s.timer == nil {
		s.timer = time.NewTimer(time.Second * 5)
		go s.run(ctx)
	}
	s.mu.Unlock()
}

// run A go routine to run the sync
func (s *syncer) run(ctx context.Context) {
	select {
	case <-s.timer.C:
		s.sync(ctx)
		s.mu.Lock()
		s.timer = nil
		s.mu.Unlock()
	case <-ctx.Done():
		s.mu.Lock()
		if !s.timer.Stop() {
			<-s.timer.C
		}
		s.timer = nil
		s.mu.Unlock()
	}
}

// sync 
func (s *syncer) sync(ctx context.Context) {
	start := time.Now()
	logger := logr.FromContextOrDiscard(ctx)
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
	logger.Info("Syncing routes finish", "duration", time.Now().Sub(start))
}
