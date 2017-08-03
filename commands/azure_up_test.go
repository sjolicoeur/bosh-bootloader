package commands_test

import (
	"github.com/cloudfoundry/bosh-bootloader/commands"
	"github.com/cloudfoundry/bosh-bootloader/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AzureUp", func() {
	Describe("Execute", func() {
		var (
			command commands.AzureUp
			logger  *fakes.Logger
		)
		BeforeEach(func() {
			logger = &fakes.Logger{}
			command = commands.NewAzureUp(commands.NewAzureUpArgs{
				Logger: logger,
			})
		})
		Describe("checks the credentials", func() {
			Context("when the credentials are valid", func() {
				FIt("prints what it is doing", func() {
					command.Execute(commands.AzureUpConfig{})

					Expect(logger.StepCall.Receives.Message).To(Equal("verifying credentials"))
				})
			})
			Context("when the credentials are invalid", func() {
				PIt("prints an error")
			})
		})
	})
})
