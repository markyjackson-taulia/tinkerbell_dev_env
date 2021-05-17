#!/usr/bin/env sh

# set -o errexit -o nounset -o pipefail

(
	echo "creating directory"
	mkdir -p "certs"
	./gencerts.sh
)

"$@"
