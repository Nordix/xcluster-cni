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
##   install
##     Install or upgrade the CNI-plugins
cmd_install() {
	test -d /cni/net.d && cp /etc/cni/net.d/10-xcluster-cni.conf /cni/net.d
	test -d /cni/bin && cp /opt/cni/bin/* /cni/bin
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
