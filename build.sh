#!/bin/bash

OS=$1

build () {
  echo "building" $1 $2

  dir="$(dirname -- $1)"
  file="$(basename -- $1)"
  binary="${file%".go"}"

  cd $dir

  if [[ $2 == "linux" ]]; then
    GOOS=linux GOARCH=amd64 go build -o $binary $file
  else
    go build -o $binary $file
  fi
}

printf "\nbuilding examples ${OS}\n"

rm -rf ./bin/;
mkdir -p bin;
cp -R ./examples ./bin;
export -f build;
os=$OS find ./bin/examples -name "*.go" -exec bash -c 'build {} "${os}"' \;

printf "done\n"
