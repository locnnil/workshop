#!/bin/bash

# shellcheck source=tests/lib/retry.sh
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/retry.sh"

# Allow CI jobs to override environment.
LXD_CHANNEL='6/stable'
if [ -f "$SCRIPT_DIR/.env" ]; then
    . "$SCRIPT_DIR/.env"
fi

function setup_lxd() {
    if snap list lxd >/dev/null; then
        retry 5 snap refresh --channel="$LXD_CHANNEL" lxd
    else
        retry 5 snap install --channel="$LXD_CHANNEL" lxd
    fi
    lxd waitready --timeout=180

    # can already be initialised if reused
    # https://discuss.linuxcontainers.org/t/how-do-i-know-if-lxd-is-initialized/15473/3
    if [ "$(lxc storage list -f compact | grep -c default)" -eq 0 ]; then
        lxd init --auto --storage-backend=zfs
    fi
}

function prepare_environment() {
    # The unattended upgrades hold locks on reusable instances and can break a
    # spread run. This is to prevent the prepare script from failing (e.g. when
    # reusing an existing spread instance). Since workshops don't currently
    # interact with apt/dpkg on the host, it shouldn't have implications for the
    # tests.
    systemctl stop unattended-upgrades.service || true
    systemctl disable unattended-upgrades.service || true
    systemctl disable apt-daily.timer || true
    systemctl disable apt-daily-upgrade.timer || true

    # Configure apt to retry operations to handle flaky network issues
    # This helps with mirror sync errors and transient network failures
    cat <<EOF >/etc/apt/apt.conf.d/80-retries
Acquire::Retries "10";
Acquire::http::Timeout "30";
Acquire::http::Pipeline-Depth "0";
Acquire::CompressionTypes::Order { "gz"; "xz"; };
EOF

    while pgrep -f "apt|dpkg" >/dev/null; do
        echo "Waiting for any apt-related process to release the lock..."
        sleep 5
    done

    pkgs=(
        bsdutils
        fish
        jq
        "linux-modules-extra-$(uname -r)"
        moreutils
        zfsutils-linux
        zsh
    )
    retry 5 apt-get update
    apt-get install -y --no-install-recommends "${pkgs[@]}"

    mkdir -p /etc/systemd/system/snapd.service.d
    cat <<EOF >/etc/systemd/system/snapd.service.d/override.conf
# Workaround for https://bugs.launchpad.net/snapd/+bug/2104066
[Service]
Environment=SNAPD_STANDBY_WAIT=1m
EOF
    systemctl daemon-reload

    systemctl unmask snapd.service
    systemctl restart snapd.service
    snap wait system seed.loaded
    # The /snap directory does not exist in some environments
    [ ! -d /snap ] && ln -s /var/lib/snapd/snap /snap

    setup_lxd

    retry 5 snap install --classic --channel=1.25/stable go
    retry 5 snap install yq
}

function setup_workshop() {
    local use_real_store="${1:-false}"

    snap install --dangerous --classic /workshop/tests/*.snap

    if [ "$use_real_store" != "true" ]; then
        snap set workshop store.url=http://localhost:8080/storage/v1/
        start_sdk_store
    fi

    snap set workshop workshop.image.server.url="$IMAGE_SERVER"
    snap alias workshop.sdk sdk
    snap restart workshop

    # required to keep /lib/systemd/systemd --user running for a regular user
    loginctl enable-linger ubuntu
}

function cleanup_workshop() {
    local use_real_store="${1:-false}"

    if [ "$use_real_store" != "true" ]; then
        stop_sdk_store
    fi

    snap remove workshop --purge
    snap remove sdkcraft --purge
    find /workshop -name .workshop.lock -delete
    loginctl disable-linger ubuntu
}

function start_sdk_store() {
    # run fake GCS bucket storage to emulate SDK store
    publish_test_sdks "$TESTS_SDKS" "$SDK_STORE_BUCKET_DIR"
    # a bug with fake-gcs-server that returns 404 if not owned by the user
    chown -R ubuntu.ubuntu /data
    mkdir -p /storage
    chown -R ubuntu.ubuntu /storage

    go install github.com/fsouza/fake-gcs-server@latest
    fake-gcs-server -data /data -scheme http -port 8080 -public-host localhost:8080 >~/fake_sdk_store.log 2>&1 &

    echo "Waiting for the fake SDK store to start on port 8080..."
    while ! nc -z localhost 8080; do
        sleep 2
    done
}

function stop_sdk_store() {
    pkill -f fake-gcs-server || true
    rm -rf /data
    rm -rf /storage
}

# Publish test SDKs in the fake SDK Store
function publish_test_sdks() {
    for i in "$1"/*; do
        SDK_NAME=$(basename "$i")
        SDK_FILE=$SDK_NAME.sdk
        SDK_PATH=$(readlink -f "$i")
        STORE_PATH="$2/$SDK_NAME"

        echo "$SDK_NAME:"
        find "$SDK_PATH/" -mindepth 2 -maxdepth 2 -type d -printf "%P\n" | while IFS= read -r track; do
            printf "\t%s -> %s\n" "$track" "$STORE_PATH/$track"
            mkdir -p "$STORE_PATH/$track"
            rm -f "$STORE_PATH/$track/$SDK_FILE"

            readarray -d '' -t SDK_FILES < <(ls --almost-all --zero "$SDK_PATH/$track")
            tar \
                --create \
                --format=posix \
                --use-compress-program='zstd -10 --threads=0' \
                --mode='a-st,go-w' \
                --owner='root:0' \
                --group='root:0' \
                --mtime="$(date --utc +'%Y-%m-%dT%H:%M:%S.%6NZ')" \
                --sort=name \
                --force-local \
                --file="$STORE_PATH/$track/$SDK_FILE" \
                --directory="$SDK_PATH/$track" \
                "${SDK_FILES[@]}"

            cp -f "$SDK_PATH/$track/meta/sdk.yaml" "$STORE_PATH/$track"
        done
    done
}

# Workshop sub-command wrappers
function workshop_exec() {
    sudo -u ubuntu -- workshop "$@" 2>&1
}

function sdk_exec() {
    sudo -u ubuntu -- sdk "$@" 2>&1
}

function run_sdkcraft() {
    sdkcraft "$@"
}

# Install sdkcraft from a local snap file
function install_sdkcraft() {
    snap install --dangerous --classic /workshop/tests/sdkcraft_*.snap
}
