#! /bin/sh
##
## xcluster-cni.sh --
##
##   The container start point for github.com/Nordix/xcluster-cni
##
## Commands;
##

prg=$(basename $0)
dir=$(dirname $0); dir=$(readlink -f $dir)
tmp=/tmp/${prg}_$$

die() {
    echo "ERROR: $*" >&2
    rm -rf $tmp
    exit 1
}
help() {
    grep '^##' $0 | cut -c3-
    rm -rf $tmp
    exit 0
}
test -n "$1" || help
echo "$1" | grep -qi "^help\|-h" && help

log() {
	echo "$(date +%T): $*" >&2
}

##   env
##     Print environment.
cmd_env() {
	test "$cmd" = "env" && set | grep -E '^(__.*|K8S_NODE)='
}
##   generate_kubeconfig [--path=/cni/net.d/xcluster-cni.kubeconfig]
##     Generate a kubeconfig file
cmd_generate_kubeconfig() {
	# Seems to originate from;
	# https://github.com/projectcalico/cni-plugin/blob/be4df4db2e47aa7378b1bdf6933724bac1f348d0/k8s-install/scripts/install-cni.sh#L104-L153
	test -n "$__path" || __path=/cni/net.d/xcluster-cni.kubeconfig
	log "Generate kubeconfig; $__path"
	mkdir -p $(dirname $__path) || die "mkdir $(dirname $__path)"
	local sa=/var/run/secrets/kubernetes.io/serviceaccount
	KUBE_CA_FILE=$sa/ca.crt
	SKIP_TLS_VERIFY=false
	# Pull out service account token.
	SERVICEACCOUNT_TOKEN=$(cat $sa/token)

  test -f "$KUBE_CA_FILE" && \
      TLS_CFG="certificate-authority-data: $(cat $KUBE_CA_FILE | base64 | tr -d '\n')"

  # Write a kubeconfig file for the CNI plugin.  Do this
  # to skip TLS verification for now.  We should eventually support
  # writing more complete kubeconfig files. This is only used
  # if the provided CNI network config references it.
  cat > $__path <<EOF
apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}
    $TLS_CFG
users:
- name: multus
  user:
    token: "${SERVICEACCOUNT_TOKEN}"
contexts:
- name: multus-context
  context:
    cluster: local
    user: multus
current-context: multus-context
EOF
}
##   install
##     Install or upgrade the CNI-plugins. This is the initContainer
##     entry-point.
cmd_install() {
	if test -d /cni/net.d; then
		echo "Checking K8s CNI-plugin"
		if echo "$INSTALL_K8S_NET" | grep -qi -E 'no|false'; then
			cmd_generate_kubeconfig
		else
			if find /cni/net.d -maxdepth 1 -type f | sort | grep -q xcluster; then
				echo "A Xcluster-cni already K8s CNI-plugin. Nothing updated"
			else
				echo "Installing xcluster-cni as K8s CNI-plugin"
				cp /etc/cni/net.d/10-xcluster-cni.conf /cni/net.d
				local mtu=$(xcluster-cni k8smtu)
				if test $mtu -ne 1500; then
					echo "Set MTU=$mtu"
					sed -i -e "s,1500,$mtu," /cni/net.d/10-xcluster-cni.conf
				fi
				cmd_generate_kubeconfig
			fi
		fi
	else
		echo "K8s CNI-plugin NOT installed"
	fi
	if test -d /cni/bin; then
		echo "Checking CNI-plugin binaries"
		local p f ver xver
		for p in $(find /opt/cni/bin/ -maxdepth 1 -type f -executable); do
			f=$(basename $p)
			if ! test -e /cni/bin/$f; then
				echo "Installing $f"
				cp $p /cni/bin
				continue
			fi
			if echo $f | grep -q -E "host-local|bridge"; then
				ver=$($p -V 2>&1)
				xver=$(/cni/bin/$f -V 2>&1)
				echo "$f: have $ver, installed $xver"
				if test "$ver" = "$xver"; then
					echo "Same version. No upgrade"
				else
					echo "Upgrade to $ver"
					cp $p /cni/bin
				fi
			elif echo $f | grep -q "kube-node"; then
				ver=$($p -version)
				xver=$(/cni/bin/$f -version)
				echo "$f: have $ver, installed $xver"
				if test "$ver" = "$xver"; then
					echo "Same version. No upgrade"
				else
					echo "Upgrade to $ver"
					cp $p /cni/bin
				fi
			else
				echo "Force upgrade $f"
				cp $p /cni/bin
			fi
		done
	else
		echo "CNI-plugin binaries NOT installed"
	fi
}
##   start
##	   Start xcluster-cni. This is the container entry-point.
cmd_start() {
	exec xcluster-cni daemon
}

##
# Get the command
cmd=$1
shift
grep -q "^cmd_$cmd()" $0 $hook || die "Invalid command [$cmd]"

while echo "$1" | grep -q '^--'; do
    if echo $1 | grep -q =; then
	o=$(echo "$1" | cut -d= -f1 | sed -e 's,-,_,g')
	v=$(echo "$1" | cut -d= -f2-)
	eval "$o=\"$v\""
    else
	o=$(echo "$1" | sed -e 's,-,_,g')
	eval "$o=yes"
    fi
    shift
done
unset o v
long_opts=`set | grep '^__' | cut -d= -f1`

# Execute command
trap "die Interrupted" INT TERM
cmd_$cmd "$@"
status=$?
rm -rf $tmp
exit $status
