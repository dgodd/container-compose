# container-compose

A lightweight alternative to `docker-compose` for macOS that manages containers using Apple's native `container` runtime instead of Docker.

## Prerequisites

- macOS with the [Apple `container` runtime](https://github.com/apple/container-oss) installed and available on `$PATH`
- The container system must be started:

  ```sh
  container system start
  ```

- A `docker-compose.yml` file in the current working directory

## Installation

```sh
git clone https://github.com/dgodd/container-compose
cd container-compose
go build -o container-compose .
# optional: place on your PATH
```

## Usage

All commands read `docker-compose.yml` from the current directory.

### `container-compose start`

Creates and starts all services defined in `docker-compose.yml`. For each service:

- If a container with that name already exists and is **running**, it is left alone.
- If a container exists but is **stopped**, it is started with `container start`.
- If no container exists, one is created and started with `container run`.

If the image tag of an existing container differs from what's specified in `docker-compose.yml`, a warning is printed immediately and repeated in a summary at the end.

```sh
container-compose start
```

### `container-compose status`

Prints the status of each service's container (e.g., `running`, or an error if the container does not exist).

```sh
container-compose status
```

### `container-compose stop`

Stops all running service containers.

```sh
container-compose stop
```

## Supported `docker-compose.yml` fields

| Field | Example | Behavior |
|---|---|---|
| `services.<name>.image` | `redis:7-alpine` | Required. Full image reference. |
| `services.<name>.platform` | `linux/amd64` | Translated to `--arch amd64` |
| `services.<name>.environment` | `- FOO=bar` | Passed as `--env` flags |
| `services.<name>.volumes` | `- ./data:/data` | Relative paths resolved from CWD |
| `services.<name>.deploy.resources.limits.memory` | `2G` | Passed as `--memory` |

Fields like `ports`, `command`, `entrypoint`, `working_dir`, `restart`, `depends_on`, and `networks` are parsed from YAML but silently ignored at runtime.

## How it works

The tool shells out to Apple's `container` binary with commands like:

```sh
container inspect <name>
container run --detach --name <name> <image>
container start <name>
container stop <name>
```

Containers are created with `--rm` so they are automatically removed when stopped.

## License

See [LICENSE](LICENSE).
