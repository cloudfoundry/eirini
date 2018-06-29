package cmd

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
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

	handler := handler.New(bifrost, handlerLogger)

	log.Fatal(http.ListenAndServe("0.0.0.0:8085", handler))
}

type DesirerSimulator struct{}

func (d *DesirerSimulator) Desire(ctx context.Context, lrps []opi.LRP) error {
	return nil
}

func (d *DesirerSimulator) List(ctx context.Context) ([]opi.LRP, error) {
	panic("not implemented")
}

func (d *DesirerSimulator) Get(ctx context.Context, name string) (*opi.LRP, error) {
	if name != "jeff" {
		return &opi.LRP{}, errors.New("this is not jeff")
	}
	return &opi.LRP{
		Name:             "jeff",
		TargetInstances:  4,
		RunningInstances: 2,
	}, nil
}

func (d *DesirerSimulator) Update(ctx context.Context, updated opi.LRP) error {
	panic("not implemented")
}

func (d *DesirerSimulator) Stop(ctx context.Context, guid string) error {
	panic("not implemented")
}

type ConverterSimulator struct{}

func (c *ConverterSimulator) Convert(request cf.DesireLRPRequest) (opi.LRP, error) {
	return opi.LRP{}, nil
}
