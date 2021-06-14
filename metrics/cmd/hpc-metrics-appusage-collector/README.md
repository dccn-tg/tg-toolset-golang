# HPC application usage metrics

The program `hpc-metrics-appusage-collector` is a daemon for monitoring the HPC software usage by collecting data instrumented in the environment modules on the HPC cluster.

This program contains two parts:

- _Collector_ service listens on incoming data sent from the `module load` command. The data is as simple as the string containing the module name and the version.  See the `send_usage` function in the `/opt/_modules/share/common.tcl` file on the HPC cluster.

- _Metrics pusher_ sends POST request to a OpenTSDB endpoint specified by `-l` option every given period of time (default: 10 seconds).

## Build Docker image

1. Go to the root directory of this repository

1. build the image

    Assuming that we want to build the image in a docker registry `${DOCKER_REGISTRY}` with image name `hpc-metrics-appusage-collector` and version `${VERSION}`:

    ```bash
    $ docker build -f build/docker/hpc-metrics-appusage-collector/Dockerfile \
      --force-rm -t ${DOCKER_REGISTRY}/hpc-metrics-appusage-collector:${VERSION} .
    ```

1. push the image to `${DOCKER_REGISTRY}`

    ```bash
    $ docker push ${DOCKER_REGISTRY}/hpc-metrics-appusage-collector:${VERSION}
    ```

## Run container with docker-compose

```
version: '3.7'

services:

  hpc-metrics-appusage-collector:
    hostname: hpc-metrics-appusage-collector
    image: ${DOCKER_REGISTRY}/hpc-metrics-appusage-collector:${VERSION}
    ports:
      - 9999:9999/udp
    command: ["-p", "180", "-l", "http://opentsdb:4242/api/put"]
```
