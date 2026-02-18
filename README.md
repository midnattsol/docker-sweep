# docker-sweep

Interactive cleanup for Docker resources with a picker UI.

`docker-sweep` runs as a Docker CLI plugin (`docker sweep`) and helps remove:

- stopped/old containers
- dangling/unused images
- unused volumes
- unused networks

Resources labeled `sweep.protect=true` are never auto-deleted.

## Install

Build and install as Docker CLI plugin:

```bash
make install
```

This installs to:

```text
~/.docker/cli-plugins/docker-sweep
```

Then run:

```bash
docker sweep
```

## Usage

Interactive mode:

```bash
docker sweep
```

In the picker:

- press `d` to toggle dangling images visibility without restarting
- after deleting, the picker stays open so you can continue cleaning
- exit explicitly with `q` or `Ctrl+C`

Delete suggested resources without interaction:

```bash
docker sweep --yes
```

Dry run:

```bash
docker sweep --dry-run
```

Garbage-collect mode (non-interactive):

```bash
docker sweep --gc
```

### Scope Flags

Filter which resource types are analyzed from the root command:

- `-c`, `--containers`
- `-i`, `--images`
- `-n`, `--networks`
- `-v`, `--volumes`

Examples:

```bash
docker sweep -i --dry-run
docker sweep -i --no-dangling --dry-run
docker sweep --gc --dry-run
docker sweep -c -n --dry-run
docker sweep -v --yes
```

Version:

```bash
docker sweep -V
docker sweep --version
```

## Type-Specific Filters

- `--exited` applies to containers
- `--min-size`, `--dangling`, `--no-dangling` apply to images
- `--anonymous` applies to volumes
- `--older-than` applies to all supported resource types

By default, dangling images are excluded unless you pass `--dangling`.

`--dangling` and `--no-dangling` are mutually exclusive.
`--gc` is mutually exclusive with both `--dangling` and `--no-dangling`.

If a type-specific filter is used without its resource scope, `docker-sweep` returns a clear validation error.

## Subcommands

You can also run per-type commands:

```bash
docker sweep containers
docker sweep images
docker sweep volumes
docker sweep networks
docker sweep update --check
```

## Podman

Podman does not support Docker-style generic CLI plugins (`podman <plugin>`). So `podman sweep` is not discovered like Docker plugins.

`docker-sweep` supports Podman runtime selection:

```bash
DOCKER_SWEEP_RUNTIME=podman docker sweep
```

Runtime selection priority is:

1. `DOCKER_SWEEP_RUNTIME` (`docker` or `podman`)
2. binary name contains `podman` (for standalone usage)
3. auto-detect (`docker` first, then `podman`)

Standalone usage with symlink:

```bash
ln -sf ~/code/docker-sweep/docker-sweep ~/bin/podman-sweep
podman-sweep --dry-run
```

Or shell wrapper for `podman sweep` style command:

```bash
podman() {
  if [ "$1" = "sweep" ]; then
    shift
    DOCKER_SWEEP_RUNTIME=podman docker sweep "$@"
  else
    command podman "$@"
  fi
}
```

## Protection Label

Protect any resource from deletion:

```bash
--label sweep.protect=true
```

Compose project labels are detected and shown in the picker when present.
