#! /bin/sh
##
## xcluster-cni-test.sh --
##
##   Help script for the xcluster ovl/xcluster-cni-test.
##

prg=$(basename $0)
dir=$(dirname $0); dir=$(readlink -f $dir)
me=$dir/$prg
tmp=/tmp/${prg}_$$
test -n "$PREFIX" || PREFIX=1000::1

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

## Commands;
##

##   env
##     Print environment.
cmd_env() {

	if test "$cmd" = "env"; then
		set | grep -E '^(__.*)='
		return 0
	fi

	topd=$(readlink -f $dir/../..)
	images=$($XCLUSTER ovld images)/images.sh
	test -n "$xcluster_DOMAIN" || xcluster_DOMAIN=xcluster
	test -n "$XCLUSTER" || die 'Not set [$XCLUSTER]'
	test -x "$XCLUSTER" || die "Not executable [$XCLUSTER]"
	eval $($XCLUSTER env)
}
##   build_image
##     Re-build the xcluster-cni image
cmd_build_image() {
	cmd_env
	test -n "$__version" || __version=local
	cd $topd
	export __tag=registry.nordix.org/cloud-native/xcluster-cni:$__version
	./build.sh image || die "build"
	$images lreg_upload --strip-host $__tag
}

##
## Tests;
##   test [--xterm] [--no-stop] [test] [ovls...] > logfile
##     Exec tests
cmd_test() {
	cmd_env
	start=starts
	test "$__xterm" = "yes" && start=start
	rm -f $XCLUSTER_TMP/cdrom.iso

	local t
	if test -n "$1"; then
		t=$1
		shift
		test_$t $@
	else
		unset __no_stop
		for t in ping multilan install_secondary restart_pod; do
			tlog "========== test $t"
			$me test $t
		done
	fi		

	now=$(date +%s)
	tlog "Xcluster test ended. Total time $((now-begin)) sec"
}
##   test start_empty
##     Start empty cluster
test_start_empty() {
	export xcluster_PREFIX=$PREFIX
	test -n "$__nrouters" || export __nrouters=1
	cd $dir
	xcluster_start . $@
	otc 1 check_namespaces
	otc 1 check_nodes
	otcr vip_routes
}
##   test start
##     Start cluster xcluster-cni
test_start() {
	export xcluster_CNI_INFO=xcluster-cni-test
	test -n "$__nvm" || export __nvm=5
	export __image=$XCLUSTER_HOME/hd-k8s-xcluster.img
	test_start_empty $@
	otc 1 version
}
##   test start_multilan
##     Start a cluster with multilan and multus
test_start_multilan() {
	export TOPOLOGY=multilan-router
	export xcluster_TOPOLOGY=$TOPOLOGY
	. $($XCLUSTER ovld network-topology)/$TOPOLOGY/Envsettings
	export __nrouters=2
	test_start_empty multus $@
	otc 1 multus_crd
}
##   test start_routing
##     Start a multilan cluster and setup routing using "xcluster-cni
##     daemon" on eth2 and eth3
test_start_routing() {
	export INSTALL_BINARIES=yes
	test_start_multilan $@
	otc 1 "annotate eth2"
	otcw "daemon eth2"
	otc 1 "annotate eth3"
	otcw "daemon eth3"
}
##   test ping
##     Start with xcluster-cni and test ping between PODs
test_ping() {
	test_start $@
	otc 1 start_alpine
	otc 1 "collect_addresses --app=alpine eth0"
	otc 1 ping
	xcluster_stop
}
##   test multilan
##     Test PODs with interfaces to net3 and net4 (eth2 and eth3 on nodes).
##     POD addresses on net3 and net4 are ping'ed from main netns
test_multilan() {
	test_start_routing $@
	otc 1 alpine2
	otc 2 "collect_addresses net3"
	otc 2 "collect_addresses net4"
	otc 2 "ping_collected_addresses net3"
	otc 2 "ping_collected_addresses net4"
	xcluster_stop
}
##   test install_secondary
##     Test installation for secondary network (only). Test re-start
##     of the routing POD and verify that no re-installation of
##     existing binaries are done
test_install_secondary() {
	test_start_multilan $@
	otc 1 "annotate eth2"
	otc 1 "annotate eth3"
	otc 1 "install_secondary eth2 eth3"
	otc 1 alpine2
	otc 2 "collect_addresses net3"
	otc 2 "collect_addresses net4"
	otc 2 "ping_collected_addresses net3"
	otc 2 "ping_collected_addresses net4"
	xcluster_stop
}
##   test restart_pod
##     Re-start a POD and verify that no binaries are re-installed
test_restart_pod() {
	test_start_multilan $@
	otc 1 "annotate eth2"
	otc 1 "install_secondary eth2"
	otc 2 restart_pod
	xcluster_stop
}


##
. $($XCLUSTER ovld test)/default/usr/lib/xctest
indent=''

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
