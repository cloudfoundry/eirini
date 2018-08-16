package k8s

import (
	"fmt"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
)

type StatefulSetManager struct {
	Client         kubernetes.Interface
	Namespace      string
	ServiceManager ServiceManager
}

func NewStatefulSetManager(client kubernetes.Interface, namespace string) InstanceManager {
	return &StatefulSetManager{
		Client:         client,
		Namespace:      namespace,
		ServiceManager: NewServiceManager(client, namespace),
	}
}

func (m *StatefulSetManager) List() ([]*opi.LRP, error) {
	statefulsets, err := m.statefulSets().List(meta.ListOptions{})
	if err != nil {
		return nil, err
	}

	lrps := statefulSetsToLRPs(statefulsets)

	return lrps, nil
}

func (m *StatefulSetManager) Delete(appName string) error {
	backgroundPropagation := meta.DeletePropagationBackground
	if err := m.statefulSets().Delete(appName, &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation}); err != nil {
		return err
	}
	return m.ServiceManager.DeleteHeadless(appName)
}

func (m *StatefulSetManager) Create(lrp *opi.LRP) error {
	if _, err := m.statefulSets().Create(toStatefulSet(lrp)); err != nil {
		return err
	}
	return m.ServiceManager.CreateHeadless(lrp)
}

func (m *StatefulSetManager) Update(lrp *opi.LRP) error {
	statefulSet, err := m.statefulSets().Get(lrp.Name, meta.GetOptions{})
	if err != nil {
		return err
	}

	count := int32(lrp.TargetInstances)
	statefulSet.Spec.Replicas = &count
	statefulSet.Annotations[cf.LastUpdated] = lrp.Metadata[cf.LastUpdated]

	_, err = m.statefulSets().Update(statefulSet)
	return err
}

func (m *StatefulSetManager) Exists(appName string) (bool, error) {
	selector := fmt.Sprintf("name=%s", appName)
	list, err := m.statefulSets().List(meta.ListOptions{LabelSelector: selector})
	if err != nil {
		return false, err
	}

	return len(list.Items) > 0, nil
}

func (m *StatefulSetManager) Get(appName string) (*opi.LRP, error) {
	statefulSet, err := m.statefulSets().Get(appName, meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	lrp := statefulSetToLRP(statefulSet)

	return lrp, nil
}

func (m *StatefulSetManager) statefulSets() types.StatefulSetInterface {
	return m.Client.AppsV1beta2().StatefulSets(m.Namespace)
}

func statefulSetsToLRPs(statefulSets *v1beta2.StatefulSetList) []*opi.LRP {
	lrps := []*opi.LRP{}
	for _, s := range statefulSets.Items {
		lrp := statefulSetToLRP(&s)
		lrps = append(lrps, lrp)
	}
	return lrps
}

func statefulSetToLRP(s *v1beta2.StatefulSet) *opi.LRP {
	return &opi.LRP{
		Name:             s.Name,
		Image:            s.Spec.Template.Spec.Containers[0].Image,
		Command:          s.Spec.Template.Spec.Containers[0].Command,
		RunningInstances: int(s.Status.CurrentReplicas),
		Metadata: map[string]string{
			cf.ProcessGUID: s.Annotations[cf.ProcessGUID],
			cf.LastUpdated: s.Annotations[cf.LastUpdated],
		},
	}
}

func toStatefulSet(lrp *opi.LRP) *v1beta2.StatefulSet {
	envs := MapToEnvVar(lrp.Env)
	envs = append(envs, v1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	statefulSet := &v1beta2.StatefulSet{
		Spec: v1beta2.StatefulSetSpec{
			Replicas: int32ptr(lrp.TargetInstances),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "opi",
							Image:   lrp.Image,
							Command: lrp.Command,
							Env:     envs,
							Ports: []v1.ContainerPort{
								{
									Name:          "expose",
									ContainerPort: 8080,
								},
							},
						},
					},
				},
			},
		},
	}

	statefulSet.Name = lrp.Name
	statefulSet.Spec.Template.Labels = map[string]string{
		"name": lrp.Name,
	}

	statefulSet.Spec.Selector = &meta.LabelSelector{
		MatchLabels: map[string]string{
			"name": lrp.Name,
		},
	}

	statefulSet.Labels = map[string]string{
		"eirini": "eirini",
		"name":   lrp.Name,
	}

	statefulSet.Annotations = lrp.Metadata

	return statefulSet
}
