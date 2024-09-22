package tunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alebeck/boring/internal/log"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	sshPort        = 22
	sshConnTimeout = 10 * time.Second
)

var defaultKeys = []string{"~/.ssh/id_rsa", "~/.ssh/id_ecdsa", "~/.ssh/id_ed25519"}

type runConfig struct {
	localAddress  string
	remoteAddress string
	hostName      string
	user          string
	port          int
	identityFile  string
	clientConfig  *ssh.ClientConfig
}

func (t *Tunnel) makeRunConfig() error {
	rc := runConfig{}

	// Fill in rc's values based on ssh config
	if err := parseSSHConf(t.Host, &rc); err != nil {
		return fmt.Errorf("could not parse SSH config: %v", err)
	}

	// Override values which were manually set by user
	if t.User != "" {
		rc.user = t.User
	}
	if t.Port != 0 {
		rc.port = t.Port
	}
	if t.IdentityFile != "" {
		rc.identityFile = t.IdentityFile
	}

	// If port is still not set, use default
	if rc.port == 0 {
		rc.port = sshPort
	}

	// If t.Host could not be resolved from ssh config, take it literally
	if rc.hostName == "" {
		rc.hostName = t.Host
	}

	if err := validate(&rc); err != nil {
		return err
	}

	// Make SSH client config
	var err error
	if rc.clientConfig, err = makeClientConfig(rc.user, rc.identityFile); err != nil {
		return fmt.Errorf("could not make client config: %v", err)
	}

	rc.remoteAddress = t.RemoteAddress
	rc.localAddress = t.LocalAddress
	if !strings.Contains(t.LocalAddress, ":") {
		rc.localAddress = "localhost:" + rc.localAddress
	}

	t.rc = &rc
	return nil
}

func parseSSHConf(alias string, rc *runConfig) error {
	// TODO check /etc/ssh/ssh_config if user one does not exist
	sshConfFile, err := os.Open(sshConfigPath("config"))
	if err != nil {
		return nil
	}
	sshConf, err := ssh_config.Decode(sshConfFile)
	if err != nil {
		return fmt.Errorf("could not decode ssh config: %v", err)
	}
	sshConfFile.Close()

	// TODO: allow multiple key files
	rc.identityFile, _ = sshConf.Get(alias, "IdentityFile")

	rc.user, _ = sshConf.Get(alias, "User")

	port, _ := sshConf.Get(alias, "Port")
	rc.port, _ = strconv.Atoi(port)

	rc.hostName, _ = sshConf.Get(alias, "HostName")

	return nil
}

func validate(rc *runConfig) error {
	if rc.hostName == "" {
		return fmt.Errorf("no host specified.")
	}
	if rc.user == "" {
		return fmt.Errorf("no user specified.")
	}
	if rc.port == 0 {
		return fmt.Errorf("no port specified.")
	}
	return nil
}

func makeClientConfig(user, identityFile string) (*ssh.ClientConfig, error) {
	var signers []ssh.Signer
	identityFiles := append([]string{identityFile}, defaultKeys...)

	for _, i := range identityFiles {
		key, err := os.ReadFile(fillHome(i))
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			log.Warningf("Unable to parse private key %v: %v", i, err)
			continue
		}
		signers = append(signers, signer)
	}

	if len(signers) == 0 {
		return nil, fmt.Errorf("no key files found.")
	}

	knownHostsCallback, err := knownhosts.New(sshConfigPath("known_hosts"))
	if err != nil {
		return nil, err
	}

	conf := ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: knownHostsCallback,
		Timeout:         sshConnTimeout,
	}
	return &conf, nil
}

func sshConfigPath(filename string) string {
	return filepath.Join(os.Getenv("HOME"), ".ssh", filename)
}

func fillHome(path string) string {
	home := os.Getenv("HOME")
	if path == "~" {
		return home
	} else if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}