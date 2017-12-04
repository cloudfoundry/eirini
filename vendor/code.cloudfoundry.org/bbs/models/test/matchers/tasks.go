package matchers

import (
	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func MatchTask(task *models.Task) types.GomegaMatcher {
	return SatisfyAll(
		WithTransform(func(t *models.Task) string {
			return t.TaskGuid
		}, Equal(task.TaskGuid)),
		WithTransform(func(t *models.Task) string {
			return t.Domain
		}, Equal(task.Domain)),
		WithTransform(func(t *models.Task) *models.TaskDefinition {
			return t.TaskDefinition
		}, Equal(task.TaskDefinition)),
	)
}

func MatchTasks(tasks []*models.Task) types.GomegaMatcher {
	matchers := []types.GomegaMatcher{}
	matchers = append(matchers, HaveLen(len(tasks)))

	for _, task := range tasks {
		matchers = append(matchers, ContainElement(MatchTask(task)))
	}

	return SatisfyAll(matchers...)
}
