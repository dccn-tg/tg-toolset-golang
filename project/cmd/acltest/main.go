package main

import (
	acl "github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"
)

func main() {
	acl.PosixGetfacl("/project_cephfs/3055010.01")
}
