# SYNOPSIS

[![Go Reference](https://pkg.go.dev/badge/github.com/msantos/goreap.svg)](https://pkg.go.dev/github.com/msantos/goreap)

goreap [*options*] <command> <...>

# DESCRIPTION

Supervise and terminate subprocesses.

See [reap](https://github.com/leahneukirchen/reap).

# BUILDING

```
cd cmd/goreap
CGO_ENABLED=0 go build -trimpath -ldflags "-w"
```

# EXAMPLES

```
goreap cat

goreap sh -c "sleep inf & sleep inf & sleep 5"

$ goreap sh -c "sleep inf & sleep inf & pstree -pga $$; sleep 5"
bash,9062,9062
	└─goreap,31262,31262 sh -c ...
			├─sh,31267,31262 -c sleep inf & sleep inf & pstree -pga 9062; sleep 5
			│   ├─pstree,31270,31262 -pga 9062
			│   ├─sleep,31268,31262 inf
			│   └─sleep,31269,31262 inf
			├─{goreap},31263,31262
			├─{goreap},31264,31262
			├─{goreap},31265,31262
			├─{goreap},31266,31262
			├─{goreap},31271,31262
			└─{goreap},31272,31262
```

# OPTIONS

deadline
: send SIGKILL if processes running after deadline (0 to disable) (default 60s)

disable-setuid
: disallow setuid (unkillable) subprocesses

delay *duration*
: interval between signals (0 to disable) (default 1s)

signal *int*
: signal sent to supervised processes (default 15)

verbose
: debug output

wait
: wait for subprocesses to exit

# TESTS

```
bats test
```
