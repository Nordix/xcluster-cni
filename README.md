# xcluster-cni

`Xcluster-cni` is a CNI-plugin for
[Kubernetes](https://kubernetes.io/). It is developed for
[xcluster](https://github.com/Nordix/xcluster) but can be used in any
K8s cluster. Key features;

* **Multi networking** - `xcluster-cni` can create POD-POD connectivity on
  secondary networks using. [Multus](
  https://github.com/k8snetworkplumbingwg/multus-cni) can be used to
  bring in extra interfaces to PODs

* **Flexible** - Making it useful for experiments. But it also lay a
  greater responsibility on the user. For instance network overlays
  are not directly supported, but you can create them yourself

* **Very small footprint** - `xcluster-cni` setup routes and watches
  the K8s node objects. If the K8s node objects doesn't change often
  the overhead created by `xcluster-cni` is basically none


The main components of `xcluster-cni` are:

* [kube-node IPAM](https://github.com/Nordix/ipam-node-annotation).
  Reads information (address ranges) from the K8s node object and
  assigns addresses by delegating to the `host-local` ipam

* The `xcluster-cni` router binary. Sets up routing to address ranges
  (CIDRs) between K8s nodes. This is deployed as a DaemonSet that will
  also install (and upgrade) the CNI-plugin on the nodes.


To install `xcluster-cni` as the main K8s CNI-plugin make sure
`--allocate-node-cidrs=true` is given to the
`kube-controller-manager` and do:

```bash
kubectl apply -n kube-system -f https://raw.githubusercontent.com/Nordix/xcluster-cni/master/xcluster-cni.yaml
```



### Multi Networking

A key feature is **multi networking**. `Xcluster-cni` can be installed
on different interfaces on K8s nodes and can create POD-POD
connectivity on secondary networks using [Multus](
https://github.com/k8snetworkplumbingwg/multus-cni). Installing
`xcluster-cni` on secondary networks requires annotations on Node
objects:

```
kubectl annotate node vm-003 cidr.example.com/net3=192.168.55.0/26,fd00:5::1:0/112
kubectl annotate node vm-003 node-ip.nordix.org/net3=192.168.2.3,1000::1:c0a8:203
```

and configuring the annotation names as environment variables:

```yaml
        env:
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
          - name: LOG_LEVEL
            value: "debug"
          - name: CIDR_ANNOTATION
            value: "cidr.example.com/net3"
          - name: ADDRESS_ANNOTATION
            value: "adr.example.com/net3"
          - name: PROTOCOL
            value: "200"
```

The `protocol` is used to handle routes and must be unique for each
`xcluster-cni` instance on the node. Default is 202.



### Network overlay

`xcluster-cni` does not setup a network overlay, but you may configure
an overlay yourself (e.g. vxlan) and use the `ADDRESS_ANNOTATION`
environment variable to stear traffic to the overlay. An MTU adjustment
is probably needed.

This area will see some improvements in the future.

### Network policies

K8s [network policies](
https://kubernetes.io/docs/concepts/services-networking/network-policies/)
are not supported.


## Build the xcluster-cni image

The image is built with "docker build" so `docker` must be installed.

```
./build.sh         # Help printout
./build.sh image   # Build the image
```



