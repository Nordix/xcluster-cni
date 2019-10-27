#! /bin/sh
##
## xcluster-cni.sh --
##
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
dbg() {
	test -n "$__verbose" && echo "$prg: $*" >&2
}

##  env
##    Print environment.
##
cmd_env() {
	test -n "$K8S_NODE" || K8S_NODE=$(hostname)
	test "$cmd" = "env" && set | grep -E '^(__.*|K8S_NODE)='
}

cmd_pod_cidrs() {
	cmd_env
	mkdir -p $tmp
	list-nodes | jq -r "select(.metadata.name == \"$K8S_NODE\")" > $tmp/out
	if test "$(jq .spec.podCIDRs < $tmp/out)" != "null"; then
		jq -r '.spec.podCIDRs[]' < $tmp/out
	else
		jq -r '.spec.podCIDR' < $tmp/out
	fi
}

##	start
##	  Start xcluster-cni. This is the container entry-point.
##
cmd_start() {
	echo "K8S_NODE=[$K8S_NODE]"
	cmd_env
	ip link add name cbr0 type bridge
	ip link set dev cbr0 up
	if test -d /cni/bin; then
		cp -r /opt/cni/bin/* /cni/bin
		cmd_pod_cidrs >  /cni/bin/podCIDR
	fi
	test -d /cni/net.d && cp -r /etc/cni/net.d/* /cni/net.d
	exec /bin/xcluster-cni-router.sh monitor
}

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
