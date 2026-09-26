#!/bin/bash
set -e

CG=/sys/fs/cgroup/mydocker

mkdir -p $CG
echo "+memory +cpu +pids" > /sys/fs/cgroup/cgroup.subtree_control
echo 256M > $CG/memory.max
echo 0 > $CG/memory.swap.max
echo "50000 100000" > $CG/cpu.max
echo 30 > $CG/pids.max

echo $$ > $CG/cgroup.procs

exec capsh --drop=cap_sys_time -- -c '
  unshare --pid --fork --mount --mount-proc --net --uts --ipc \
    bash -c "hostname mycontainer; ip link set lo up; exec /home/nabbasov.linux/lab/api"
'
