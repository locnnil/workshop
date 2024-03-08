#!/bin/bash
  
set -eux
whoami


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

# Variable injection
workshop exec nimble --env GO111MODULE=off -- go build -x

# Omitting 'Interactive shell' in testing
workshop exec nimble -- bash -c "uname -a"

# Changes in project
touch outside.txt
workshop exec nimble -- bash -c "ls -l"
workshop exec nimble -- touch inside.txt
ls -l

workshop remove nimble
