package commands_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/iam"
	"github.com/pivotal-cf-experimental/bosh-bootloader/commands"
	"github.com/pivotal-cf-experimental/bosh-bootloader/fakes"
	"github.com/pivotal-cf-experimental/bosh-bootloader/storage"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Update LBs", func() {
	var (
		command                   commands.UpdateLBs
		incomingState             storage.State
		certFile                  *os.File
		keyFile                   *os.File
		certificateManager        *fakes.CertificateManager
		availabilityZoneRetriever *fakes.AvailabilityZoneRetriever
		infrastructureManager     *fakes.InfrastructureManager
	)

	var updateLBs = func(certificatePath string, keyPath string, state storage.State) (storage.State, error) {
		return command.Execute([]string{
			"--cert", certificatePath,
			"--key", keyPath,
		}, state)
	}

	BeforeEach(func() {
		var err error

		certificateManager = &fakes.CertificateManager{}
		availabilityZoneRetriever = &fakes.AvailabilityZoneRetriever{}
		infrastructureManager = &fakes.InfrastructureManager{}

		availabilityZoneRetriever.RetrieveCall.Returns.AZs = []string{"a", "b", "c"}
		certificateManager.CreateCall.Returns.CertificateName = "some-certificate-name"
		certificateManager.DescribeCall.Returns.Certificate = iam.Certificate{
			ARN: "some-certificate-arn",
		}

		infrastructureManager.ExistsCall.Returns.Exists = true

		incomingState = storage.State{
			Stack: storage.Stack{
				LBType:          "concourse",
				CertificateName: "some-certificate-name",
			},
		}

		certFile, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(certFile.Name(), []byte("some-certificate-contents"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		keyFile, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(keyFile.Name(), []byte("some-key-contents"), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		command = commands.NewUpdateLBs(certificateManager, availabilityZoneRetriever, infrastructureManager)
	})

	Describe("Execute", func() {
		It("creates the new certificate and private key", func() {
			updateLBs(certFile.Name(), keyFile.Name(), storage.State{
				Stack: storage.Stack{
					LBType: "cf",
				},
				AWS: storage.AWS{
					AccessKeyID:     "some-access-key-id",
					SecretAccessKey: "some-secret-access-key",
					Region:          "some-region",
				},
			})

			Expect(certificateManager.CreateCall.Receives.Certificate).To(Equal(certFile.Name()))
			Expect(certificateManager.CreateCall.Receives.PrivateKey).To(Equal(keyFile.Name()))
		})

		It("updates cloudformation with the new certificate", func() {
			updateLBs(certFile.Name(), keyFile.Name(), storage.State{
				Stack: storage.Stack{
					Name:   "some-stack",
					LBType: "concourse",
				},
				AWS: storage.AWS{
					AccessKeyID:     "some-access-key-id",
					SecretAccessKey: "some-secret-access-key",
					Region:          "some-region",
				},
				KeyPair: storage.KeyPair{
					Name: "some-key-pair",
				},
			})

			Expect(availabilityZoneRetriever.RetrieveCall.Receives.Region).To(Equal("some-region"))

			Expect(certificateManager.DescribeCall.Receives.CertificateName).To(Equal("some-certificate-name"))

			Expect(infrastructureManager.UpdateCall.Receives.KeyPairName).To(Equal("some-key-pair"))
			Expect(infrastructureManager.UpdateCall.Receives.NumberOfAvailabilityZones).To(Equal(3))
			Expect(infrastructureManager.UpdateCall.Receives.StackName).To(Equal("some-stack"))
			Expect(infrastructureManager.UpdateCall.Receives.LBType).To(Equal("concourse"))
			Expect(infrastructureManager.UpdateCall.Receives.LBCertificateARN).To(Equal("some-certificate-arn"))
		})

		It("deletes the existing certificate and private key", func() {
			updateLBs(certFile.Name(), keyFile.Name(), storage.State{
				Stack: storage.Stack{
					LBType:          "cf",
					CertificateName: "some-certificate-name",
				},
			})

			Expect(certificateManager.DeleteCall.Receives.CertificateName).To(Equal("some-certificate-name"))
		})

		It("returns an error if the user hasn't bbl up'd yet", func() {
			infrastructureManager.ExistsCall.Returns.Exists = false
			_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
			Expect(err).To(MatchError(commands.BBLNotFound))
		})

		It("returns an error if there is no lb", func() {
			_, err := updateLBs(certFile.Name(), keyFile.Name(), storage.State{
				Stack: storage.Stack{
					LBType: "none",
				},
			})
			Expect(err).To(MatchError("no load balancer has been found for this bbl environment"))
		})

		It("does not update the certificate if the provided certificate is the same", func() {
			certificateManager.DescribeCall.Returns.Certificate = iam.Certificate{
				Body: "some-certificate-contents",
			}

			_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
			Expect(err).NotTo(HaveOccurred())

			Expect(certificateManager.CreateCall.CallCount).To(Equal(0))
			Expect(certificateManager.DeleteCall.CallCount).To(Equal(0))
			Expect(infrastructureManager.UpdateCall.CallCount).To(Equal(0))
		})

		Describe("state manipulation", func() {
			It("updates the state with the new certificate name", func() {
				certificateManager.CreateCall.Returns.CertificateName = "some-new-certificate-name"

				state, err := updateLBs(certFile.Name(), keyFile.Name(), storage.State{
					Stack: storage.Stack{
						LBType:          "cf",
						CertificateName: "some-certificate-name",
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(state.Stack.CertificateName).To(Equal("some-new-certificate-name"))
			})
		})

		Describe("failure cases", func() {
			It("returns an error when the original certificate cannot be described", func() {
				certificateManager.DescribeCall.Stub = func(certificateName string) (iam.Certificate, error) {
					if certificateName == "some-certificate-name" {
						return iam.Certificate{}, errors.New("old certificate failed to describe")
					}

					return iam.Certificate{}, nil
				}

				_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
				Expect(err).To(MatchError("old certificate failed to describe"))
			})

			It("returns an error when new certificate cannot be described", func() {
				certificateManager.CreateCall.Returns.CertificateName = "some-new-certificate-name"

				certificateManager.DescribeCall.Stub = func(certificateName string) (iam.Certificate, error) {
					if certificateName == "some-new-certificate-name" {
						return iam.Certificate{}, errors.New("new certificate failed to describe")
					}

					return iam.Certificate{}, nil
				}

				_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
				Expect(err).To(MatchError("new certificate failed to describe"))
			})

			It("returns an error when the certificate file cannot be read", func() {
				_, err := updateLBs("some-fake-file", keyFile.Name(), incomingState)
				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})

			It("returns an error when the infrastructure manager fails to check the existance of a stack", func() {
				infrastructureManager.ExistsCall.Returns.Error = errors.New("failed to check for stack")
				_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
				Expect(err).To(MatchError("failed to check for stack"))
			})

			It("returns an error when invalid flags are provided", func() {
				_, err := command.Execute([]string{
					"--invalid-flag",
				}, incomingState)

				Expect(err).To(MatchError(ContainSubstring("flag provided but not defined")))
			})

			It("returns an error when infrastructure update fails", func() {
				infrastructureManager.UpdateCall.Returns.Error = errors.New("failed to update stack")
				_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
				Expect(err).To(MatchError("failed to update stack"))
			})

			It("returns an error when availability zone retriever fails", func() {
				availabilityZoneRetriever.RetrieveCall.Returns.Error = errors.New("az retrieve failed")
				_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
				Expect(err).To(MatchError("az retrieve failed"))
			})

			It("returns an error when certificate creation fails", func() {
				certificateManager.CreateCall.Returns.Error = errors.New("certificate creation failed")
				_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
				Expect(err).To(MatchError("certificate creation failed"))
			})

			It("returns an error when certificate deletion fails", func() {
				certificateManager.DeleteCall.Returns.Error = errors.New("certificate deletion failed")
				_, err := updateLBs(certFile.Name(), keyFile.Name(), incomingState)
				Expect(err).To(MatchError("certificate deletion failed"))
			})
		})
	})
})
