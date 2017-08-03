package commands

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-bootloader/flags"
	"github.com/cloudfoundry/bosh-bootloader/storage"
)

type Up struct {
	awsUp       awsUp
	gcpUp       gcpUp
	azureUp     azureUp
	envGetter   envGetter
	boshManager boshManager
}

type awsUp interface {
	Execute(awsUpConfig AWSUpConfig, state storage.State) error
}

//go:generate counterfeiter -o ./fakes/gcp_up.go --fake-name GCPUp . gcpUp
type gcpUp interface {
	Execute(gcpUpConfig GCPUpConfig, state storage.State) error
}

//go:generate counterfeiter -o ./fakes/azure_up.go --fake-name AzureUp . azureUp
type azureUp interface {
	Execute(azureUpConfig AzureUpConfig) error
}

type envGetter interface {
	Get(name string) string
}

type upConfig struct {
	awsAccessKeyID       string
	awsSecretAccessKey   string
	awsRegion            string
	awsBOSHAZ            string
	gcpServiceAccountKey string
	gcpProjectID         string
	gcpZone              string
	gcpRegion            string
	azureClientSecret    string
	azureTenantID        string
	azureClientID        string
	azureSubscriptionID  string
	iaas                 string
	name                 string
	opsFile              string
	noDirector           bool
	jumpbox              bool
}

func NewUp(awsUp awsUp, gcpUp gcpUp, azureUp azureUp, envGetter envGetter, boshManager boshManager) Up {
	return Up{
		awsUp:       awsUp,
		gcpUp:       gcpUp,
		azureUp:     azureUp,
		envGetter:   envGetter,
		boshManager: boshManager,
	}
}

func (u Up) CheckFastFails(args []string, state storage.State) error {
	config, err := u.parseArgs(args)
	if err != nil {
		return err
	}

	if !config.noDirector && !state.NoDirector {
		err = fastFailBOSHVersion(u.boshManager)
		if err != nil {
			return err
		}
	}

	if state.IAAS == "" && config.iaas == "" {
		return errors.New("--iaas [gcp, aws] must be provided or BBL_IAAS must be set")
	}

	if state.IAAS != "" && config.iaas != "" && state.IAAS != config.iaas {
		return fmt.Errorf("The iaas type cannot be changed for an existing environment. The current iaas type is %s.", state.IAAS)
	}

	if state.EnvID != "" && config.name != "" && config.name != state.EnvID {
		return fmt.Errorf("The director name cannot be changed for an existing environment. Current name is %s.", state.EnvID)
	}

	return nil
}

func (u Up) Execute(args []string, state storage.State) error {
	var desiredIAAS string

	config, err := u.parseArgs(args)
	if err != nil {
		return err
	}

	if state.IAAS != "" {
		desiredIAAS = state.IAAS
	} else {
		desiredIAAS = config.iaas
	}

	switch desiredIAAS {
	case "aws":
		err = u.awsUp.Execute(AWSUpConfig{
			AccessKeyID:     config.awsAccessKeyID,
			SecretAccessKey: config.awsSecretAccessKey,
			Region:          config.awsRegion,
			BOSHAZ:          config.awsBOSHAZ,
			OpsFilePath:     config.opsFile,
			Name:            config.name,
			NoDirector:      config.noDirector,
		}, state)
	case "gcp":
		err = u.gcpUp.Execute(GCPUpConfig{
			ServiceAccountKey: config.gcpServiceAccountKey,
			ProjectID:         config.gcpProjectID,
			Zone:              config.gcpZone,
			Region:            config.gcpRegion,
			OpsFilePath:       config.opsFile,
			Name:              config.name,
			NoDirector:        config.noDirector,
			Jumpbox:           config.jumpbox,
		}, state)
	case "azure":
		err = u.azureUp.Execute(AzureUpConfig{
			SubscriptionID: config.azureSubscriptionID,
			TenantID:       config.azureTenantID,
			ClientID:       config.azureClientID,
			ClientSecret:   config.azureClientSecret,
		})
		if err != nil {
			panic(err)
		}

	default:
		return fmt.Errorf("%q is an invalid iaas type, supported values are: [gcp, aws]", desiredIAAS)
	}

	if err != nil {
		return err
	}

	return nil
}

func (u Up) parseArgs(args []string) (upConfig, error) {
	var config upConfig

	upFlags := flags.New("up")

	upFlags.String(&config.iaas, "iaas", u.envGetter.Get("BBL_IAAS"))

	upFlags.String(&config.awsAccessKeyID, "aws-access-key-id", u.envGetter.Get("BBL_AWS_ACCESS_KEY_ID"))
	upFlags.String(&config.awsSecretAccessKey, "aws-secret-access-key", u.envGetter.Get("BBL_AWS_SECRET_ACCESS_KEY"))
	upFlags.String(&config.awsRegion, "aws-region", u.envGetter.Get("BBL_AWS_REGION"))
	upFlags.String(&config.awsBOSHAZ, "aws-bosh-az", u.envGetter.Get("BBL_AWS_BOSH_AZ"))

	upFlags.String(&config.gcpServiceAccountKey, "gcp-service-account-key", u.envGetter.Get("BBL_GCP_SERVICE_ACCOUNT_KEY"))
	upFlags.String(&config.gcpProjectID, "gcp-project-id", u.envGetter.Get("BBL_GCP_PROJECT_ID"))
	upFlags.String(&config.gcpZone, "gcp-zone", u.envGetter.Get("BBL_GCP_ZONE"))
	upFlags.String(&config.gcpRegion, "gcp-region", u.envGetter.Get("BBL_GCP_REGION"))

	upFlags.String(&config.azureSubscriptionID, "azure-subscription-id", "")
	upFlags.String(&config.azureTenantID, "azure-tenant-id", "")
	upFlags.String(&config.azureClientID, "azure-client-id", "")
	upFlags.String(&config.azureClientSecret, "azure-client-secret", "")

	upFlags.String(&config.name, "name", "")
	upFlags.String(&config.opsFile, "ops-file", "")
	upFlags.Bool(&config.noDirector, "", "no-director", false)
	upFlags.Bool(&config.jumpbox, "", "jumpbox", false)

	err := upFlags.Parse(args)
	if err != nil {
		return upConfig{}, err
	}

	return config, nil
}
