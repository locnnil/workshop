#!/usr/bin/env bash
#
# Workaround for https://github.com/canonical/lxd/issues/18362.

set -e

pid=$(</var/snap/lxd/common/lxd.pid)

pending() {
    findmnt --noheadings --output=TARGET --raw --task="$pid" > mounted.log
    grep '/storage-pools/workshop/containers/workshop-snapshots\.' < mounted.log > mounted-snapshots.log
}

for ((i = 0; i < 120; i++)); do
    status=0
    pending || status="$?"
    if [ "$status" -eq 0 ]; then
        echo 'Still mounted:'
        cat mounted-snapshots.log
        sleep 1
    elif [ "$status" -eq 1 ]; then
        exit 0
    else
        exit "$status"
    fi
done

echo "Timed out!"
exit 1
