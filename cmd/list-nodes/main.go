package main

import (
	"encoding/json"
	"flag"
	"fmt"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
)

var version string = "unknown"

func main() {
	ver := flag.Bool("version", false, "Print version and quit")
	flag.Parse()

	if *ver {
		fmt.Println(version)
		os.Exit(0)
	}

	dumpNodes()
	os.Exit(0)
}

func getClientset() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig :=
			clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}

func dumpNodes() error {
	clientset, err := getClientset()
	if err != nil {
		return err
	}

	api := clientset.CoreV1()
	nodes, err := api.Nodes().List(meta.ListOptions{})
	if err != nil {
		return err
	}

	// Types in; k8s.io/kubernetes/pkg/apis/core/types.go
	for _, n := range nodes.Items {
		if s, err := json.Marshal(n); err == nil {
			fmt.Println(string(s))
		}
	}

	return nil
}
