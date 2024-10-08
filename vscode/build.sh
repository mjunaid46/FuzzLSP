#!/bin/bash

pushd extension
    npm install
    npm run compile
popd
