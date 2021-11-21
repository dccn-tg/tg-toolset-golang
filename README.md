# TG toolset golang

CLI tools and re-usable libraries for interacting and managing various ICT services provided by the DCCN technical group, written in [Golang](https://golang.org).

## Code structure

Currently, the whole package is divided into three major parts, each provides a set of CLI tools.  They are:

- [dataflow](dataflow) contains tools and libraries for automatic MEG/MRI dataflow.
  * [lab_bookings](dataflow/cmd/lab_bookings): a CLI for retrieving lab-booking events from the calendar system. It is utilized on the MEG console to provide information for structuring MEG raw data in the project storage.
  * [pacs_getstudies](dataflow/cmd/pacs_getstudies): a CLI for retrieving MRI studies from the Orthanc PACS server.
  * [pacs_streamdata](dataflow/cmd/pacs_streamdata): a CLI for (re-)streaming data from the Orthanc PACS server.
- [project](project) contains tools and libraries for project storage management.
  * [prj_getacl](project/cmd/prj_getacl): a CLI for getting ACLs of a project storage and translating it to data-access roles (e.g. manager, contributor, viewer).
  * [prj_setacl](project/cmd/prj_setacl): a CLI for setting ACLs on a project storage to implement data-access roles.
  * [prj_delacl](project/cmd/prj_delacl): a CLI for deleting ACLs from a project storage to remove data-access roles.
  * [prj_mine](project/cmd/prj_mine): a CLI for retrieving the current user's data-access roles in all projects.
  * [pdbutil](project/cmd/pdbutil): a project database utility for performing actions such as provisioning storage resource or changing storage quota of project.
- [repository](repository) contains tools and libraries for repository data management.
  * [repoadm](repository/cmd/repoadm): administrator's CLI for managing the Donders Repository collections, using the iROD's iCommands.
  * [repocli](repository/cmd/repocli): cross-platform user CLI for managing data in the Donders Repository, using the WebDAV interface.
- [metrics](metrics) contains tools and libraries for collecting metrics.
  * [hpc-metrics-appusage-collector](metrics/cmd/hpc-metrics-appusage-collector): a daemon collects count on `module load` on HPC into Prometheus metrics. See [README](metrics/cmd/hpc-metrics-appusage-collector/README.md) for more detail.
 
Various CLIs take a YAML-based configuration file (via the `-c` option) for setting up connections to, e.g., project database, filers, etc.. An example YAML file is provided [here](configs/config.yml); and the codes that "objectize" the YAML file are located in the [pkg/config](pkg/config) directory.

Most of the re-usable libraries are written to support the CLI tools listed above.  Those libraries are organised in various `pkg` directories:

- [pkg](pkg): common libraries shared between the sub-modules.
- [dataflow/pkg](dataflow/pkg): libraries for the CLI tools of the MEG/MRI dataflow management.
- [project/pkg](project/pkg): libraries for the CLI tools of the project management.
- [repository/pkg](repository/pkg): libraries for the CLI tools of the repository data management.

## Build

To build the CLIs, simply run:

```bash
make
```

After the build, the executable binaries are located in `${GOPATH}/bin` directory.  If `${GOPATH}` is not set in the environment, default is `${HOME}/go`.

## Release

The [Makefile](Makefile) has a target to build a GitHub release with an RPM package as the release asset.  For making a GitHub release `X.Y.Z`, one does: 

```bash
VERSION=X.Y.Z make github-release
```

The [GitHub personal access token](https://help.github.com/en/github/authenticating-to-github/creating-a-personal-access-token-for-the-command-line) is required for interacting with the GitHub APIs.

## Run

All CLI commands have a `-h` option to print out a brief usage of the command.
