package k8s

import (
	"context"
	"github.com/devtron-labs/devtron/pkg/cluster"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

type K8sCapacityService interface {
	GetClusterCapacityDetailsForAllClusters() ([]*ClusterCapacityDetails, error)
	GetClusterCapacityDetailsByClusterId(clusterId int) (*ClusterCapacityDetails, error)
	GetNodeCapacityDetailsListByClusterId(clusterId int) ([]*NodeCapacityDetails, error)
}
type K8sCapacityServiceImpl struct {
	logger                *zap.SugaredLogger
	clusterService        cluster.ClusterService
	k8sApplicationService K8sApplicationService
}

func NewK8sCapacityServiceImpl(Logger *zap.SugaredLogger,
	clusterService cluster.ClusterService,
	k8sApplicationService K8sApplicationService) *K8sCapacityServiceImpl {
	return &K8sCapacityServiceImpl{
		logger:                Logger,
		clusterService:        clusterService,
		k8sApplicationService: k8sApplicationService,
	}
}

type ClusterCapacityDetails struct {
	Cluster                  *cluster.ClusterBean `json:"cluster"`
	NodeCount                int                  `json:"nodeCount"`
	NodesK8sVersion          []string             `json:"nodesK8sVersion"`
	ClusterCpuCapacity       string               `json:"clusterCpuCapacity"`
	ClusterMemoryCapacity    string               `json:"clusterMemoryCapacity"`
	ClusterCpuAllocatable    string               `json:"clusterCpuAllocatable"`
	ClusterMemoryAllocatable string               `json:"clusterMemoryAllocatable"`
}

type NodeCapacityDetails struct {
	Name              string            `json:"name"`
	StatusReasonMap   map[string]string `json:"statusReasonMap"`
	PodCount          int               `json:"podCount"`
	TaintCount        int               `json:"taintCount"`
	CpuCapacity       string            `json:"cpuCapacity"`
	MemoryCapacity    string            `json:"memoryCapacity"`
	CpuAllocatable    string            `json:"cpuAllocatable"`
	MemoryAllocatable string            `json:"memoryAllocatable"`
	CpuUsage          string            `json:"cpuUsage"`
	MemoryUsage       string            `json:"memoryUsage"`
}

type ResourceMetric struct {
	ResourceType string `json:"resourceType"`
	Allocatable  string `json:"allocatable"`
	Utilization  string `json:"utilization"`
	Request      string `json:"request"`
	Limit        string `json:"limit"`
}

func (impl *K8sCapacityServiceImpl) GetClusterCapacityDetailsForAllClusters() ([]*ClusterCapacityDetails, error) {
	clusters, err := impl.clusterService.FindAll()
	if err != nil {
		impl.logger.Errorw("error in getting all clusters", "err", err)
		return nil, err
	}
	var clustersDetails []*ClusterCapacityDetails
	for _, cluster := range clusters {
		clusterCapacityDetails, err := impl.GetClusterCapacityDetailsByClusterId(cluster.Id)
		if err != nil {
			impl.logger.Errorw("error in getting cluster capacity details by id", "err", err)
			return nil, err
		}
		clusterCapacityDetails.Cluster = cluster
		clustersDetails = append(clustersDetails, clusterCapacityDetails)
	}
	return clustersDetails, nil
}

func (impl *K8sCapacityServiceImpl) GetClusterCapacityDetailsByClusterId(clusterId int) (*ClusterCapacityDetails, error) {
	//getting rest config by clusterId
	restConfig, err := impl.k8sApplicationService.GetRestConfigByClusterId(clusterId)
	if err != nil {
		impl.logger.Errorw("error in getting rest config by cluster id", "err", err, "clusterId", clusterId)
		return nil, err
	}
	//getting kubernetes clientSet by rest config
	k8sClientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		impl.logger.Errorw("error in getting client set by rest config", "err", err, "restConfig", restConfig)
		return nil, err
	}
	clusterDetails := &ClusterCapacityDetails{}
	nodeList, err := k8sClientSet.CoreV1().Nodes().List(context.Background(), v1.ListOptions{})
	if err != nil {
		impl.logger.Errorw("error in getting node list", "err", err)
		return nil, err
	}
	//TODO: add node status
	var clusterCpuCapacity resource.Quantity
	var clusterMemoryCapacity resource.Quantity
	var clusterCpuAllocatable resource.Quantity
	var clusterMemoryAllocatable resource.Quantity
	nodeCount := 0
	for _, node := range nodeList.Items {
		nodeCount += 1
		clusterDetails.NodesK8sVersion = append(clusterDetails.NodesK8sVersion, node.ResourceVersion)
		clusterCpuCapacity.Add(node.Status.Capacity["cpu"])
		clusterMemoryCapacity.Add(node.Status.Capacity["memory"])
		clusterCpuAllocatable.Add(node.Status.Allocatable["cpu"])
		clusterMemoryAllocatable.Add(node.Status.Allocatable["memory"])
	}
	clusterDetails.NodeCount = nodeCount
	clusterDetails.ClusterCpuCapacity = clusterCpuCapacity.String()
	clusterDetails.ClusterMemoryCapacity = clusterMemoryCapacity.String()
	clusterDetails.ClusterCpuAllocatable = clusterCpuAllocatable.String()
	clusterDetails.ClusterMemoryAllocatable = clusterMemoryAllocatable.String()
	return clusterDetails, nil
}

func (impl *K8sCapacityServiceImpl) GetNodeCapacityDetailsListByClusterId(clusterId int) ([]*NodeCapacityDetails, error) {
	//getting rest config by clusterId
	restConfig, err := impl.k8sApplicationService.GetRestConfigByClusterId(clusterId)
	if err != nil {
		impl.logger.Errorw("error in getting rest config by cluster id", "err", err, "clusterId", clusterId)
		return nil, err
	}
	//getting kubernetes clientSet by rest config
	k8sClientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		impl.logger.Errorw("error in getting client set by rest config", "err", err, "restConfig", restConfig)
		return nil, err
	}
	//getting metrics clientSet by rest config
	metricsClientSet, err := metrics.NewForConfig(restConfig)
	if err != nil {
		impl.logger.Errorw("error in getting metrics client set", "err", err)
		return nil, err
	}
	var nodeCpuUsage map[string]resource.Quantity
	var nodeMemoryUsage map[string]resource.Quantity
	nodeMetricsList, err := metricsClientSet.MetricsV1beta1().NodeMetricses().List(context.Background(), v1.ListOptions{})
	if err != nil {
		impl.logger.Errorw("error in getting node metrics", "err", err)
		return nil, err
	}

	for _, nodeMetrics := range nodeMetricsList.Items {
		nodeCpuUsage[nodeMetrics.Name] = nodeMetrics.Usage["cpu"]
		nodeMemoryUsage[nodeMetrics.Name] = nodeMetrics.Usage["memory"]
	}

	var nodeDetails []*NodeCapacityDetails
	nodeList, err := k8sClientSet.CoreV1().Nodes().List(context.Background(), v1.ListOptions{})
	if err != nil {
		impl.logger.Errorw("error in getting node list", "err", err)
		return nil, err
	}
	//empty namespace: get pods for all namespaces
	podList, err := k8sClientSet.CoreV1().Pods("").List(context.Background(), v1.ListOptions{})
	if err != nil {
		impl.logger.Errorw("error in getting pod list", "err", err)
		return nil, err
	}
	for _, node := range nodeList.Items {
		tmpPodCount := 0
		for _, pod := range podList.Items {
			if pod.Spec.NodeName == node.Name {
				tmpPodCount++
			}
		}
		cpuCapacity := node.Status.Capacity["cpu"]
		memoryCapacity := node.Status.Capacity["memory"]
		cpuAllocatable := node.Status.Allocatable["cpu"]
		memoryAllocatable := node.Status.Allocatable["memory"]
		nodeDetail := &NodeCapacityDetails{
			Name:              node.Name,
			PodCount:          tmpPodCount,
			TaintCount:        len(node.Spec.Taints),
			CpuCapacity:       cpuCapacity.String(),
			MemoryCapacity:    memoryCapacity.String(),
			CpuAllocatable:    cpuAllocatable.String(),
			MemoryAllocatable: memoryAllocatable.String(),
		}
		if cpuUsage, ok := nodeCpuUsage[node.Name]; ok {
			nodeDetail.CpuUsage = cpuUsage.String()
		}
		if memoryUsage, ok := nodeMemoryUsage[node.Name]; ok {
			nodeDetail.MemoryUsage = memoryUsage.String()
		}
		nodeDetails = append(nodeDetails, nodeDetail)
	}
	return nodeDetails, nil
}
