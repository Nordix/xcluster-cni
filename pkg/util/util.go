package util

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"k8s.io/client-go/kubernetes"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Nordix/xcluster-cni/pkg/log"
)

// GetClientset Returns a Kubernetes Clientset. Works inside PODs as well
// as outside
func GetClientset() (*kubernetes.Clientset, error) {
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
func GetApi(ctx context.Context) core.CoreV1Interface {
	clientset, err := GetClientset()
	if err != nil {
		log.Fatal(ctx, "Get clientset", "error", err)
	}
	return clientset.CoreV1()
}

// EmitJson Prints the object in json format on stdout
func EmitJson(object any) {
	if s, err := json.Marshal(object); err == nil {
		fmt.Println(string(s))
	}
}

// CreateCIDR Creates a CIDR from a "double-dash" CIDR and a node number.
func CreateCIDR(doubleDashCIDR string, nodeNo uint) (string, error) {
	citems := strings.Split(doubleDashCIDR, "/")
	if len(citems) != 3 {
		return "", fmt.Errorf("Invalid DoubleDashCIDR %s", doubleDashCIDR)
	}
	bits1, err := strconv.Atoi(citems[1])
	if err != nil {
		return "", fmt.Errorf("Bits1 invalid %s", citems[1])
	}
	bits2, err := strconv.Atoi(citems[2])
	if err != nil {
		return "", fmt.Errorf("Bits2 invalid %s", citems[2])
	}
	if bits1 > bits2 {
		return "", fmt.Errorf("Bits1 < bits2")
	}
	if (bits2-bits1) < 32 && nodeNo >= 1<<(bits2-bits1) {
		// There is no room in the "node-field" for this number
		return "", fmt.Errorf("nodeNo too large")
	}

	_, ipNet, err := net.ParseCIDR(strings.Join(citems[0:2], "/"))
	if err != nil {
		return "", fmt.Errorf("Failed to parse CIDR")
	}
	if ipNet.IP.To4() != nil {
		// An IPv4 CIDR
		if bits2 > 32 {
			return "", fmt.Errorf("Bits2 too high")
		}
		if bits1 < 8 {
			return "", fmt.Errorf("Bits1 too low. Must be >=8 for IPv4")
		}
		shiftOr(ipNet.IP, uint64(nodeNo), 32-bits2)
	} else {
		// An IPv6 CIDR
		if bits2 > 128 {
			return "", fmt.Errorf("Bits2 too high")
		}
		if bits1 < 48 {
			return "", fmt.Errorf("Bits1 too low. Must be >=48 for IPv6")
		}
		shiftOr(ipNet.IP, uint64(nodeNo), 128-bits2)
	}
	return fmt.Sprintf("%s/%d", ipNet.IP.String(), bits2), nil
}

// shiftOr Shift the number left and OR it with the bytes.
func shiftOr(b []byte, n uint64, shift int) {
	// 1. Shift the number by the fraction of 8
	// 2. Convert the number to bytes in network byte order (big endian)
	// 3. OR bytes with a byte-shift offset
	// (it feels like there HAS to be a better way!)
	n = n << (shift % 8)
	nb := make([]byte, 8)
	binary.BigEndian.PutUint64(nb, n)
	inb := 7
	for i := len(b) - 1 - (shift / 8); i >= 0; i-- {
		if inb < 0 {
			break
		}
		b[i] = b[i] | nb[inb]
		inb--
	}
}
