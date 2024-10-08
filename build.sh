#!/usr/bin/env bash

set -euo pipefail

readonly APP_NAME=fuzz-lsp.exe
readonly PROJ_ROOT=$(cd $(dirname $0) && pwd)
export GOPATH=$PROJ_ROOT
export PATH=$PATH:$GOPATH/bin
readonly build_ref=$(git rev-parse --short HEAD)

function gen_mod() {
        path=$1
        mod=$2

        pushd $path > /dev/null
#        rm -f go.mod
        if [ ! -e go.mod ]; then
                go mod init $mod
                go mod tidy
        fi
        popd > /dev/null
}

function build_lsp_deps() {
        echo [building deps]
        gen_mod ./src lspserver

        rm -f go.work
        if [ ! -e go.work ]; then
                go work init
                go work use src
                go work use vendor/go-lsp
        fi
        echo [done]
}

function build_lsp() {
        echo [building lsp]
        pushd src > /dev/null
        go build -o $PROJ_ROOT/$APP_NAME -ldflags "-X main.version=${build_ref} -X main.AppName=$APP_NAME" main.go
        popd > /dev/null
        echo [done]
}

function install_lsp() {
        echo [installing lsp]
        ln -fs $PROJ_ROOT/fuzz-lsp.sh $PROJ_ROOT/src
        echo [done]
}

function main() {
        build_lsp_deps
        build_lsp
        install_lsp
        echo [success]
}

main $@
