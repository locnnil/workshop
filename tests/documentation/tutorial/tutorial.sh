#!/bin/bash
  
set -eux
whoami

# Check prerequisites (snapcraft doesn't appear here deliberately)

sudo snap install lxd
sudo lxd init --auto

sudo snap start --enable lxd.daemon
snap services lxd.daemon


# Install

cd ~/workshop
sudo snap install snapcraft --classic  # needed to build from source only
snapcraft

sudo snap install --devmode ./workshop_0.1.0_amd64.snap
sleep 5 # naively wait for the service to start, FIXME
# Run
workshop --help

# Define

mkdir hello-workshop
cd hello-workshop

cat > .workshop.nimble.yaml << EOF
name: nimble
base: ubuntu@22.04
sdks:
  go:
    channel: latest/stable
EOF

workshop list


# Launch

workshop launch nimble
cd ..
mv hello-workshop hi-workshop
cd hi-workshop
workshop list


# Note: Changes and tasks after launch

workshop changes
workshop tasks $(workshop changes | awk 'END{print $1}')


# Start and stop

workshop stop nimble
workshop start nimble


# Update components

workshop refresh nimble


# Add or remove an SDK

cat > .workshop.nimble.yaml << EOF
name: nimble
base: ubuntu@22.04
sdks:
  go:
    channel: latest/edge
EOF

# Expected to fail - the latest/edge channel above does not exist
workshop refresh --wait-on-error nimble || true


# Wait on error/Note: Changes and tasks after refresh

workshop changes
workshop tasks $(workshop changes | awk 'END{print $1}')

# Expected to fail, nothing changed
workshop refresh --continue nimble || true
workshop refresh --abort nimble


# Execute commands

cat > main.go << EOF
package main

import "fmt"

func main() {
fmt.Println("hello, Workshop")
}
EOF

workshop exec nimble go build main.go
workshop exec nimble --env GO111MODULE=off -- go build -x
workshop exec nimble -- bash -c "uname -a"
workshop remove nimble
