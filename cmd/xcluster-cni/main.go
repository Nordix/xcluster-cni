/*
  SPDX-License-Identifier: Apache-2.0
  Copyright (c) 2019-2023 Nordix Foundation
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Nordix/xcluster-cni/pkg/cmd"
	"github.com/Nordix/xcluster-cni/pkg/log"
	"github.com/Nordix/xcluster-cni/pkg/util"
	"github.com/go-logr/logr"
	k8s "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	klog "k8s.io/klog/v2"
)

var version = "unknown"

// Main just register sub-command and hand execution to cmd
func main() {
	cmd.Register("version", cmdVersion)
	cmd.Register("nodes", cmdNodes)
	cmd.Register("pods", cmdPods)
	cmd.Register("watchsvc", cmdWatchSvc)
	cmd.Register("watchpod", cmdWatchPod)
	cmd.Register("watchnodes", cmdWatchNodes)
	cmd.Register("cidr", cmdCIDR)
	cmd.Register("kernelroutes", cmdKernelRoutes)
	cmd.Register("mtu", cmdMTU)
	cmd.Register("k8smtu", cmdK8sMTU)
	cmd.Register("daemon", cmdDaemon)
	os.Exit(cmd.Run(version))
}

func cmdVersion(ctx context.Context, args []string) int {
	fmt.Println(version)
	return 0
}

func cmdNodes(ctx context.Context, args []string) int {
	api := util.GetApi(ctx)
	nodes, err := api.Nodes().List(ctx, meta.ListOptions{})
	if err != nil {
		log.Fatal(ctx, "List Nodes", "error", err)
	}
	// Types in; k8s.io/kubernetes/pkg/apis/core/types.go
	util.EmitJson(nodes.Items)
	return 0
}

func cmdPods(ctx context.Context, args []string) int {
	flagset := flag.NewFlagSet("pods", flag.ExitOnError)
	label := flagset.String("label", "", "Selector label")
	if err := flagset.Parse(args[1:]); err != nil {
		log.Fatal(ctx, "Parse options", "error", err)
	}
	listOptions := meta.ListOptions{}
	if *label != "" {
		listOptions.LabelSelector = *label
	}
	api := util.GetApi(ctx)
	pods, err := api.Pods("default").List(ctx, listOptions)
	if err != nil {
		log.Fatal(ctx, "List Pods", "error", err)
	}
	util.EmitJson(pods.Items)
	return 0
}

func cmdWatchSvc(ctx context.Context, args []string) int {
	api := util.GetApi(ctx)
	watchlist := cache.NewListWatchFromClient(
		api.RESTClient(), "services", k8s.NamespaceDefault,
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&k8s.Service{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				util.EmitJson(obj)
			},
			DeleteFunc: func(obj interface{}) {
				util.EmitJson(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				util.EmitJson(oldObj)
				util.EmitJson(newObj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	<-ctx.Done()
	return 0
}

func cmdWatchPod(ctx context.Context, args []string) int {
	flagset := flag.NewFlagSet("watchpod", flag.ExitOnError)
	label := flagset.String("label", "", "Selector label")
	if err := flagset.Parse(args[1:]); err != nil {
		log.Fatal(ctx, "Parse options", "error", err)
	}
	withlabel := func(options *meta.ListOptions) {
		if *label != "" {
			options.LabelSelector = *label
		}
	}
	api := util.GetApi(ctx)
	watchlist := cache.NewFilteredListWatchFromClient(
		api.RESTClient(), "pods", k8s.NamespaceDefault, withlabel)

	_, controller := cache.NewInformer(
		watchlist,
		&k8s.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				util.EmitJson(obj)
			},
			DeleteFunc: func(obj interface{}) {
				util.EmitJson(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				util.EmitJson(oldObj)
				util.EmitJson(newObj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	<-ctx.Done()
	return 0
}

func cmdWatchNodes(ctx context.Context, args []string) int {
	klog.SetLogger(logr.FromContextOrDiscard(ctx))
	flagset := flag.NewFlagSet("watchnodes", flag.ExitOnError)
	if err := flagset.Parse(args[1:]); err != nil {
		log.Fatal(ctx, "Parse options", "error", err)
	}
	clientset, err := util.GetClientset()
	if err != nil {
		log.Fatal(ctx, "util.GetClientset", "error", err)
	}

	funcs := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			fmt.Println("ADD")
		},
		DeleteFunc: func(obj interface{}) {
			fmt.Println("DELETE")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			fmt.Println("UPDATE")
		},
	}
	toctx, cancel := context.WithTimeout(ctx, time.Second*10)
	h, err := util.CreateNodeHandler(toctx, clientset, &funcs)
	cancel()
	if err != nil {
		log.Fatal(ctx, "util.CreateHandler", "error", err)
	}
	nlist := h.List()
	for i := range nlist {
		np := nlist[i].(*k8s.Node)
		fmt.Println(np)
	}
	//fmt.Println(h.List())

	<-ctx.Done()
	return 0
}

func cmdCIDR(ctx context.Context, args []string) int {
	if len(args) < 3 {
		fmt.Fprintf(os.Stderr, "Syntax: cidr double-slash-cidr number\n")
		return 0
	}
	n, err := strconv.Atoi(args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid number %s\n", args[2])
		return 1
	}
	cidr, err := util.CreateCIDR(args[1], uint(n))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error %v\n", err)
		return 1
	}
	fmt.Println(cidr)
	return 0
}

func cmdKernelRoutes(ctx context.Context, args []string) int {
	h, err := util.NewRouteHandler(ctx, "kernel")
	if err != nil {
		log.Fatal(ctx, "util.NewRouteHandler", "error", err)
	}
	routes, err := h.GetRoutes(ctx)
	if err != nil {
		log.Fatal(ctx, "GetRoutes", "error", err)
	}
	util.EmitJson(routes)
	return 0
}

func cmdMTU(ctx context.Context, args []string) int {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, `
Syntax: mtu <IP>

  Find the interface for the passed IP and print it's MTU.
  If the MTU can't be determined, 1500 is printed.

`)
		return 0
	}

	mtu, err := util.GetMTU(args[1])
	if err != nil {
		logger := logr.FromContextOrDiscard(ctx)
		logger.Error(err, "GetMTU")
	}
	fmt.Println(mtu)
	return 0
}

func cmdK8sMTU(ctx context.Context, args []string) int {
	logger := logr.FromContextOrDiscard(ctx)
	myNode := os.Getenv("NODE_NAME")
	if myNode == "" {
		logger.Info("Env NODE_NAME not set")
		fmt.Println(1500)
		return 1
	}
	nodeReader := util.RealNodeReader()
	n, err := nodeReader.GetNode(ctx, myNode)
	if err != nil {
		logger.Error(err, "GetNode", "name", myNode)
		fmt.Println(1500)
		return 1
	}

	var nodeAddresses []string
	annotation := os.Getenv("ADDRESS_ANNOTATION")
	if annotation != "" {
		// If an annotation is specified, it is prioritized
		if a, ok := n.ObjectMeta.Annotations[annotation]; ok {
			nodeAddresses = strings.Split(a, ",")
		} else {
			logger.Info("Annotation not found", "annotation", annotation)
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
		logger.Info("No node addresses found")
		fmt.Println(1500)
		return 1
	}

	// We assume that addresses of both families are on the same
	// interface, so which address we use doesn't matter
	mtu, err := util.GetMTU(nodeAddresses[0])
	if err != nil {
		logger.Error(err, "GetMTU")
		fmt.Println(mtu)
		return 1
	}

	fmt.Println(mtu)
	return 0
}
