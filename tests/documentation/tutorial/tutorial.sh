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
workshop info nimble
cd ..
mv hello-workshop hi-workshop
workshop list --global
mv hi-workshop hello-workshop


# Changes and tasks after launch

workshop changes
workshop tasks $(workshop changes | awk 'END{print $1}')


# Start and stop

workshop stop nimble
workshop start nimble


# Refresh

# Change base

cat > .workshop.nimble.yaml << EOF
name: nimble
base: ubuntu@20.04
sdks:
  go:
    channel: latest/stable
EOF

workshop refresh nimble


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

# Interactive shell needs a tweak in testing
workshop exec nimble -- bash -c "pwd"
workshop exec nimble -- bash -c "uname -a"

# Changes in project
touch outside.txt
workshop exec nimble -- bash -c "ls -l"
workshop exec nimble -- touch inside.txt
ls -l

# Interfaces

workshop connections
workshop disconnect nimble/go:mod-cache
workshop connect nimble/go:mod-cache :content
workshop remount nimble/go:mod-cache ~/new-location/
workshop info nimble

workshop remove nimble
