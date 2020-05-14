package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/handler"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
)

func main() {
	handlerLogger := lager.NewLogger("handler-simulator-logger")
	handlerLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	syncLogger := lager.NewLogger("sync-simulator-logger")
	syncLogger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	lrpBifrost := &bifrost.LRP{
		Converter: &ConverterSimulator{},
		Desirer:   &DesirerSimulator{},
	}

	stager := &StagerSimulator{}
	task := &TaskSimulator{}

	handler := handler.New(lrpBifrost, stager, stager, task, handlerLogger)

	fmt.Println("Starting to listen at 127.0.0.1:8085")
	handlerLogger.Fatal("simulator-crahsed", http.ListenAndServe("127.0.0.1:8085", handler))
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
		TargetInstances:  4,
		RunningInstances: 2,
	}, nil
}

func (d *DesirerSimulator) Update(updated *opi.LRP) error {
	return nil
}

func (d *DesirerSimulator) GetInstances(identifier opi.LRPIdentifier) ([]*opi.Instance, error) {
	if identifier.GUID == "jeff" && identifier.Version == "0.1.0" {
		return []*opi.Instance{
			{Index: 0, Since: 123456, State: opi.RunningState},
			{Index: 1, Since: 567891, State: opi.RunningState},
		}, nil
	}

	return []*opi.Instance{}, errors.New("no such app")
}

func (d *DesirerSimulator) Stop(identifier opi.LRPIdentifier) error {
	panic("not implemented")
}

func (d *DesirerSimulator) StopInstance(identifier opi.LRPIdentifier, index uint) error {
	return nil
}

type ConverterSimulator struct{}

func (c *ConverterSimulator) ConvertLRP(request cf.DesireLRPRequest) (opi.LRP, error) {
	return opi.LRP{}, nil
}

func (c *ConverterSimulator) ConvertTask(taskGUID string, taskRequest cf.TaskRequest) (opi.Task, error) {
	return opi.Task{}, nil
}

func (c *ConverterSimulator) ConvertStaging(stagingGUID string, stagingRequest cf.StagingRequest) (opi.StagingTask, error) {
	return opi.StagingTask{}, nil
}

type StagerSimulator struct{}

func (s *StagerSimulator) TransferStaging(ctx context.Context, stagingGUID string, request cf.StagingRequest) error {
	return nil
}

func (s *StagerSimulator) CompleteStaging(cf.TaskCompletedRequest) error {
	return nil
}

type TaskSimulator struct{}

func (t *TaskSimulator) TransferTask(ctx context.Context, taskGUID string, request cf.TaskRequest) error {
	return nil
}

func (t *TaskSimulator) CompleteTask(taskGUID string) error {
	return nil
}
