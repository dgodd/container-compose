# Container Compose

Run containers using apple's new "container" command and "docker-compose.yml"

NOTE: This is currently very much a proof of concept to see if the idea is
interesting.

## Install

1. First install Apple's "container" using [Container
Tutoral](https://github.com/apple/container/blob/main/docs/tutorial.md), you
will need to setup dns using "test" as a domain (as suggested in the doc).
Currently that means running `container system start` and `sudo container system dns create test`.
3. Copy the latest release of "container-compose" to the path
4. Run "container-compose" from a directory with a "docker-compose.yml" file.

## Build

```
go build -trimpath -buildvcs -o ./container-compose .
```

