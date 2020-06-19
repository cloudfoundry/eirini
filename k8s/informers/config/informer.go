package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v2"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/lager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	OpiConfigName       = "opi.yml"
	EiriniConfigMapName = "eirini"
)

//counterfeiter:generate . SecretReplicator
type SecretReplicator interface {
	ReplicateSecret(srcNamespace, srcSecretName, dstNamespace, dstSecretName string) error
}

type Informer struct {
	clientset        kubernetes.Interface
	syncPeriod       time.Duration
	namespace        string
	secretReplicator SecretReplicator
	stopperChan      chan struct{}
	logger           lager.Logger
}

func NewInformer(
	client kubernetes.Interface,
	syncPeriod time.Duration,
	namespace string,
	secretReplicator SecretReplicator,
	stopperChan chan struct{},
	logger lager.Logger,
) *Informer {
	return &Informer{
		clientset:        client,
		syncPeriod:       syncPeriod,
		namespace:        namespace,
		secretReplicator: secretReplicator,
		stopperChan:      stopperChan,
		logger:           logger,
	}
}

func (c *Informer) Start() {
	factory := informers.NewSharedInformerFactoryWithOptions(
		c.clientset,
		c.syncPeriod,
		informers.WithNamespace(c.namespace),
		informers.WithTweakListOptions(withName(EiriniConfigMapName)),
	)

	informer := factory.Core().V1().ConfigMaps().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			config := obj.(*corev1.ConfigMap)
			c.addConfig(config)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldConfig := oldObj.(*corev1.ConfigMap)
			newConfig := newObj.(*corev1.ConfigMap)
			c.updateConfig(oldConfig, newConfig)
		},
	})

	informer.Run(c.stopperChan)
}

func (c *Informer) addConfig(configMap *corev1.ConfigMap) {
	opiConfig, err := unmarshalConfig(configMap.Data[OpiConfigName])
	if err != nil {
		c.logger.Debug("error-unmarshaling-opi-config", lager.Data{
			"config": configMap.Data[OpiConfigName],
			"reason": err.Error(),
		})
		return
	}

	srcSecretName := opiConfig.Properties.RegistrySecretName
	dstNamespace := opiConfig.Properties.Namespace

	if err := c.secretReplicator.ReplicateSecret(c.namespace, srcSecretName, dstNamespace, eirini.RegistrySecretName); err != nil {
		c.logger.Error("failed-to-replicate-registry-secret", err, lager.Data{
			"srcNamespace":  c.namespace,
			"srcSecretName": srcSecretName,
			"dstNamespace":  dstNamespace,
			"dstSecretName": eirini.RegistrySecretName,
		})
	}
}

func (c *Informer) updateConfig(oldConfigMap, newConfigMap *corev1.ConfigMap) {
	oldOpiConfig, err := unmarshalConfig(oldConfigMap.Data[OpiConfigName])
	if err != nil {
		c.logger.Debug("error-unmarshaling-opi-config", lager.Data{
			"config": oldConfigMap.Data[OpiConfigName],
			"reason": err.Error(),
		})
		return
	}

	newOpiConfig, err := unmarshalConfig(newConfigMap.Data[OpiConfigName])
	if err != nil {
		c.logger.Debug("error-unmarshaling-opi-config", lager.Data{
			"config": oldConfigMap.Data[OpiConfigName],
			"reason": err.Error(),
		})
		return
	}

	if newOpiConfig.Properties.RegistrySecretName != oldOpiConfig.Properties.RegistrySecretName {
		srcSecretName := newOpiConfig.Properties.RegistrySecretName
		dstNamespace := newOpiConfig.Properties.Namespace

		if err := c.secretReplicator.ReplicateSecret(c.namespace, srcSecretName, dstNamespace, eirini.RegistrySecretName); err != nil {
			c.logger.Error("failed-to-replicate-registry-secret", err, lager.Data{
				"srcNamespace":  c.namespace,
				"srcSecretName": srcSecretName,
				"dstNamespace":  dstNamespace,
				"dstSecretName": eirini.RegistrySecretName,
			})
		}
	}
}

func withName(name string) func(opts *metav1.ListOptions) {
	return func(opts *metav1.ListOptions) {
		opts.FieldSelector = fmt.Sprintf("metadata.name=%s", name)
	}
}

func unmarshalConfig(data string) (*eirini.Config, error) {
	var conf eirini.Config
	err := yaml.Unmarshal([]byte(data), &conf)
	return &conf, err
}
