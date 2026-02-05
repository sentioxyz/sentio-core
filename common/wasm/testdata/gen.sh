#!/bin/sh

cd $(dirname $0)

rm -rf build generated && yarn install && yarn codegen && yarn build
