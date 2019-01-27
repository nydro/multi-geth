#!/usr/bin/env bash

CGO_LDFLAGS="$GOPATH/src/github.com/etclabscore/sputnikvm-ffi/c/libsputnikvm.a -ldl -lm" go build -ldflags "-X core.UseSputnikVM=true" -o build/bin/geth ./cmd/geth

