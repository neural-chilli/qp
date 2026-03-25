package main

import (
	"sort"

	"github.com/neural-chilli/qp/internal/config"
)

func taskPlanGraph(cfg *config.Config, rootTask string) ([]string, [][2]string) {
	seenNode := map[string]bool{}
	seenEdge := map[[2]string]bool{}
	nodes := []string{}
	edges := [][2]string{}
	visiting := map[string]bool{}

	var addNode = func(name string) {
		if seenNode[name] {
			return
		}
		seenNode[name] = true
		nodes = append(nodes, name)
	}
	var addEdge = func(from, to string) {
		edge := [2]string{from, to}
		if seenEdge[edge] {
			return
		}
		seenEdge[edge] = true
		edges = append(edges, edge)
	}

	var visitTask func(string)
	visitTask = func(name string) {
		if visiting[name] {
			return
		}
		visiting[name] = true
		defer func() { delete(visiting, name) }()

		addNode(name)
		task, ok := cfg.Tasks[name]
		if !ok {
			return
		}
		for _, dep := range task.Needs {
			addEdge(name, dep)
			visitTask(dep)
		}
		for _, step := range task.Steps {
			addEdge(name, step)
			visitTask(step)
		}
		if task.Run != "" {
			runExpr, err := config.ParseRunExpr(task.Run)
			if err == nil {
				for _, ref := range config.RunExprRefs(runExpr) {
					addEdge(name, ref)
					visitTask(ref)
				}
			}
		}
	}

	visitTask(rootTask)
	sort.Strings(nodes)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i][0] != edges[j][0] {
			return edges[i][0] < edges[j][0]
		}
		return edges[i][1] < edges[j][1]
	})
	return nodes, edges
}

func guardPlanGraph(cfg *config.Config, guardName string) ([]string, [][2]string) {
	guardCfg, ok := cfg.Guards[guardName]
	if !ok {
		return nil, nil
	}
	nodes := []string{"guard:" + guardName}
	edges := [][2]string{}
	root := "guard:" + guardName
	for _, step := range guardCfg.Steps {
		nodes = append(nodes, step)
		edges = append(edges, [2]string{root, step})
	}
	sort.Strings(nodes)
	return nodes, edges
}
