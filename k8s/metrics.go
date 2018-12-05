package k8s

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"code.cloudfoundry.org/eirini/metrics"
	"code.cloudfoundry.org/eirini/route"
	"github.com/pkg/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodMetricsList struct {
	Metadata Metadata      `json:"metadata"`
	Items    []*PodMetrics `json:"items"`
}

type PodMetrics struct {
	Metadata   Metadata     `json:"metadata"`
	Containers []*Container `json:"containers"`
}

type Metadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Container struct {
	Name  string `json:"name"`
	Usage Usage  `json:"usage"`
}

type Usage struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type MetricsCollector struct {
	work      chan<- []metrics.Message
	source    string
	scheduler route.TaskScheduler
	podClient core.PodInterface
}

func NewMetricsCollector(work chan []metrics.Message, scheduler route.TaskScheduler, source string, podClient core.PodInterface) *MetricsCollector {
	return &MetricsCollector{
		work:      work,
		source:    source,
		scheduler: scheduler,
		podClient: podClient,
	}
}

func (c *MetricsCollector) Start() {
	c.scheduler.Schedule(func() error {
		metricList, err := collectMetrics(c.source)
		if err != nil {
			fmt.Println("Failed to collect metric: ", err)
			return err
		}

		messages, err := c.convertMetricsList(metricList)
		if err != nil {
			return err
		}

		if len(messages) > 0 {
			c.work <- messages
		}

		return nil
	})
}

func collectMetrics(source string) (*PodMetricsList, error) {
	resp, err := http.Get(source)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("metrics source responded with code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	metricList := &PodMetricsList{}
	err = json.Unmarshal(body, metricList)
	return metricList, err
}

func (c *MetricsCollector) convertMetricsList(metricList *PodMetricsList) ([]metrics.Message, error) {
	messages := []metrics.Message{}
	for _, metric := range metricList.Items {
		if len(metric.Containers) == 0 {
			continue
		}
		container := metric.Containers[0]
		_, indexID, err := parsePodName(metric.Metadata.Name)
		if err != nil {
			return nil, err
		}
		cpuValue, err := extractValue(container.Usage.CPU)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to convert cpu value")
		}
		memoryValue, err := extractValue(container.Usage.Memory)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to convert memory values")
		}

		pod, err := c.podClient.Get(metric.Metadata.Name, meta.GetOptions{})
		if err != nil {
			return []metrics.Message{}, err
		}

		messages = append(messages, metrics.Message{
			AppID:       pod.Labels["guid"],
			IndexID:     indexID,
			CPU:         convertCPU(cpuValue),
			Memory:      convertMemory(memoryValue),
			MemoryQuota: 10,
			Disk:        42000000,
			DiskQuota:   10,
		})
	}
	return messages, nil
}

func isInt(str string) bool {
	_, err := strconv.Atoi(str)
	return err == nil
}

func parsePodName(podName string) (string, string, error) {
	sl := strings.Split(podName, "-")

	if len(sl) <= 1 {
		return "", "", fmt.Errorf("Could not parse pod name from %s", podName)
	}

	podID := strings.Join(sl[:len(sl)-1], "-")
	indexID := sl[len(sl)-1]
	if !isInt(indexID) {
		indexID = "0"
	}

	return podID, indexID, nil
}

func extractValue(metric string) (float64, error) {
	re := regexp.MustCompile("[a-zA-Z]+")
	match := re.FindStringSubmatch(metric)
	if len(match) == 0 {
		f, err := strconv.ParseFloat(metric, 64)
		return f, errors.Wrap(err, fmt.Sprintf("failed to parse metric %s", metric))
	}

	unit := match[0]
	valueStr := strings.Trim(metric, unit)

	return strconv.ParseFloat(valueStr, 64)
}

func convertCPU(cpuUsage float64) float64 {
	return cpuUsage / 1000
}

func convertMemory(memoryUsage float64) float64 {
	return memoryUsage * 1024
}
