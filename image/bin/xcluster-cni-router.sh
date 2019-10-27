#! /bin/sh
##
## xcluster-cni-router.sh --
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
	if test -z "$__log_file"; then
		echo "$prg: $*"
		return 0
	fi
	echo "$prg $(date +%T): $*" >> "$__log_file"
	return 0
}
dbg() {
	test -n "$__verbose" && echo "$prg: $*" >&2
}

##  env
##    Print environment.
##
cmd_env() {
	test -n "$__info_file" || __info_file=/tmp/node-info
	test -n "$K8S_NODE" || K8S_NODE=$(hostname)
	test "$cmd" = "env" && set | grep -E '^(__.*)='
}

##  info
##    Print a json array with name, address and podCIDR for all nodes
##
cmd_info() {
	list-nodes | jq '[.|{name: .metadata.name,podCIDRs: .spec.podCIDRs, address: .status.addresses[]|select(.type == "InternalIP").address}]|sort_by(.name)'
}

##  check_info [--info-file=/tmp/node-info]
##    Read info and returns ok (0) if the info is updated
##
cmd_check_info() {
	cmd_env

	mkdir -p $tmp
	if ! cmd_info > $tmp/node-info; then
		log "Failed to read node-info"
		rm -f $tmp/node-info
		return 0
	fi

	if ! test -r "$__info_file"; then
		log "First node-info read"
		cp $tmp/node-info "$__info_file"
		return 0
	fi
	
	if ! diff -q "$__info_file" $tmp/node-info > /dev/null; then
		log "Node-info updated"
		cp $tmp/node-info "$__info_file"
		return 0
	fi

	return 1
}

##  check_routes
##    Check routes to podCIDR's, update if needed.
##  monitor [--interval=5]
##    Sit in a loop doing "check_routes".
##
cmd_check_routes() {
	cmd_check_info || return 0
	cmd_update_routes
	return 0      # Must return ok!
}
cmd_monitor() {
	test -n "$__interval" || __interval=5
	while true; do
		cmd_check_routes
		sleep $__interval
	done
}


##  update_routes [--info-file=/tmp/node-info]
##    Update routes to podCIDR's
##
cmd_update_routes() {
	cmd_env
	if ! test -r "$__info_file"; then
		log "Not readable [$__info_file]"
		return 0
	fi

	local n i cidr a h
	h=$K8S_NODE
	for n in $(cat $__info_file | jq -r '.[].name'); do
		test "$n" = "$h" && continue
		i=$(cat $__info_file | jq ".[]|select(.name == \"$n\")")
		a=$(echo $i | jq -r .address)
		for cidr in $(echo $i | jq -r '.podCIDRs[]'); do
			echo $cidr | grep -qi null | continue
			if echo $cidr | grep -q : ; then
				if echo $a | grep -q : ; then
					cmd_x ip -6 ro replace $cidr via $a
				else
					cmd_x ip -6 ro replace $cidr via 1000::1:$a
				fi
			else
				cmd_x ip ro replace $cidr via $a
			fi
		done
	done
}

cmd_x() {
	log "$@"
	test "$__dry_run" = "yes" && return 0
	$@
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
