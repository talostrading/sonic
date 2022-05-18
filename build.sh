#!/bin/bash

build () {
  echo $0

  dir="$(dirname -- $0)"
  file="$(basename -- $0)"
  built="${file%".go"}"

  cd $dir
  go build -o $built $file
}

printf "\nbuilding examples\n"

rm -rf ./bin/;
mkdir -p bin;
cp -R ./examples ./bin;
export -f build;
find ./bin/examples -name "*.go" -exec bash -c 'build "$0"' {} \;

printf "done\n"
