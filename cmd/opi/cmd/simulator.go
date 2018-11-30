package cmd

import (
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/spf13/cobra"
)

var simulatorCmd = &cobra.Command{
	Use:   "simulator",
	Short: "Simulate pre-defined state of the Kubernetes cluster.",
	Run:   simulate,
}

func simulate(cmd *cobra.Command, args []string) {
	handlerLogger := lager.NewLogger("handler-simulator-logger")
	handlerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	syncLogger := lager.NewLogger("sync-simulator-logger")
	syncLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	bifrost := &bifrost.Bifrost{
		Converter: &ConverterSimulator{},
		Desirer:   &DesirerSimulator{},
		Logger:    syncLogger,
	}

	stager := &StagerSimulator{}
	handler := handler.New(bifrost, stager, handlerLogger)

	log.Fatal(http.ListenAndServe("127.0.0.1:8085", handler))
}

type DesirerSimulator struct{}

func (d *DesirerSimulator) Desire(lrps *opi.LRP) error {
	return nil
}

func (d *DesirerSimulator) List() ([]*opi.LRP, error) {
	panic("not implemented")
}

func (d *DesirerSimulator) Get(identifier opi.LRPIdentifier) (*opi.LRP, error) {
	return &opi.LRP{
		Name:             "jeff",
		TargetInstances:  4,
		RunningInstances: 2,
		Metadata:         map[string]string{},
	}, nil
}

func (d *DesirerSimulator) Update(updated *opi.LRP) error {
	return nil
}

func (d *DesirerSimulator) GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error) {
	return []*opi.Instance{}, nil
}

func (d *DesirerSimulator) Stop(identifier opi.LRPIdentifier) error {
	panic("not implemented")
}

type ConverterSimulator struct{}

func (c *ConverterSimulator) Convert(request cf.DesireLRPRequest) (opi.LRP, error) {
	return opi.LRP{}, nil
}

type StagerSimulator struct{}

func (s *StagerSimulator) Stage(stagingGUID string, request cc_messages.StagingRequestFromCC) error {
	return nil
}

func (s *StagerSimulator) CompleteStaging(task *models.TaskCallbackResponse) error {
	return nil
}
