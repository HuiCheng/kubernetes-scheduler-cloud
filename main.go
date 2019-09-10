package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"sort"

	macaron "gopkg.in/macaron.v1"
	coreV1 "k8s.io/api/core/v1"
	extenderArgsAPIV1 "k8s.io/kubernetes/pkg/scheduler/api/v1"
)

var (
	version string
)

type scheduler struct {
}

// 获取pod的处理器、内存的设置
func (s *scheduler) getPodResources(pod *coreV1.Pod) (cpuNum, memoryNum int64) {
	for _, c := range pod.Spec.Containers {
		cpuNum = cpuNum + c.Resources.Limits.Cpu().MilliValue()
		memoryNum = memoryNum + c.Resources.Limits.Memory().MilliValue()
	}
	return
}

// 选出一个资源最少且可以满足要求的节点
func (s *scheduler) getMostSuitableNode(nodes *coreV1.NodeList, cpuNum, memoryNum int64) coreV1.Node {
	nodeCPUSortList := nodes.DeepCopy().Items
	sort.Slice(nodeCPUSortList, func(i, j int) bool {
		return nodeCPUSortList[i].Status.Allocatable.Cpu().MilliValue() < nodeCPUSortList[j].Status.Allocatable.Cpu().MilliValue()
	})

	nodeMemorySortList := nodes.DeepCopy().Items
	sort.Slice(nodeMemorySortList, func(i, j int) bool {
		return nodeMemorySortList[i].Status.Allocatable.Memory().MilliValue() < nodeMemorySortList[j].Status.Allocatable.Memory().MilliValue()
	})

	nodeCPUSortChan := make(chan coreV1.Node, len(nodeCPUSortList))
	for _, n := range nodeCPUSortList {
		nodeCPUSortChan <- n
	}
	close(nodeCPUSortChan)

	nodeMemorySortChan := make(chan coreV1.Node, len(nodeMemorySortList))
	for _, n := range nodeMemorySortList {
		nodeMemorySortChan <- n
	}
	close(nodeMemorySortChan)

	var nodeCPUOK coreV1.Node
	var nodeMemoryOK coreV1.Node

	for i := 1; i < len(nodes.Items)*2; i++ {
		select {
		case cpuNode := <-nodeCPUSortChan:
			log.Println("CPU", cpuNode.Name)
			if nodeCPUOK.Name == "" && cpuNode.Status.Allocatable.Cpu().MilliValue() > cpuNum {
				nodeCPUOK = cpuNode
			}
		case nodeMemory := <-nodeMemorySortChan:
			log.Println("MEMORY", nodeMemory.Name)
			if nodeMemoryOK.Name == "" && nodeMemory.Status.Allocatable.Memory().MilliValue() > cpuNum {
				nodeMemoryOK = nodeMemory
			}
		}

		if nodeCPUOK.Name != "" &&
			nodeMemoryOK.Name != "" &&
			nodeCPUOK.Name == nodeMemoryOK.Name {
			log.Println("END", nodeCPUOK.Name, nodeMemoryOK.Name)
			return nodeCPUOK
		}
	}
	return coreV1.Node{}
}

func (s *scheduler) predicatesHandler(ctx *macaron.Context) string {
	extenderArgs := extenderArgsAPIV1.ExtenderArgs{}
	extenderFilterResult := extenderArgsAPIV1.ExtenderFilterResult{}
	if err := json.NewDecoder(ctx.Req.Body().ReadCloser()).Decode(&extenderArgs); err != nil {
		extenderFilterResult.Error = "非法信息"
	}

	cpuNum, memoryNum := s.getPodResources(extenderArgs.Pod)
	if cpuNum == 0 || memoryNum == 0 {
		extenderFilterResult.Error = "Pod没有设置Resource.Limit.[cpu|memory]的仠!"
	}

	nodeOK := s.getMostSuitableNode(extenderArgs.Nodes, cpuNum, memoryNum)
	if nodeOK.Name == "" {
		extenderFilterResult.Error = "没有找到合适的节点"
	} else {
		extenderFilterResult.Nodes = &coreV1.NodeList{Items: []coreV1.Node{nodeOK}}
	}

	tmp, _ := json.Marshal(extenderFilterResult)
	return string(tmp)
}

func init() {
	flag.StringVar(&version, "api version", "/v1/", "")
	flag.Parse()
}

func main() {
	s := scheduler{}
	m := macaron.Classic()

	m.Get("/", func() string { return "Hello Scheduler!\n" })
	m.Post(fmt.Sprint(version, "predicates"), s.predicatesHandler)
	m.Run(8880)
}
