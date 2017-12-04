package main_test

import (
	"time"

	"code.cloudfoundry.org/bbs"
	"code.cloudfoundry.org/bbs/cmd/bbs/testrunner"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/durationjson"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("Convergence API", func() {
	Describe("ConvergeLRPs", func() {
		var processGuid string

		BeforeEach(func() {
			// make the converger more aggressive by running every second
			bbsConfig.ConvergeRepeatInterval = durationjson.Duration(time.Second)
			bbsRunner = testrunner.New(bbsBinPath, bbsConfig)
			bbsProcess = ginkgomon.Invoke(bbsRunner)

			cellPresence := models.NewCellPresence(
				"some-cell",
				"cell.example.com",
				"http://cell.example.com",
				"the-zone",
				models.NewCellCapacity(128, 1024, 6),
				[]string{},
				[]string{},
				[]string{},
				[]string{},
			)
			consulHelper.RegisterCell(&cellPresence)
			processGuid = "some-process-guid"
			err := client.DesireLRP(logger, model_helpers.NewValidDesiredLRP(processGuid))
			Expect(err).NotTo(HaveOccurred())
			err = client.RemoveActualLRP(logger, processGuid, 0, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("converges the lrps", func() {
			Eventually(func() []*models.ActualLRPGroup {
				groups, err := client.ActualLRPGroupsByProcessGuid(logger, processGuid)
				Expect(err).NotTo(HaveOccurred())
				return groups
			}).Should(HaveLen(1))
		})

		Context("when a task is desired but its cell is dead", func() {
			BeforeEach(func() {
				task := model_helpers.NewValidTask("task-guid")

				err := client.DesireTask(logger, task.TaskGuid, task.Domain, task.TaskDefinition)
				Expect(err).NotTo(HaveOccurred())

				_, err = client.StartTask(logger, task.TaskGuid, "dead-cell")
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the task as completed and failed", func() {
				Eventually(func() []*models.Task {
					return getTasksByState(client, models.Task_Completed)
				}).Should(HaveLen(1))

				Expect(getTasksByState(client, models.Task_Completed)[0].Failed).To(BeTrue())
			})
		})
	})
})

func getTasksByState(client bbs.InternalClient, state models.Task_State) []*models.Task {
	tasks, err := client.Tasks(logger)
	Expect(err).NotTo(HaveOccurred())

	filteredTasks := make([]*models.Task, 0)
	for _, task := range tasks {
		if task.State == state {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}
