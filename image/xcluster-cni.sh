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

my_node_info=/tmp/my_node_info

##  env
##    Print environment.
##
cmd_env() {
	test -n "$K8S_NODE" || K8S_NODE=$(hostname)
	test "$cmd" = "env" && set | grep -E '^(__.*|K8S_NODE)='
}

# Print the own podCIDR's
cmd_pod_cidrs() {
	cmd_env
	list-nodes | jq "select(.metadata.name == \"$K8S_NODE\")" > $my_node_info
	if test "$(jq .spec.podCIDRs < $my_node_info)" != "null"; then
		jq -r '.spec.podCIDRs[]' < $my_node_info
	else
		jq -r '.spec.podCIDR' < $my_node_info
	fi
}

# Print the interface that holds the passed address
cmd_interface_for() {
	test -n "$1" || die "No address"
	local iface
	for iface in $(ip link show | grep -E '^[0-9]+:' | cut -d: -f2 | cut -d@ -f1); do
		if ip addr show dev $iface | grep -qF " $1/"; then
			echo $iface
			return
		fi
	done
}

# Try to find the MTU for the k8s insterface and compensate for the
# tunnel-header if necessary. Then update the passed file.
update_mtu() {
	local adr=$(cat $my_node_info | jq -r '[.status.addresses[]|select(.type == "InternalIP")][0].address')
	log "K8s node address; $adr"
	test -z "$adr" -o "$adr" = "null" && return 0
	local iface=$(cmd_interface_for $adr)
	test -n "$iface" || return 0
	local mtu=$(ip link show dev $iface | grep -oE 'mtu [0-9]+' | cut -d' ' -f2)
	if test -n "$mtu" -a "$mtu" != "null"; then
		test "$TUNNEL_MODE" = "sit" && mtu=$((mtu-20))
		log "Using MTU=$mtu"
		ip link set dev sit0 mtu $mtu
		sed -i -e "s,1500,$mtu," $1
	fi
}

##	start
##	  Start xcluster-cni. This is the container entry-point.
##
cmd_start() {
	echo "xcluster-cni; $(cat /build-date)"
	echo "K8S_NODE=[$K8S_NODE]"
	cmd_env
	ip link add name cbr0 type bridge
	ip link set dev cbr0 up
	cmd_pod_cidrs > /opt/cni/bin/podCIDR
	while ! test -s /opt/cni/bin/podCIDR; do
		log "No podCIDRs found"
		sleep 5
		cmd_pod_cidrs > /opt/cni/bin/podCIDR
	done
	log "Generated /cni/bin/podCIDR"
	if test -d /cni/bin; then
		cp -r /opt/cni/bin/* /cni/bin
	fi
	if test -d /cni/net.d; then
		if ! test -r /cni/net.d/10-xcluster-cni.conf; then
			cp /etc/cni/net.d/10-xcluster-cni.conf /cni/net.d
			update_mtu /cni/net.d/10-xcluster-cni.conf
		fi
	fi
	set_sit0_address
	exec /bin/xcluster-cni-router.sh monitor
}

# The sit0 ipv4 address must be set to the same as the k8s InternalIP
# (but with /32). Otherwise the routing may become asymetric and not working.
set_sit0_address() {
	test "$TUNNEL_MODE" = "sit" || return
	local a a4
	for a in $(cat $my_node_info | jq -r '.status.addresses[]|select(.type == "InternalIP").address'); do
		echo $a | grep -q : && continue
		a4=$a
		break
	done
	if test -n "$a4"; then
		log "Set addr $a4/32 on dev sit0"
		ip link set up dev sit0
		ip addr add $a4/32 dev sit0
	fi
	return 0
}


##  stop
##    Stop xcluster-cni and cleanup. This is the container "preStop" hook.
##
cmd_stop() {
	cmd_env
	kill 1
	ip link set dev cbr0 down
	ip link del dev cbr0
	test -d /cni/bin && rm -f /cni/bin/podCIDR /cni/bin/node-local
	test -d /cni/net.d && rm /cni/net.d/10-xcluster-cni.conf
	/bin/xcluster-cni-router.sh remove_routes
	return 0
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
