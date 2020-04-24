# TG toolset golang

CLI tools and re-usable libraries for interacting and managing various ICT services provided by the DCCN technical group, written in [Golang](https://golang.org).

## Code structure

Currently, the whole package is divided into two major sub-modules, each provides a set of CLI tools.  They are listed below:

- [dataflow](dataflow): tools and libraries concerning automatic dataflow from MEG/EEG labs.
  * [lab_bookings](dataflow/cmd/lab_bookings): a CLI for retrieving MEG events from the calendar system in order to fill the MEG console with relevant information for structure MEG raw data in the project storage.
  * [pacs_getstudies](dataflow/cmd/pacs_getstudies): a CLI for retrieving MRI studies from the Orthanc PACS server.
  * [pacs_streamdata](dataflow/cmd/pacs_streamdata): a CLI for (re-)streaming data from the Orthanc PACS server.
- [project](project): tools and libraries concerning managing project storage.
  * [prj_getacl](project/cmd/prj_getacl): a CLI for getting ACLs of a project storage and translating it to data-access roles (e.g. manager, contributor, viewer).
  * [prj_setacl](project/cmd/prj_setacl): a CLI for setting ACLs on a project storage to implement data-access roles.
  * [prj_delacl](project/cmd/prj_delacl): a CLI for deleting ACLs from a project storage to remove data-access roles.
  * [prj_mine](project/cmd/prj_mine): a CLI for retrieving the current user's data-access roles in all projects.
  * [project](project/cmd/project): a CLI for system administrator to perform actions such as provisioning storage resource or changing quota for projects.

Most of the re-usable libraries are written to implement the CLI tools listed above.  Those libraries are organised in various `pkg` directories:

- [pkg](pkg): common libraries shared between the sub-modules.
- [dataflow/pkg](dataflow/pkg): libraries used/introduced specifically for the development of the dataflow sub-module.
- [project/pkg](project/pkg): libraries used/introduced specifically for the development of the dataflow sub-module.

## Build

To build the CLIs, simply run:

```bash
make
```

After the build, the executable binaries are located in `${GOPATH}/bin` directory.  If `${GOPATH}` is not set in the environment, default is `${HOME}/go`.

## Run

All CLI commands have a `-h` option to print out a brief usage of the command.
