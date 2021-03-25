#!/usr/bin/env bats

export PATH="$PATH:$PWD:$PWD/test/src"

@test "exit: subprocesses terminated" {
    run goreap worm 0
    [ "$status" -eq 0 ]
    run pgrep worm
    [ "$status" -eq 1 ]
}

@test "signal: subprocesses terminated" {
    run timeout 1 goreap worm 120 
    [ "$status" -eq 124 ]
    run pgrep worm
    [ "$status" -eq 1 ]
}
