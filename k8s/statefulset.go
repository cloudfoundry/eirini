package k8s

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	types "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
)

type StatefulSetDesirer struct {
	Client                kubernetes.Interface
	Namespace             string
	LivenessProbeCreator  ProbeCreator
	ReadinessProbeCreator ProbeCreator
}

//go:generate counterfeiter . ProbeCreator
type ProbeCreator func(lrp *opi.LRP) *v1.Probe

func NewStatefulSetDesirer(client kubernetes.Interface, namespace string) opi.Desirer {
	return &StatefulSetDesirer{
		Client:                client,
		Namespace:             namespace,
		LivenessProbeCreator:  CreateLivenessProbe,
		ReadinessProbeCreator: CreateReadinessProbe,
	}
}

func (m *StatefulSetDesirer) List() ([]*opi.LRP, error) {
	statefulsets, err := m.statefulSets().List(meta.ListOptions{})
	if err != nil {
		return nil, err
	}

	lrps := statefulSetsToLRPs(statefulsets)

	return lrps, nil
}

func (m *StatefulSetDesirer) Stop(appName string) error {
	backgroundPropagation := meta.DeletePropagationBackground
	return m.statefulSets().Delete(appName, &meta.DeleteOptions{PropagationPolicy: &backgroundPropagation})
}

func (m *StatefulSetDesirer) Desire(lrp *opi.LRP) error {
	_, err := m.statefulSets().Create(m.toStatefulSet(lrp))
	return err
}

func (m *StatefulSetDesirer) Update(lrp *opi.LRP) error {
	statefulSet, err := m.statefulSets().Get(lrp.Name, meta.GetOptions{})
	if err != nil {
		return err
	}

	count := int32(lrp.TargetInstances)
	statefulSet.Spec.Replicas = &count
	statefulSet.Annotations[cf.LastUpdated] = lrp.Metadata[cf.LastUpdated]
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]

	_, err = m.statefulSets().Update(statefulSet)
	return err
}

func (m *StatefulSetDesirer) Get(appName string) (*opi.LRP, error) {
	statefulSet, err := m.statefulSets().Get(appName, meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	lrp := statefulSetToLRP(statefulSet)

	return lrp, nil
}

func (m *StatefulSetDesirer) statefulSets() types.StatefulSetInterface {
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
		RunningInstances: int(s.Status.ReadyReplicas),
		Metadata: map[string]string{
			cf.ProcessGUID:          s.Annotations[cf.ProcessGUID],
			cf.LastUpdated:          s.Annotations[cf.LastUpdated],
			cf.VcapAppUris:          s.Annotations[cf.VcapAppUris],
			eirini.RegisteredRoutes: s.Annotations[cf.VcapAppUris],
		},
	}
}

func (m *StatefulSetDesirer) toStatefulSet(lrp *opi.LRP) *v1beta2.StatefulSet {
	envs := MapToEnvVar(lrp.Env)
	envs = append(envs, v1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	})

	livenessProbe := m.LivenessProbeCreator(lrp)
	readinessProbe := m.ReadinessProbeCreator(lrp)

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
							LivenessProbe:  livenessProbe,
							ReadinessProbe: readinessProbe,
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
	statefulSet.Annotations[eirini.RegisteredRoutes] = lrp.Metadata[cf.VcapAppUris]

	return statefulSet
}
