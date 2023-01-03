#! /bin/sh
##
## build.sh --
##   Build xcluster-cni.
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
	echo "INFO: $*" >&2
}
dbg() {
	test -n "$__verbose" && echo "$prg: $*" >&2
}
findf() {
	f=$ARCHIVE/$1
	test -r $f || f=$HOME/Downloads/$1
	test -r $f || die "Not found [$f]"
}

##  env
##    Print environment.
##
cmd_env() {
	test -n "$__tag" || __tag=registry.nordix.org/cloud-native/xcluster-cni:latest
	if ! test -n "$__plugin_tar"; then
		findf cni-plugins-linux-amd64-v1.1.1.tgz
		__plugin_tar=$f
	fi
	test "$cmd" = "env" && set | grep -E '^(__.*)='
}
##  annotate_nodes --cidrs=<double-dash-cidrs> --annotation=<name>
##    Annotate nodes in a cluster. The CIDRs should be in double-dash
##    format. Only some nodes may get IPv4 ranges. Example:
##
##      --cidrs=172.20.0.0/24/28,fd00:1000/96/112
##
cmd_annotate_nodes() {
	test -n "$__cidrs" || die 'Cidrs not specified'
	test -n "$__annotation" || die 'Annotation not specified'
	mkdir -p $tmp
	kubectl get nodes -o name > $tmp/nodes || die "Can't get nodes"
	log "Found $(cat $tmp/nodes | wc -l) nodes"
	local cidr1 cidr2 v
	cidr1=$(echo $__cidrs | cut -d, -f1)
	cidr2=$(echo $__cidrs | cut -d, -f2)
}
##  binaries [--version=]
##    Build binaries in ./_output
cmd_binaries() {
	cmd_env
	cd $dir
	mkdir -p _output
	test -n "$__version" || __version=$(date +%Y.%m.%d-%H.%M.%S)
	CGO_ENABLED=0 GOOS=linux go build \
		-ldflags "-extldflags '-static' -X main.version=$__version $LDFLAGS" \
		-o _output ./cmd/... || die "go build"
	strip _output/*
}
##  image [--tag=image:version] [--plugin-tar=file]
##    Build the "xcluster-cni" image.
cmd_image() {
	cmd_env
	local ver=$(echo $__tag | cut -d: -f2)
	echo "$ver" | grep -qE 'latest|local' || __version=$ver
	test -r "$__plugin_tar" || die "Not readable [$__plugin_tar]"
	cmd_binaries
	mkdir -p $tmp/bin
	cp -r $dir/image/* $tmp
	cp $dir/_output/xcluster-cni $tmp/bin
	mkdir -p $tmp/opt/cni/bin
	tar -C $tmp/opt/cni/bin -xf "$__plugin_tar" ./host-local ./bridge
	local d=$GOPATH/src/github.com/Nordix/ipam-node-annotation
	if test -x $d/_output/kube-node; then
		cp $d/_output/kube-node $tmp/opt/cni/bin
	else
		findf kube-node.xz $tmp/opt/cni/bin
		xz -d $tmp/opt/cni/bin/kube-node.xz
		chmod a+x $tmp/opt/cni/bin/kube-node
	fi
	cd $tmp
	rm -f Dockerfile
	docker build -f $dir/image/Dockerfile -t $__tag .
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
