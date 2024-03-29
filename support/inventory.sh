#!/bin/sh
# Copyright 2023 Northern.tech AS
#
#    Licensed under the Apache License, Version 2.0 (the "License");
#    you may not use this file except in compliance with the License.
#    You may obtain a copy of the License at
#
#        http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS,
#    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#    See the License for the specific language governing permissions and
#    limitations under the License.
#
# The example script collects information about current host including:
#  * hostname
#  * kernel name
#  * cpu info
#  * Total memory
#  * Network interfaces
#
# Environment variable(s):
#
# INCLUDE_DOCKER_INTERFACES=true -- Include docker intefaces in output
#

set -ue

LC_ALL=C
export LC_ALL

grep 'model name' /proc/cpuinfo | uniq | awk -F': ' '
     // { printf("cpu_model=%s\n", $2);}
'
echo "kernel=$(cat /proc/version)"

cat /proc/meminfo | awk '
/MemTotal/ {printf("mem_total_kB=%d\n", $2)}
'

hostname="localhost"
hostname >/dev/null 2>&1 && hostname="$(hostname)"
[ "$hostname" = "" ] && [ -f /etc/hostname ] && hostname=$(cat /etc/hostname 2>/dev/null)
echo hostname=${hostname:-"localhost"}

INCLUDE_DOCKER_INTERFACES="${INCLUDE_DOCKER_INTERFACES:-false}"

SCN=/sys/class/net
min=65535
ifdev=

# find iface with lowest ifindex, except loopback
for devpath in $SCN/*; do
    dev=$(basename $devpath)
    if [ $dev = "lo" ]; then
        continue
    fi
    if [ "${INCLUDE_DOCKER_INTERFACES}" = "false" ]; then
        if echo $dev | grep -q -E '^(br-.*|docker.*|veth.*)'; then
            continue
        fi
    fi
    if ! [ "x$(cat $devpath/address)x" = "xx" ]; then
        echo "mac_$dev=$(cat $devpath/address)"
    fi
    echo "network_interfaces=$dev"

    ip addr show dev $dev | awk -v dev=$dev '
       /inet / { printf("ipv4_%s=%s\n", dev, $2) }
       /inet6 / {printf("ipv6_%s=%s\n", dev, $2) }
    '
done

OS="unknown"
for file in /etc/os-release /usr/lib/os-release; do
    if [ ! -e $file ]; then
        continue
    fi

    eval "$(grep -E '^(PRETTY_NAME|NAME|VERSION)=("[^"]*"|[^" ]*)' $file)"
    if [ -n "$PRETTY_NAME" ]; then
        OS="$PRETTY_NAME"
        break
    elif [ -n "$NAME" ] && [ -n "$VERSION" ]; then
        OS="$NAME $VERSION"
        break
    fi
done

if [ -z "$OS" ]; then
	for lsb_release in /bin/lsb_release /usr/bin/lsb_release; do
	    if [ -x $lsb_release ]; then
	        OS="$($lsb_release -sd)"
	        if [ -n "$OS" ]; then
	            break
	        fi
	    fi
	done
fi

if [ -z "$OS" ] && [ -e /etc/issue ]; then
    OS="$(cat /etc/issue)"
    if [ -n "$OS" ]; then
        break
    fi
fi

echo "os=$OS"
FS_TYPE="$(grep ' / ' /proc/mounts | grep -v "^rootfs" | awk '{print $3}')"
if [ -z "${FS_TYPE}" ]; then
    FS_TYPE=Unknown
fi
echo rootfs_type="$FS_TYPE"

exit 0
