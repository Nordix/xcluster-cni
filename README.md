# xcluster-cni

A basic [Kubernetes](https://kubernetes.io/) CNI-plugin.

This is the basic CNI-plugin used in [xcluster](https://github.com/Nordix/xcluster). It uses;

* [bridge plugin](https://github.com/containernetworking/plugins/tree/master/plugins/main/bridge)

* [host-local ipam](https://github.com/containernetworking/plugins/tree/master/plugins/ipam/host-local)

The `xcluster-cni` is a very simple CNI-plugin which makes ti suitable for
experiments and as an introduction to container networking in Kubernetes.

