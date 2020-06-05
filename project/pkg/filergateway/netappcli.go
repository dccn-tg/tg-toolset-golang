package filergateway

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os/user"
	"path"
	"strings"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	log "github.com/Donders-Institute/tg-toolset-golang/pkg/logger"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
	"golang.org/x/crypto/ssh"
)

// NetAppCLI implements functions to manage project qtree on NetApp filer
// via the SSH-based management interface.  This is needed due to the
// unknown reason causing the FlexGroup not shown in the NetApp's REST API
// interface and therefore there is no way to manage project qtrees under
// the `project` FlexGroup.
//
// This is a temporary workaround until the REST API issue is resolved.
// Therefore it is implemented with few assumptions in order to keep it "just work".
//
// - volume quota policy called `default` is expected to exist in advance.
// - the `default` quota policy already contains a default rule for `tree` type.
// - password-less SSH authentication is expected.
//
type NetAppCLI struct {
	// Config is the configuration data structure for the NetAppCLI.
	Config config.NetAppCLIConfiguration
}

// CreateProjectQtree creates a new qtree and set its quota.
func (c *NetAppCLI) CreateProjectQtree(projectID string, data *pdb.DataProjectUpdate) error {

	// construct ontap command for creating a qtree.
	cmd := "volume qtree create -security-style unix -oplock-mode enable -unix-permissions 0700"
	cmd = fmt.Sprintf("%s -export-policy %s -vserver %s -volume %s -qtree %s",
		cmd, c.Config.ExportPolicy, c.Config.SVM, c.Config.ProjectVolume, projectID)

	log.Debugf("create qtree cmd: %s", cmd)

	// perform command via SSH
	stdout, stderr, err := c.execFilerMI(cmd)

	if err != nil {
		return err
	}
	log.Debugf("create qtree cmd stdout: %s", stdout)
	log.Warnf("create qtree cmd stderr: %s", stderr)

	// construct ontap command for creating quota policy rule for the qtree
	cmd = "volume quota policy rule create -policy-name default -type tree"
	cmd = fmt.Sprintf("%s -vserver %s -volume %s -target %s -disk-limit %dGB",
		cmd, c.Config.SVM, c.Config.ProjectVolume, projectID, data.Storage.QuotaGb)

	log.Debugf("update quota policy cmd: %s", cmd)

	// perform command via SSH
	stdout, stderr, err = c.execFilerMI(cmd)

	if err != nil {
		return err
	}
	log.Debugf("create quota policy stdout: %s", stdout)
	log.Warnf("create quota policy stderr: %s", stderr)

	return nil
}

// UpdateProjectQuota sets or updates the qtree quota for a projectID.
func (c *NetAppCLI) UpdateProjectQuota(projectID string, data *pdb.DataProjectUpdate) error {

	// construct ontap command for creating a qtree.
	cmd := "volume quota policy rule modify -policy-name default -type tree"
	cmd = fmt.Sprintf("%s -vserver %s -volume %s -target %s -disk-limit %dGB",
		cmd, c.Config.SVM, c.Config.ProjectVolume, projectID, data.Storage.QuotaGb)

	log.Debugf("update quota policy cmd: %s", cmd)

	// perform command via SSH
	stdout, stderr, err := c.execFilerMI(cmd)

	if err != nil {
		return err
	}
	log.Debugf("update quota policy stdout: %s", stdout)
	log.Warnf("update quota policy stderr: %s", stderr)

	return nil
}

// connect makes a SSH connection to the NetApp filer management interface and
// returns a ssh.Session.
func (c *NetAppCLI) connect() (session *ssh.Session, err error) {
	// get current user's SSH privat key $HOME/.ssh/id_rsa
	me, err := user.Current()
	if err != nil {
		return
	}

	key, err := ioutil.ReadFile(path.Join(me.HomeDir, ".ssh/id_rsa"))
	if err != nil {
		return
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return
	}

	config := &ssh.ClientConfig{
		User: "admin",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", c.Config.MgmtHost, config)
	if err != nil {
		return
	}

	session, err = client.NewSession()
	if err != nil {
		return
	}

	return
}

// execFilerMI executes given filer command on the management interface remotely via SSH.
// It returns stdout and stderr as slice of strings, each contains the data of a line.
func (c *NetAppCLI) execFilerMI(cmd string) (stdout []string, stderr []string, err error) {
	session, err := c.connect()
	if err != nil {
		return
	}
	defer session.Close()

	var outReader, errReader io.Reader
	outReader, err = session.StdoutPipe()
	if err != nil {
		return
	}
	outScanner := bufio.NewScanner(outReader)

	errReader, err = session.StdoutPipe()
	if err != nil {
		return
	}
	errScanner := bufio.NewScanner(errReader)

	if err = session.Run(cmd); err != nil {
		return
	}

	for outScanner.Scan() {
		stdout = append(stdout, strings.TrimSpace(outScanner.Text()))
	}
	if err = outScanner.Err(); err != nil {
		return
	}

	for errScanner.Scan() {
		stderr = append(stderr, strings.TrimSpace(errScanner.Text()))
	}
	if err = errScanner.Err(); err != nil {
		return
	}

	return

}
