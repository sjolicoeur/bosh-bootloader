package commands

type AzureUpConfig struct {
	SubscriptionID string
	TenantID       string
	ClientID       string
	ClientSecret   string
}

type NewAzureUpArgs struct {
	Logger logger
}

type AzureUp struct {
	Logger logger
}

func NewAzureUp(upArgs NewAzureUpArgs) AzureUp {
	return AzureUp{
		Logger: upArgs.Logger,
	}
}

func (u AzureUp) Execute(upConfig AzureUpConfig) error {
	u.Logger.Step("verifying credentials")
	return nil
}
