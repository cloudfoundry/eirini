package test_helpers

import (
	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

func MatchActualLRPGroup(expected *models.ActualLRPGroup) types.GomegaMatcher {
	removeUntestedLRPFields := func(lrp *models.ActualLRPGroup) *models.ActualLRPGroup {
		newLRP := *lrp

		if newLRP.Instance != nil {
			newLRP.Instance.Since = 0
			newLRP.Instance.ModificationTag = models.ModificationTag{}
		}

		if newLRP.Evacuating != nil {
			newLRP.Evacuating.Since = 0
			newLRP.Evacuating.ModificationTag = models.ModificationTag{}
		}
		return &newLRP
	}

	expected = removeUntestedLRPFields(expected)
	return WithTransform(removeUntestedLRPFields, Equal(expected))
}
