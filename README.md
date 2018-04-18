# Tools for managing project data access

This package is rewriting the existing python scripts of `prj_getacl`, `prj_setacl`, and `prj_delacl`, with the following objectives:

* supporting role setting and deletion on individual file/directory level so that there will be no permission "overwriting" issue when managing different access roles on sub-directories in a project storage.

  In this context, the program should be fast enought to walk through all the files in a project (in an order of 10^7 - 10^9). This requirement drives the idea of writing the program with the Go language given that the concurrency model in the language can potentially be used to boost the speed of the massive setacl and getacl operations.

* supporting intellegent traverse role setting while following a link referring to a path that is outside the current project storage.

  Traverse role setting becomes non-trivial when the following symbolic link happens:
  
  ```bash
  /project/3010000.01/symlink --> /project_ext/3010000.04/referent
  ```
  
  In this situation, the traverse role setting should be applied not only on path `/project/3010000.01`, but also `/project_ext/3010000.04`.  Also note that the two root directories (`/project` and `/project_ext`) are mount points to two different storage systems.  It also requires the program to switch between two ACL-setting implementations that fit well with the logic exposed by the storage systems.
  
* facilitating possibility of extending the code for different storage system supporting NFSv4 ACL.

  This requires the code to be modulized and well documented so that future developers can easily extend the code for other different systems.  It is another reason to choose the Go language for the implementation.

## Build

```bash
make
```

After the build, the executable binaries are located in the `bin` directory.
