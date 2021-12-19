# repocli: cross-platform user CLI for managing the Donders Repository data

A command-line tool for performing basic operations on the Donders Repository data.  The tool uses the WebDAV protocol, therefore it is also a genetic tool for managing data that are accessible via an WebDAV interface.

The implemented operations are:

- ls: list a directory
- mkdir: create a new directory
- mv: rename a file or a directory
- rm: remove a file or a directory
- get: download a file or a directory
- put: upload a file or a directory

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
A user's CLI for managing data content of the Donders Repository collections.

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
  -c, --config path       path of the configuration YAML file. (default "$HOME/.repocli.yml")
  -h, --help              help for repocli
  -n, --nthreads number   number of concurrent worker threads. (default 4)
  -s, --silent            set to slient mode (i.e. do not show progress)
  -l, --url URL           URL of the webdav server. (default "https://webdav.data.donders.ru.nl")
  -v, --verbose           verbose output

Use "repocli [command] --help" for more information about a command.
```

The username/password of the data-access account should be provided in the configuration file (i.e. the `-c` flag) in YAML format.  The default location of this configuration file is `${HOME}/.repocli.yml` on Linux/MacOSX and `C:\Users\<username>\.repocli.yml` on Windows. Hereafter is an example:

```yaml
repository:
  username: "username"
  password: "password"
```

At the moment, the configuration is in plain text.  Therefore, it is highly recommended to make the configuration file only accessible for the current user on the client. On Linux and MacOSX, one can run the following command in a terminal:

```bash
$ chmod 600 $HOME/.repocli.yml
```

### listing a collection

Given a collection with identifier `di.dccn.DAC_3010000.01_173`, the following command lists the content of it: 

```bash
$ repocli ls /dccn/DAC_3010000.01_173
/dccn/DAC_3010000.01_173:
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/Cropped
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/raw
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/test1
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/test2021
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/test3
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/test_loc.new
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/test_sync
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/testx
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/xyz.5
 drwxrwxr-x            0 /dccn/DAC_3010000.01_173/xyz.x
 -rw-rw-r--          203 /dccn/DAC_3010000.01_173/MANIFEST.txt.1
 -rw-rw-r--       191503 /dccn/DAC_3010000.01_173/MD5E-s191503--8661ce04ccbbf51e96ce124e30fc0c8c.txt
 -rw-rw-r--     49152352 /dccn/DAC_3010000.01_173/MP2RAGE.nii
 -rw-rw-r--         2589 /dccn/DAC_3010000.01_173/Makefile
...
```

### removing a file or sub-directory (sub-collection)

Assuming that we want to remove the file `MANIFEST.txt.1` from the collection content listed above, we do

```bash
$ repocli rm /dccn/DAC_3010000.01_173/MANIFEST.txt.1
```

If we want to remove the entire sub-directory `testx`, we use the command

```bash
$ repocli rm -r /dccn/DAC_3010000.01_173/textx
```

where the extra flag `-r` indicates recursive removal.

### creating sub-directory in the collection

To create a subdirectory `demo` in the collection, we do

```bash
$ repocli mkdir /dccn/DAC_3010000.01_173/demo
```

One could also create a directory tree use the same command, any missing parent directories will also be created (similar to the `mkdir -p` command on Linux).  For example, if we want to create a directory tree `demo1/data/sub-001/ses-mri01`, we do

```bash
$ repocli mkdir /dccn/DAC_3010000.01_173/demo1/data/sub-001/ses-mri01
```

It can be done with or without the existence of the parent tree structure `demo1/data/sub-001`.

### uploading/download single file to/from the collection

For uploading/downloading a single file to/from the collection in the repository.  One use the `put` and `get` sub-commands, respectively.  The `put` and `get` sub-arguments require two arguments.  The first argument refers to the _source_, while the second refers to the _destination_. 

For example, to upload a local file `test.txt` to `/project/3010000.01/demo`, one does

```bash
$ repocli put test.txt /project/3010000.01/demo/test.txt
```

To download a remote file `/project/3010000.01/demo/test.txt` to `test.txt.new` at local, one does

```bash
$ repocli get /project/3010000.01/demo/test.txt test.txt.new
```

If the destination is a directory, file will be downloaded/uploaded into the directory with the same name.  If the destination is an existing file, the file will be overwritten by the content of the source.

### resursive uploading/downloading to/from the collection

Assuming that we have a local directory `/project/3010000.01/demo`, and we want to upload the content of it recursively to the collection under the sub-directory `demo`.  We use the command below:

```bash
$ repocli put /project/3010000.01/demo/ /dccn/DAC_3010000.01_173/demo
```

where the first argument to `put` is a directory locally as the source, and the second is a directory in the repository as the destination.

For downloading a collection (or a sub-directory) from the repository, one does

```bash
$ repocli get /dccn/DAC_3010000.01_173/demo/ /project/3010000.01/demo.new
```

where the first argument is a directory in the repository as the source, and the second is a local directory as the destination.

__Note:__ The same as the `rsync` command, the tailing `/` in the first argument (i.e. the source) will causes the program to _copy the content_ into the destination.  If the tailing `/` is not given, it will _copy the directory by name_ in to the destination, resulting in the content being put into a (new) sub-directory in the destination.

### moving/renaming file/directory in a collection

For renaming a file within a collection, one uses the `mv` sub-command.

For example, if we want to rename a file `/dccn/DAC_3010000.01_173/test.txt` to `/dccn/DAC_3010000.01_173/test.txt.old` in the repository, we do

```bash
$ repocli mv /dccn/DAC_3010000.01_173/test.txt /dccn/DAC_3010000.01_173/test.txt.old
```

We could also rename an entire directory.  For example, if we want to rename a `/dccn/DAC_3010000.01_173/demo` to `/dccn/DAC_3010000.01_173/demo.new`, we use the command

```bash
$ repocli mv /dccn/DAC_3010000.01_173/demo /dccn/DAC_3010000.01_173/demo.new
```
