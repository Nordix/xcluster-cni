#! /bin/sh
##
## xcluster-cni-test.sh --
##
##   Help script for test of xcluster-cni in KinD
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
	echo "$prg: $*" >&2
}

##  env
##    Print environment.
cmd_env() {
	test -n "$KIND_CLUSTER_NAME" || export KIND_CLUSTER_NAME=xcluster-cni-test
	if test "$cmd" = "env"; then
		set | grep -E '^(__.*|ARCHIVE)='
		return 0
	fi
}
##   private_reg [--localhost]
##     Print the address of the local private registry. --localhost will
##     print "localhost:<port>" which is needed for local upload.
cmd_private_reg() {
	mkdir -p $tmp
	docker inspect registry | jq -r '.[0].NetworkSettings' > $tmp/private_reg \
		|| die "No private registry?"
	local port=$(cat $tmp/private_reg | jq -r '.Ports."5000/tcp"[0].HostPort')
	if test "$__localhost" = "yes"; then
		echo "localhost:$port"
		return 0
	fi
	local adr=$(cat $tmp/private_reg | jq -r .Gateway)
	echo "$adr:$port"
}
##   kind_start
##     Start a Kubernetes-in-Docker (KinD) cluster for Meridio tests.
##     NOTE: Images are loaded from the private registry!
cmd_kind_start() {
	cmd_kind_stop > /dev/null 2>&1
	# Use default kind config and alter the private registry if needed
	local private_reg=$(cmd_private_reg)
	log "Using private registry [$private_reg]"
	local kind_config=$dir/kind.yaml
	if test "$private_reg" != "172.17.0.1:80"; then
		mkdir -p $tmp
		sed -e "s,172.17.0.1,$private_reg," < $kind_config > $tmp/meridio.yaml
		kind_config=$tmp/kind.yaml
	fi
	log "Start KinD cluster [$KIND_CLUSTER_NAME] ..."
	kind create cluster --name $KIND_CLUSTER_NAME --config $kind_config \
		$KIND_CREATE_ARGS || die
	kubectl create -f $dir/../../xcluster-cni.yaml || die
}
##   kind_stop
##     Stop and delete KinD cluster
cmd_kind_stop() {
	cmd_env
	kind delete cluster --name $KIND_CLUSTER_NAME
}
##   kind_sh [node]
##     Open a xterm-shell on a KinD node (default control-plane).
cmd_kind_sh() {
	cmd_env
	local node=control-plane
	test -n "$1" && node=$1
	if echo $node | grep -q '^trench'; then
		xterm -bg "#400" -fg wheat -T $node -e docker exec -it $node sh &
		return 0
	fi
	xterm -bg "#040" -fg wheat -T $node -e docker exec -it $KIND_CLUSTER_NAME-$node bash -l &
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
