package actors

import (
	"fmt"
	"os/exec"
)

type BOSHCLI struct{}

func NewBOSHCLI() BOSHCLI {
	return BOSHCLI{}
}

func (BOSHCLI) DirectorExists(address, caCertPath string) (bool, error) {
	_, err := exec.Command("bosh2",
		"--ca-cert", caCertPath,
		"-e", address,
		"env",
	).Output()

	return err == nil, err
}

func (BOSHCLI) Env(address, caCertPath string) (string, error) {
	env, err := exec.Command("bosh2",
		"--ca-cert", caCertPath,
		"-e", address,
		"env",
	).Output()

	return string(env), err
}

func (BOSHCLI) CloudConfig(address, caCertPath, username, password string) (string, error) {
	cloudConfig, err := exec.Command("bosh2",
		"--ca-cert", caCertPath,
		"--client", username,
		"--client-secret", password,
		"-e", address,
		"cloud-config",
	).Output()

	return string(cloudConfig), err
}

func (BOSHCLI) DeleteEnv(stateFilePath, manifestPath string) error {
	_, err := exec.Command(
		"bosh2",
		"delete-env",
		fmt.Sprintf("--state=%s", stateFilePath),
		manifestPath,
	).Output()

	return err
}
