package fakes

import "github.com/cloudfoundry/bosh-bootloader/gcp"

type GCPClientProvider struct {
	ClientCall struct {
		CallCount int
		Returns   struct {
			Client gcp.GCPClient
		}
	}
	SetConfigCall struct {
		CallCount int
		Receives  struct {
			ServiceAccountKey string
			ProjectID         string
			Region            string
			Zone              string
		}
		Returns struct {
			Error error
		}
	}
}

func (g *GCPClientProvider) Client() gcp.GCPClient {
	g.ClientCall.CallCount++

	return g.ClientCall.Returns.Client
}

func (g *GCPClientProvider) SetConfig(serviceAccountKey, projectID, region, zone string) error {
	g.SetConfigCall.CallCount++
	g.SetConfigCall.Receives.ServiceAccountKey = serviceAccountKey
	g.SetConfigCall.Receives.ProjectID = projectID
	g.SetConfigCall.Receives.Region = region
	g.SetConfigCall.Receives.Zone = zone

	return g.SetConfigCall.Returns.Error
}
