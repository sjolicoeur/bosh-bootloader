package commands_test

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/bosh-bootloader/bosh"
	"github.com/cloudfoundry/bosh-bootloader/commands"
	commandsFakes "github.com/cloudfoundry/bosh-bootloader/commands/fakes"
	"github.com/cloudfoundry/bosh-bootloader/fakes"
	"github.com/cloudfoundry/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Up", func() {
	var (
		command commands.Up

		fakeAWSUp       *fakes.AWSUp
		fakeAzureUp     *commandsFakes.AzureUp
		fakeGCPUp       *commandsFakes.GCPUp
		fakeEnvGetter   *fakes.EnvGetter
		fakeBOSHManager *fakes.BOSHManager
		state           storage.State
	)

	BeforeEach(func() {
		fakeAWSUp = &fakes.AWSUp{Name: "aws"}
		fakeGCPUp = &commandsFakes.GCPUp{}
		fakeAzureUp = &commandsFakes.AzureUp{}
		// fakeAzureUp = &commandsFakes.AzureUp{Name: "azure"}
		fakeEnvGetter = &fakes.EnvGetter{}
		fakeBOSHManager = &fakes.BOSHManager{}
		fakeBOSHManager.VersionCall.Returns.Version = "2.0.24"

		command = commands.NewUp(fakeAWSUp, fakeGCPUp, fakeAzureUp, fakeEnvGetter, fakeBOSHManager)
	})

	Describe("CheckFastFails", func() {
		Context("when the version of BOSH is a dev build", func() {
			It("does not fail", func() {
				fakeBOSHManager.VersionCall.Returns.Error = bosh.NewBOSHVersionError(errors.New("BOSH version could not be parsed"))

				err := command.CheckFastFails([]string{
					"--iaas", "aws",
				}, storage.State{Version: 999})

				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the version of BOSH is lower than 2.0.24", func() {
			It("returns a helpful error message when bbling up with a director", func() {
				fakeBOSHManager.VersionCall.Returns.Version = "1.9.1"
				err := command.CheckFastFails([]string{
					"--iaas", "aws",
				}, storage.State{Version: 999})

				Expect(err).To(MatchError("BOSH version must be at least v2.0.24"))
			})

			Context("when the no-director flag is specified", func() {
				It("does not return an error", func() {
					fakeBOSHManager.VersionCall.Returns.Version = "1.9.1"
					err := command.CheckFastFails([]string{
						"--iaas", "aws",
						"--no-director",
					}, storage.State{Version: 999})

					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("when the version of BOSH cannot be retrieved", func() {
			It("returns an error", func() {
				fakeBOSHManager.VersionCall.Returns.Error = errors.New("BOOM")
				err := command.CheckFastFails([]string{
					"--iaas", "aws",
				}, storage.State{Version: 999})

				Expect(err.Error()).To(ContainSubstring("BOOM"))
			})
		})

		Context("when the version of BOSH is invalid", func() {
			It("returns an error", func() {
				fakeBOSHManager.VersionCall.Returns.Version = "X.5.2"
				err := command.CheckFastFails([]string{
					"--iaas", "aws",
				}, storage.State{Version: 999})

				Expect(err.Error()).To(ContainSubstring("invalid syntax"))
			})
		})

		Context("when iaas is not provided", func() {
			It("returns an error", func() {
				err := command.CheckFastFails([]string{}, storage.State{})
				Expect(err).To(MatchError("--iaas [gcp, aws] must be provided or BBL_IAAS must be set"))
			})
		})

		Context("when iaas specified is different than the iaas in state", func() {
			It("returns an error when the iaas is provided via args", func() {
				err := command.CheckFastFails([]string{"--iaas", "aws"}, storage.State{IAAS: "gcp"})
				Expect(err).To(MatchError("The iaas type cannot be changed for an existing environment. The current iaas type is gcp."))
			})

			It("returns an error when the iaas is provided via env vars", func() {
				fakeEnvGetter.Values = map[string]string{
					"BBL_IAAS": "aws",
				}
				err := command.CheckFastFails([]string{}, storage.State{IAAS: "gcp"})
				Expect(err).To(MatchError("The iaas type cannot be changed for an existing environment. The current iaas type is gcp."))
			})
		})

		Context("when bbl-state contains an env-id", func() {
			var (
				name  = "some-name"
				state = storage.State{EnvID: name}
			)

			Context("when the passed in name matches the env-id", func() {
				It("returns no error", func() {
					err := command.CheckFastFails([]string{
						"--iaas", "aws",
						"--name", name,
					}, state)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when the passed in name does not match the env-id", func() {
				It("returns an error", func() {
					err := command.CheckFastFails([]string{
						"--iaas", "aws",
						"--name", "some-other-name",
					}, state)
					Expect(err).To(MatchError(fmt.Sprintf("The director name cannot be changed for an existing environment. Current name is %s.", name)))
				})
			})
		})
	})

	Describe("Execute", func() {
		Context("when aws args are provided through environment variables", func() {
			BeforeEach(func() {
				fakeEnvGetter.Values = map[string]string{
					"BBL_AWS_ACCESS_KEY_ID":     "access-key-id-from-env",
					"BBL_AWS_SECRET_ACCESS_KEY": "secret-access-key-from-env",
					"BBL_AWS_REGION":            "region-from-env",
				}
			})

			It("uses the aws args provided by environment variables", func() {
				err := command.Execute([]string{
					"--iaas", "aws",
				}, storage.State{Version: 999})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeAWSUp.ExecuteCall.Receives.AWSUpConfig).To(Equal(commands.AWSUpConfig{
					AccessKeyID:     "access-key-id-from-env",
					SecretAccessKey: "secret-access-key-from-env",
					Region:          "region-from-env",
				}))
				Expect(fakeAWSUp.ExecuteCall.Receives.State).To(Equal(storage.State{
					Version: 999,
				}))
			})

			DescribeTable("gives precedence to arguments passed as command line args", func(args []string, expectedConfig commands.AWSUpConfig) {
				args = append(args, "--iaas", "aws")
				err := command.Execute(args, state)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeAWSUp.ExecuteCall.Receives.AWSUpConfig).To(Equal(expectedConfig))
			},
				Entry("precedence to aws access key id",
					[]string{"--aws-access-key-id", "access-key-id-from-args"},
					commands.AWSUpConfig{
						AccessKeyID:     "access-key-id-from-args",
						SecretAccessKey: "secret-access-key-from-env",
						Region:          "region-from-env",
					},
				),
				Entry("precedence to aws secret access key",
					[]string{"--aws-secret-access-key", "secret-access-key-from-args"},
					commands.AWSUpConfig{
						AccessKeyID:     "access-key-id-from-env",
						SecretAccessKey: "secret-access-key-from-args",
						Region:          "region-from-env",
					},
				),
				Entry("precedence to aws region",
					[]string{"--aws-region", "region-from-args"},
					commands.AWSUpConfig{
						AccessKeyID:     "access-key-id-from-env",
						SecretAccessKey: "secret-access-key-from-env",
						Region:          "region-from-args",
					},
				),
			)
		})

		Context("when an ops-file is provided via command line flag", func() {
			It("populates the aws config with the correct ops-file path", func() {
				fakeEnvGetter.Values = map[string]string{
					"BBL_AWS_ACCESS_KEY_ID":     "access-key-id-from-env",
					"BBL_AWS_SECRET_ACCESS_KEY": "secret-access-key-from-env",
					"BBL_AWS_REGION":            "region-from-env",
				}

				err := command.Execute([]string{
					"--iaas", "aws",
					"--ops-file", "some-ops-file-path",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeAWSUp.ExecuteCall.Receives.AWSUpConfig).To(Equal(commands.AWSUpConfig{
					AccessKeyID:     "access-key-id-from-env",
					SecretAccessKey: "secret-access-key-from-env",
					Region:          "region-from-env",
					OpsFilePath:     "some-ops-file-path",
				}))
			})

			It("populates the gcp config with the correct ops-file path", func() {
				fakeEnvGetter.Values = map[string]string{
					"BBL_GCP_SERVICE_ACCOUNT_KEY": "some-service-account-key-env",
					"BBL_GCP_PROJECT_ID":          "some-project-id-env",
					"BBL_GCP_ZONE":                "some-zone-env",
					"BBL_GCP_REGION":              "some-region-env",
				}

				err := command.Execute([]string{
					"--iaas", "gcp",
					"--ops-file", "some-ops-file-path",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				config, _ := fakeGCPUp.ExecuteArgsForCall(0)
				Expect(config).To(Equal(commands.GCPUpConfig{
					ServiceAccountKey: "some-service-account-key-env",
					ProjectID:         "some-project-id-env",
					Zone:              "some-zone-env",
					Region:            "some-region-env",
					OpsFilePath:       "some-ops-file-path",
				}))
			})
		})

		Context("when gcp args are provided through environment variables", func() {
			BeforeEach(func() {
				fakeEnvGetter.Values = map[string]string{
					"BBL_GCP_SERVICE_ACCOUNT_KEY": "some-service-account-key-env",
					"BBL_GCP_PROJECT_ID":          "some-project-id-env",
					"BBL_GCP_ZONE":                "some-zone-env",
					"BBL_GCP_REGION":              "some-region-env",
				}
			})

			It("uses the gcp args provided by environment variables", func() {
				err := command.Execute([]string{
					"--iaas", "gcp",
				}, storage.State{Version: 999})
				Expect(err).NotTo(HaveOccurred())

				config, state := fakeGCPUp.ExecuteArgsForCall(0)
				Expect(config).To(Equal(commands.GCPUpConfig{
					ServiceAccountKey: "some-service-account-key-env",
					ProjectID:         "some-project-id-env",
					Zone:              "some-zone-env",
					Region:            "some-region-env",
				}))
				Expect(state).To(Equal(storage.State{
					Version: 999,
				}))
			})

			DescribeTable("gives precedence to arguments passed as command line args", func(args []string, expectedConfig commands.GCPUpConfig) {
				args = append(args, "--iaas", "gcp")

				err := command.Execute(args, state)
				Expect(err).NotTo(HaveOccurred())

				config, _ := fakeGCPUp.ExecuteArgsForCall(0)
				Expect(config).To(Equal(expectedConfig))
			},
				Entry("precedence to service account key",
					[]string{"--gcp-service-account-key", "some-service-account-key-from-args"},
					commands.GCPUpConfig{
						ServiceAccountKey: "some-service-account-key-from-args",
						ProjectID:         "some-project-id-env",
						Zone:              "some-zone-env",
						Region:            "some-region-env",
					},
				),
				Entry("precedence to project id",
					[]string{"--gcp-project-id", "some-project-id-from-args"},
					commands.GCPUpConfig{
						ServiceAccountKey: "some-service-account-key-env",
						ProjectID:         "some-project-id-from-args",
						Zone:              "some-zone-env",
						Region:            "some-region-env",
					},
				),
				Entry("precedence to zone",
					[]string{"--gcp-zone", "some-zone-from-args"},
					commands.GCPUpConfig{
						ServiceAccountKey: "some-service-account-key-env",
						ProjectID:         "some-project-id-env",
						Zone:              "some-zone-from-args",
						Region:            "some-region-env",
					},
				),
				Entry("precedence to region",
					[]string{"--gcp-region", "some-region-from-args"},
					commands.GCPUpConfig{
						ServiceAccountKey: "some-service-account-key-env",
						ProjectID:         "some-project-id-env",
						Zone:              "some-zone-env",
						Region:            "some-region-from-args",
					},
				),
			)
		})

		Context("when state does not contain an iaas", func() {
			It("uses the iaas from the env var", func() {
				fakeEnvGetter.Values = map[string]string{
					"BBL_IAAS": "gcp",
				}
				err := command.Execute([]string{
					"--gcp-service-account-key", "some-service-account-key",
					"--gcp-project-id", "some-project-id",
					"--gcp-zone", "some-zone",
					"--gcp-region", "some-region",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeGCPUp.ExecuteCallCount()).To(Equal(1))
				Expect(fakeAzureUp.ExecuteCallCount()).To(Equal(0))
				Expect(fakeAWSUp.ExecuteCall.CallCount).To(Equal(0))
			})

			It("uses the iaas from the args over the env var", func() {
				fakeEnvGetter.Values = map[string]string{
					"BBL_IAAS": "aws",
				}
				err := command.Execute([]string{
					"--iaas", "gcp",
					"--gcp-service-account-key", "some-service-account-key",
					"--gcp-project-id", "some-project-id",
					"--gcp-zone", "some-zone",
					"--gcp-region", "some-region",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeGCPUp.ExecuteCallCount()).To(Equal(1))
			})

			Context("when desired iaas is gcp", func() {
				It("executes the GCP up with gcp details from args", func() {
					err := command.Execute([]string{
						"--iaas", "gcp",
						"--gcp-service-account-key", "some-service-account-key",
						"--gcp-project-id", "some-project-id",
						"--gcp-zone", "some-zone",
						"--gcp-region", "some-region",
					}, storage.State{})
					Expect(err).NotTo(HaveOccurred())

					config, _ := fakeGCPUp.ExecuteArgsForCall(0)
					Expect(fakeGCPUp.ExecuteCallCount()).To(Equal(1))
					Expect(config).To(Equal(commands.GCPUpConfig{
						ServiceAccountKey: "some-service-account-key",
						ProjectID:         "some-project-id",
						Zone:              "some-zone",
						Region:            "some-region",
					}))
				})

				It("executes the GCP up with gcp details from env vars", func() {
					fakeEnvGetter.Values = map[string]string{
						"BBL_GCP_SERVICE_ACCOUNT_KEY": "some-service-account-key",
						"BBL_GCP_PROJECT_ID":          "some-project-id",
						"BBL_GCP_ZONE":                "some-zone",
						"BBL_GCP_REGION":              "some-region",
					}
					err := command.Execute([]string{
						"--iaas", "gcp",
					}, storage.State{})
					Expect(err).NotTo(HaveOccurred())

					config, _ := fakeGCPUp.ExecuteArgsForCall(0)
					Expect(fakeGCPUp.ExecuteCallCount()).To(Equal(1))
					Expect(config).To(Equal(commands.GCPUpConfig{
						ServiceAccountKey: "some-service-account-key",
						ProjectID:         "some-project-id",
						Zone:              "some-zone",
						Region:            "some-region",
					}))
				})

				Context("when the user provides the jumpbox flag", func() {
					It("executes the GCP up with jumpbox set to true", func() {
						err := command.Execute([]string{
							"--iaas", "gcp",
							"--jumpbox",
						}, storage.State{})
						Expect(err).NotTo(HaveOccurred())

						config, _ := fakeGCPUp.ExecuteArgsForCall(0)
						Expect(fakeGCPUp.ExecuteCallCount()).To(Equal(1))
						Expect(config.Jumpbox).To(Equal(true))
					})
				})
			})

			Context("when desired iaas is aws", func() {
				It("executes the AWS up", func() {
					err := command.Execute([]string{
						"--iaas", "aws",
						"--aws-access-key-id", "some-access-key-id",
						"--aws-secret-access-key", "some-secret-access-key",
						"--aws-region", "some-region",
						"--aws-bosh-az", "some-bosh-az",
					}, storage.State{})
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeAWSUp.ExecuteCall.CallCount).To(Equal(1))
					Expect(fakeAWSUp.ExecuteCall.Receives.AWSUpConfig).To(Equal(commands.AWSUpConfig{
						AccessKeyID:     "some-access-key-id",
						SecretAccessKey: "some-secret-access-key",
						Region:          "some-region",
						BOSHAZ:          "some-bosh-az",
					}))
				})
			})

			Context("when the desired iaas is azure", func() {
				It("executes Azure up", func() {
					err := command.Execute([]string{
						"--iaas", "azure",
						"--azure-subscription-id", "subscription-id",
						"--azure-tenant-id", "tenant-id",
						"--azure-client-id", "client-id",
						"--azure-client-secret", "client-secret",
					}, storage.State{})
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeAzureUp.ExecuteCallCount()).To(Equal(1))
					config := fakeAzureUp.ExecuteArgsForCall(0)
					Expect(config).To(Equal(commands.AzureUpConfig{
						SubscriptionID: "subscription-id",
						TenantID:       "tenant-id",
						ClientID:       "client-id",
						ClientSecret:   "client-secret",
					}))
				})
			})

			Context("when an invalid iaas is provided", func() {
				It("returns an error", func() {
					err := command.Execute([]string{"--iaas", "bad-iaas"}, storage.State{})
					Expect(err).To(MatchError(`"bad-iaas" is an invalid iaas type, supported values are: [gcp, aws]`))
				})
			})

			Context("failure cases", func() {
				It("returns an error when the desired up command fails", func() {
					fakeAWSUp.ExecuteCall.Returns.Error = errors.New("failed execution")
					err := command.Execute([]string{"--iaas", "aws"}, storage.State{})
					Expect(err).To(MatchError("failed execution"))
				})

				It("returns an error when undefined flags are passed", func() {
					err := command.Execute([]string{"--foo", "bar"}, storage.State{})
					Expect(err).To(MatchError("flag provided but not defined: -foo"))
				})
			})
		})

		Context("when state contains an iaas", func() {
			Context("when iaas is AWS", func() {
				var state storage.State

				BeforeEach(func() {
					state = storage.State{
						IAAS: "aws",
						AWS: storage.AWS{
							AccessKeyID:     "some-access-key-id",
							SecretAccessKey: "some-secret-access-key",
							Region:          "some-region",
						},
					}
				})

				It("executes the AWS up", func() {
					err := command.Execute([]string{}, state)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeAWSUp.ExecuteCall.CallCount).To(Equal(1))
					Expect(fakeAWSUp.ExecuteCall.Receives.AWSUpConfig).To(Equal(commands.AWSUpConfig{}))
					Expect(fakeAWSUp.ExecuteCall.Receives.State).To(Equal(storage.State{
						IAAS: "aws",
						AWS: storage.AWS{
							AccessKeyID:     "some-access-key-id",
							SecretAccessKey: "some-secret-access-key",
							Region:          "some-region",
						},
					}))
				})

			})

			Context("when iaas is GCP", func() {
				It("executes the GCP up", func() {
					err := command.Execute([]string{}, storage.State{IAAS: "gcp"})
					Expect(err).NotTo(HaveOccurred())

					_, state := fakeGCPUp.ExecuteArgsForCall(0)
					Expect(state).To(Equal(storage.State{
						IAAS: "gcp",
					}))
				})
			})
		})

		Context("when the user provides the name flag", func() {
			It("passes the name flag in the up config", func() {
				err := command.Execute([]string{
					"--iaas", "aws",
					"--name", "a-better-name",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeAWSUp.ExecuteCall.Receives.AWSUpConfig.Name).To(Equal("a-better-name"))
			})
		})

		Context("when the user provides the no-director flag", func() {
			It("passes no-director as true in the up config", func() {
				err := command.Execute([]string{
					"--iaas", "aws",
					"--no-director",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeAWSUp.ExecuteCall.Receives.AWSUpConfig.NoDirector).To(Equal(true))
			})

			It("passes no-director as true in the up config", func() {
				err := command.Execute([]string{
					"--iaas", "gcp",
					"--no-director",
				}, storage.State{})
				Expect(err).NotTo(HaveOccurred())

				config, _ := fakeGCPUp.ExecuteArgsForCall(0)
				Expect(config.NoDirector).To(Equal(true))
			})
		})

	})
})
