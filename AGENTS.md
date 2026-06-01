# container-compose — Agent Guide

## Overview

`container-compose` is a lightweight Go tool that reads a `docker-compose.yml` file and manages containers using a macOS-native `container` runtime (Apple's container platform). It implements a subset of Docker Compose semantics — enough to start, stop, and check the status of a set of services defined in a YAML file.

## Project structure

```
container-compose/
├── main.go               # Single-file Go application
├── docker-compose.yml    # Service definitions (consumed at runtime)
├── AGENTS.md             # This file
├── README.md             # User-facing documentation
├── go.mod / go.sum       # Go module definition (depends on gopkg.in/yaml.v3)
├── .buildkite/           # Buildkite CI configuration
├── .github/workflows/    # GitHub Actions CI configuration (build + release)
└── tmp/                  # Local working directory (gitignored)
```

## Key design decisions

- **Single-file application** — Everything lives in `main.go`. There is no framework, no sub-packages, and no external DI. Keep it this way unless complexity justifies extracting a package.
- **No Docker dependency** — The tool shells out to a `container` binary (Apple's container runtime), not Docker. The JSON output format of `container inspect` is specific to this runtime.
- **YAML-driven** — Reads `docker-compose.yml` from the current working directory at startup. Use `-f`/`--file` flag for an alternate path.

## `container inspect` JSON format

The Apple `container` tool returns inspect data in this shape:

```json
[{
  "configuration": {
    "image": {
      "reference": "docker.io/library/redis:7-alpine"
    }
  },
  "status": "running",
  "networks": [{ ... }]
}]
```

Key fields used by the code:
- `status` — top-level string: `"running"` or empty/absent when stopped.
- `configuration.image.reference` — the full image reference used to create the container.
- `networks` — array of network attachment info.

## Supported `docker-compose.yml` fields

Only the following fields are consumed (others are silently ignored):

| YAML field | Go field | Notes |
|---|---|---|
| `services.<name>.image` | `Service.Image` | Required. Full image reference. |
| `services.<name>.platform` | `Service.Platform` | `linux/amd64` → `--arch amd64` |
| `services.<name>.environment` | `Service.Environment` | Passed as `--env` flags. Accepts array (`KEY=VALUE`) or hash (`KEY: VALUE`) form. |
| `services.<name>.volumes` | `Service.Volumes` | Relative paths resolved with `os.Getwd()`, `./` prefix trimmed |
| `services.<name>.ports` | `Service.Ports` | Passed as `-p` flags, supports `[host-ip:]host-port:container-port[/protocol]` |
| `services.<name>.command` | `Service.Command` | Appended as positional args after the image on `container run` |
| `services.<name>.entrypoint` | `Service.Entrypoint` | Passed as `--entrypoint` flag |
| `services.<name>.depends_on` | `Service.DependsOn` | Used for startup ordering (topological sort) |
| `services.<name>.deploy.resources.limits.memory` | `Service.Deploy.Resources.Limits.Memory` | Passed as `--memory` |

**Not supported** (present in the YAML but ignored): `working_dir`, `deploy` (other than memory), `restart`, `networks`, `healthcheck`, etc.

## Commands

- **`container-compose start`** — For each service: inspect the container. If it exists and is running, skip. If it exists but is stopped, start it. If it doesn't exist, create and run it. Compares the container's image tag against `docker-compose.yml` and warns on mismatch. Repeats all warnings at the end.
- **`container-compose status`** (or `ps`, `ls`) — Inspects each service and prints its status (or error if container doesn't exist).
- **`container-compose stop`** — Stops each service's container.
- **`container-compose logs`** — Prints or streams logs from all service containers. Supports `-f`/`--follow` and `-n N`.

## Image version alerts

When starting, the tool compares the tag of the image the container was created with against the tag in `docker-compose.yml`. This uses a simple "last colon" split — no registry resolution, no digest comparison. If the tags differ, a warning is printed immediately and repeated in a summary section at the end.

## `container` runtime notes

- The binary is `container` (must be on `$PATH`). System must be started with `container system start`.
- Containers are created with `--detach --rm`. They are auto-removed on stop.
- The `--dns-domain test` flag is hard-coded on new containers.
- Container names are derived from the working directory name plus the service name (e.g. `myproject-db`) to avoid clashes between projects.

## Unfinished / TODO

- `restart` policy is parsed in YAML but not passed to `container run` (the macOS `container` runtime does not support restart policies).
