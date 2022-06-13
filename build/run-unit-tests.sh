#!/bin/bash -e

###############################################################################
# Copyright (c) Red Hat, Inc.
# Copyright Contributors to the Open Cluster Management project
###############################################################################

_script_dir=$(dirname "$0")

if ! which gocovmerge > /dev/null; then  echo "Installing gocovmerge..."; pushd $(mktemp -d) && GOSUMDB=off go install github.com/wadey/gocovmerge@latest  && popd; fi
if ! which patter > /dev/null; then      echo "Installing patter ..."; pushd $(mktemp -d) && GOSUMDB=off go install github.com/apg/patter@latest && popd; fi

export GOFLAGS=""
mkdir -p test/unit/coverage
echo 'mode: atomic' > test/unit/coverage/cover.out
echo '' > test/unit/coverage/cover.tmp
echo -e "${GOPACKAGES// /\\n}" | xargs -n1 -I{} $_script_dir/test-package.sh {} ${GOPACKAGES// /,}

if [ ! -f test/unit/coverage/cover.out ]; then
    echo "Coverage file test/unit/coverage/cover.out does not exist"
    exit 0
fi

COVERAGE=$(go tool cover -func=test/unit/coverage/cover.out | grep "total:" | awk '{ print $3 }' | sed 's/[][()><%]/ /g')
echo "-------------------------------------------------------------------------"
echo "TOTAL COVERAGE IS ${COVERAGE}%"
echo "-------------------------------------------------------------------------"

go tool cover -html=test/unit/coverage/cover.out -o=test/unit/coverage/cover.html
