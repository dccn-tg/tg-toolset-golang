# repocli: cross-platform user CLI for managing the Donders Repository data

The CLI uses the WebDAV interface of the Donders Repository.

When performing an recursive operation over a directory, it performs a directory walk-through and applies the operation on individual files in parallel.  This approach breaks down a lengthy bulk-operation request into multiple shorter, less resource demanding requests.  It helps improve the overall success rate of the operation.

## Build binary

__Linux__

```bash
$ make build_repocli
```

The resulting binary is produced at `${GOPATH}/bin/repocli`.

__Windows__

```bash
$ make build_repocli_windows
```

The resulting binary is produced at `${GOPATH}/bin/repocli.exe`.

__MacOSX__

```bash
$ make build_repocli_macosx
```

The resulting binary is produced at `${GOPATH}/bin/repocli.darwin`.

## Usage

```
The user's CLI for managing data content of the Donders Repository collections.

Usage:
  repocli [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  get         Download a file or a directory from the repository to the current working directory at local
  help        Help about any command
  ls          List a file or the content of a directory in the repository
  mkdir       Create a new directory in the repository
  mv          Move or rename a file or directory in the repository
  put         Upload a file or a directory at local into a repository directory
  rm          Remove a file or a directory from the repository

Flags:
  -c, --config path       path of the configuration YAML file. (default "/home/honlee/.repocli.yml")
  -h, --help              help for repocli
  -n, --nthreads number   number of concurrent worker threads. (default 4)
  -p, --pass password     password of the repository data access account.
  -l, --url URL           URL of the webdav server. (default "https://webdav.data.donders.ru.nl")
  -u, --user username     username of the repository data access account.
  -v, --verbose           verbose output

Use "repocli [command] --help" for more information about a command.
```

One could provide the username/password using the configuration file (i.e. the `-c` flag) in YAML format.  The default location of this configuration file is `${HOME}/.repocli.yml` on Linux/MacOSX and `C:\Users\<username>\.repocli.yml` on Windows. Hereafter is an example:

```yaml
repository:
  username: "username"
  password: "password"
```
