# Xcluster/ovl - xcluster-cni-test

Test `xcluster-cni` in [xcluster](https://github.com/Nordix/xcluster)


Basic checks when `xcluster-cni` is used as the K8s main CNI-plugin:

```bash
./xcluster-cni-test.sh   # (help printout)
./xcluster-cni-test.sh build_image  # Use :local and upload to local registry
./xcluster-cni-test.sh test --no-stop ping > $log
kubectl get pods -A
pod=$(kubectl get pods -n kube-system -l app=xcluster-cni -o name | head -1)
kubectl -n kube-system logs $pod -c install
kubectl -n kube-system logs $pod -c xcluster-cni | jq
```


