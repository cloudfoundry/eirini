package kubelet

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type API interface {
	StatsSummary(nodename string) (StatsSummary, error)
}

//go:generate counterfeiter . NodeAPI
type NodeAPI interface {
	List(opts metav1.ListOptions) (*corev1.NodeList, error)
}

type StatsSummary struct {
	Pods []PodStats `json:"pods"`
}

type PodStats struct {
	PodRef     PodReference     `json:"podRef"`
	Containers []ContainerStats `json:"containers"`
}

type ContainerStats struct {
	Rootfs *FsStats `json:"rootfs,omitempty"`
	Logs   *FsStats `json:"logs,omitempty"`
}

type PodReference struct {
	UID string `json:"uid"`
}

type FsStats struct {
	UsedBytes *uint64 `json:"usedBytes,omitempty"`
}
