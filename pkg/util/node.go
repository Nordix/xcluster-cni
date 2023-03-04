package util

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	k8s "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// https://github.com/feiskyer/kubernetes-handbook/blob/master/examples/client/informer/informer.go

type Handler interface {
	List() []interface{}
}

type handler struct {
	logger     logr.Logger
	controller cache.Controller
	store      cache.Store
	stop       chan struct{}
}

func CreateNodeHandler(
	ctx context.Context, client *kubernetes.Clientset,
	funcs *cache.ResourceEventHandlerFuncs) (Handler, error) {
	h := handler{
		logger: logr.FromContextOrDiscard(ctx).WithName("NodeHandler"),
		stop:   make(chan struct{}),
	}
	h.logger.V(2).Info("Start")

	watchlist := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(), "nodes", "", fields.Everything())
	h.store, h.controller = cache.NewInformer(
		watchlist, &k8s.Node{}, time.Hour, funcs)
	go h.controller.Run(h.stop)

	timer := time.NewTimer(time.Second)
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
	}()
	for {
		if h.controller.HasSynced() {
			break
		}
		select {
		case <-timer.C:
			timer.Reset(time.Second)
		case <-ctx.Done():
			return nil, fmt.Errorf("CreateNodeHandler interrupted")
		}
	}
	return &h, nil
}

func (h handler) List() []interface{} {
	return h.store.List()
}

// Find own node.  The own node is found by comparing
// status.nodeInfo.machineID with the "/etc/machine-id" file. The node
// name may differ from the hostname and several nodes may have the
// same hostname so this is the (only?) safe way.
func FindOwnNode(ctx context.Context, nodes []k8s.Node) *k8s.Node {
	logger := logr.FromContextOrDiscard(ctx)
	// First try the machine-id
	file, err := os.Open("/etc/machine-id")
	if err != nil {
		logger.Error(err, "os.Open", "file", "/etc/machine-id")
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var machineId string
	for scanner.Scan() {
		machineId = scanner.Text()
		if machineId == "" {
			// Find first non-empty line (may be only white-space though...)
			continue
		}
		logger.V(2).Info("Read machine-id", "machine-id", machineId)
		if machineId != "" {
			for _, n := range nodes {
				if n.Status.NodeInfo.MachineID == machineId {
					logger.V(2).Info(
						"Found own node", "name", n.ObjectMeta.Name)
					return &n
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Error(err, "Scanning machine-id")
	}
	return nil
}

// FindNode Returns the named node or nil
func FindNode(ctx context.Context, nodes []k8s.Node, name string) *k8s.Node {
	for _, n := range nodes {
		if n.ObjectMeta.Name == name {
			return &n
		}
	}
	return nil
}

// NodeReader Interface to simplify unit-test
type NodeReader interface {
	GetNodes(ctx context.Context) ([]k8s.Node, error)
	GetNode(ctx context.Context, name string) (*k8s.Node, error)
}
type realNodeReader struct{}

func RealNodeReader() NodeReader {
	return &realNodeReader{}
}

// GetNodes Returns all node objects
func (o *realNodeReader) GetNodes(ctx context.Context) ([]k8s.Node, error) {
	logger := logr.FromContextOrDiscard(ctx)
	api := GetApi(ctx)

	nodes, err := api.Nodes().List(ctx, meta.ListOptions{})
	if err != nil {
		return nil, err
	}
	logger.V(2).Info("Read nodes", "count", len(nodes.Items))
	return nodes.Items, nil
}

// GetNode Reads and returns a node object. Only one object is read, making
// this more efficient than call GetNodes() and FindNode()
func (o *realNodeReader) GetNode(ctx context.Context, name string) (*k8s.Node, error) {
	if name != "" {
		return nil, fmt.Errorf("No name")
	}

	// Read just the selected node
	api := GetApi(ctx)
	nodes, err := api.Nodes().List(ctx, meta.ListOptions{
		FieldSelector: "meta.name=" + name,
	})
	if err != nil {
		return nil, err
	}
	if len(nodes.Items) == 0 {
		return nil, fmt.Errorf("Node not found")
	}
	return &nodes.Items[0], nil
}
