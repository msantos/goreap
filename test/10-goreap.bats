#!/usr/bin/env bats

export PATH="$PWD:$PWD/cmd/goreap:$PATH"

@test "exit: subprocesses terminated" {
    run goreap bash -c "(while :; do (exec -a goreaptest sleep 120) & done) & sleep 2"
    [ "$status" -eq 0 ]
    run pgrep goreaptest
    [ "$status" -eq 1 ]
}

@test "signal: subprocesses terminated" {
    run timeout 1 goreap bash -c "(while :; do (exec -a goreaptest sleep 120) & done) & sleep 5"
    [ "$status" -eq 124 ]
    run pgrep goreaptest
    [ "$status" -eq 1 ]
}

@test "signal: subprocesses blocks SIGTERM" {
    run goreap --deadline=1s bash -c "trap '' TERM; (while :; do (exec -a goreaptest sleep 120) & done) & sleep 2"
    [ "$status" -eq 0 ]
    run pgrep goreaptest
    [ "$status" -eq 1 ]
}
