#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
echdir="$workspace/src/github.com/etvchaineum"
if [ ! -L "$echdir/go-etvchaineum" ]; then
    mkdir -p "$echdir"
    cd "$echdir"
    ln -s ../../../../../. go-etvchaineum
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

# Run the command inside the workspace.
cd "$echdir/go-etvchaineum"
PWD="$echdir/go-etvchaineum"

# Launch the arguments with the configured environment.
exec "$@"
