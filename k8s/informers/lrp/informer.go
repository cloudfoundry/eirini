package lrp

import (
	eiriniv1 "code.cloudfoundry.org/eirini/pkg/apis/lrp/v1"
	lrpclientset "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned"
	lrpinformers "code.cloudfoundry.org/eirini/pkg/generated/informers/externalversions"
	"code.cloudfoundry.org/lager"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

type Controller interface {
	Create(eiriniv1.LRP) error
	Update(oldLRP eiriniv1.LRP, newLRP eiriniv1.LRP) error
	Delete(eiriniv1.LRP) error
}

type Informer struct {
	logger     lager.Logger
	client     lrpclientset.Interface
	controller Controller
}

func NewInformer(logger lager.Logger, client lrpclientset.Interface, controller Controller) *Informer {
	return &Informer{
		logger:     logger,
		client:     client,
		controller: controller,
	}
}

func (i Informer) Start() {
	informerFactory := lrpinformers.NewSharedInformerFactory(i.client, 0)
	informer := informerFactory.Eirini().V1().LRPs().Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object interface{}) {
			lrp := object.(eiriniv1.LRP)
			if err := i.controller.Create(lrp); err != nil {
				i.logger.Error("create-lrp-failed", err, lager.Data{"lrp": lrp})
			}
		},
		UpdateFunc: func(oldOb, newObj interface{}) {
			newLRP := newObj.(eiriniv1.LRP)
			oldLRP := newObj.(eiriniv1.LRP)
			if err := i.controller.Update(oldLRP, newLRP); err != nil {
				i.logger.Error("update-lrp-failed", err, lager.Data{"new_lrp": newLRP, "old_lrp": oldLRP})
			}
		},
		DeleteFunc: func(object interface{}) {
			lrp := object.(eiriniv1.LRP)
			if err := i.controller.Delete(object.(eiriniv1.LRP)); err != nil {
				i.logger.Error("delete-lrp-failed", err, lager.Data{"lrp": lrp})
			}
		},
	})
	informer.Run(wait.NeverStop)
}
