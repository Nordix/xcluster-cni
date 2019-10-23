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
	echo "$prg: $*" >&2
}
dbg() {
	test -n "$__verbose" && echo "$prg: $*" >&2
}

##  env
##    Print environment.
##
cmd_env() {
	test -n "$__image" || __image=registry.nordix.org/cloud-native/xcluster-cni
	test -n "$__version" || __version=master
	test -n "$__plugin_tar" || __plugin_tar=$HOME/Downloads/cni-plugins-linux-amd64-v0.8.2.tgz
	test -n "$__node_local" || __node_local=$GOPATH/src/github.com/Nordix/ipam-node-local/node-local
	test "$cmd" = "env" && set | grep -E '^(__.*)='
}

##  image [--image=name] [--version=master] [--plugin-cni-plugins.tgz]
##    Build the "xcluster-cni" image.
##
cmd_image() {
	cmd_env
	test -r "$__plugin_tar" || die "Not readable [$__plugin_tar]"
	test -r "$__node_local" || \
		die "Not readable [$__node_local]. Do 'go get -d github.com/Nordix/ipam-node-local/node-local'"
	mkdir -p $dir/image/opt/cni/bin
	tar -C $dir/image/opt/cni/bin -xf "$__plugin_tar" ./host-local ./bridge
	cp "$__node_local" $dir/image/opt/cni/bin
	GO111MODULE=on CGO_ENABLED=0 GOOS=linux \
		go build -ldflags "-extldflags '-static' -X main.version=$__version" \
		-o image/bin/list-nodes ./cmd/...
	strip image/bin/list-nodes
	docker build -t $__image:$__version .
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
