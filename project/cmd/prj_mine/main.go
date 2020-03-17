package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/acl"

	log "github.com/sirupsen/logrus"
)

func main() {

	nworkers := 4

	uid := os.Args[1]

	dirs := make(chan string, nworkers*2)
	members := make(chan projectRole)

	wg := sync.WaitGroup{}
	for i := 0; i < nworkers; i++ {
		wg.Add(1)
		go findUserMember(uid, dirs, members, &wg)
	}

	// go routine to list all directories in the /project folder
	go func(path string) {
		// close the dirs channel on exit
		defer close(dirs)

		infoDirs, err := ioutil.ReadDir(path)
		if err != nil {
			log.Errorf("cannot get content of path: %s", path)
			return
		}

		for _, infoDir := range infoDirs {
			dirs <- filepath.Join(path, infoDir.Name())
		}

	}("/project")

	// go routine printing user's membership.
	go func() {
		for member := range members {
			fmt.Printf("%+v\n", member)
		}
	}()

	wg.Wait()

	// close up members channel
	close(members)
}

type projectRole struct {
	projectID string
	role      acl.Role
}

func findUserMember(uid string, dirs chan string, members chan projectRole, wg *sync.WaitGroup) {

	for dir := range dirs {

		log.Debugf("finding user member for %s in %s", uid, dir)

		// get all members of the dir
		runner := acl.Runner{
			RootPath:   dir,
			FollowLink: true,
			SkipFiles:  true,
			Nthreads:   1,
		}

		chanOut, err := runner.GetRoles(false)
		if err != nil {
			log.Errorf("cannot get role for path %s: %s", dir, err)
			return
		}

		// feed members channel if the user in question is in the list.
		for o := range chanOut {
			for r, users := range o.RoleMap {
				if r == acl.System {
					continue
				}
				for _, u := range users {
					if u == uid {
						members <- projectRole{
							projectID: filepath.Base(dir),
							role:      r,
						}
						break
					}
					continue
				}
			}
		}
	}

	wg.Done()
}
