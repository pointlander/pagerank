/*
Package pagerank implements the *weighted* PageRank algorithm.
*/
package pagerank

import (
	"fmt"
	"sync"
)

// Node64 is a node in a graph
type Node64 struct {
	sync.RWMutex
	weight   [2]float64
	outbound float64
	edges    map[uint]float64
}

// Graph64 holds node and edge data.
type Graph64 struct {
	Verbose bool
	count   uint
	index   map[uint64]uint
	nodes   []Node64
}

// NewGraph64 initializes and returns a new graph.
func NewGraph64(size ...int) *Graph64 {
	capacity := 8
	if len(size) == 1 {
		capacity = size[0]
	}
	return &Graph64{
		index: make(map[uint64]uint, capacity),
		nodes: make([]Node64, 0, capacity),
	}
}

// Link creates a weighted edge between a source-target node pair.
// If the edge already exists, the weight is incremented.
func (g *Graph64) Link(source, target uint64, weight float64) {
	s, ok := g.index[source]
	if !ok {
		s = g.count
		g.index[source] = s
		g.nodes = append(g.nodes, Node64{})
		g.count++
	}

	g.nodes[s].outbound += weight

	t, ok := g.index[target]
	if !ok {
		t = g.count
		g.index[target] = t
		g.nodes = append(g.nodes, Node64{})
		g.count++
	}

	if g.nodes[s].edges == nil {
		g.nodes[s].edges = map[uint]float64{}
	}

	g.nodes[s].edges[t] += weight
}

// Rank computes the PageRank of every node in the directed graph.
// α (alpha) is the damping factor, usually set to 0.85.
// ε (epsilon) is the convergence criteria, usually set to a tiny value.
//
// This method will run as many iterations as needed, until the graph converges.
func (g *Graph64) Rank(α, ε float64, callback func(id uint64, rank float64)) {
	Δ := float64(1.0)
	nodes := g.nodes
	inverse := 1 / float64(len(nodes))

	// Normalize all the edge weights so that their sum amounts to 1.
	if g.Verbose {
		fmt.Println("normalize...")
	}
	done := make(chan bool, 8)
	normalize := func(node *Node64) {
		if outbound := node.outbound; outbound > 0 {
			for target := range node.edges {
				node.edges[target] /= outbound
			}
		}
		done <- true
	}
	i, flight := 0, 0
	for i < len(nodes) && flight < NumCPU {
		go normalize(&nodes[i])
		flight++
		i++
	}
	for i < len(nodes) {
		<-done
		flight--
		go normalize(&nodes[i])
		flight++
		i++
	}
	for j := 0; j < flight; j++ {
		<-done
	}

	if g.Verbose {
		fmt.Println("initialize...")
	}
	leak := float64(0)

	a, b := 0, 1
	for source := range nodes {
		nodes[source].weight[a] = inverse

		if nodes[source].outbound == 0 {
			leak += inverse
		}
	}

	update := func(adjustment float64, node *Node64) {
		node.RLock()
		aa := α * node.weight[a]
		node.RUnlock()
		for target, weight := range node.edges {
			nodes[target].Lock()
			nodes[target].weight[b] += aa * weight
			nodes[target].Unlock()
		}
		node.Lock()
		bb := node.weight[b]
		node.weight[b] = bb + adjustment
		node.Unlock()
		done <- true
	}
	for Δ > ε {
		if g.Verbose {
			fmt.Println("updating...")
		}
		adjustment := (1-α)*inverse + α*leak*inverse
		i, flight := 0, 0
		for i < len(nodes) && flight < NumCPU {
			go update(adjustment, &nodes[i])
			flight++
			i++
		}
		for i < len(nodes) {
			<-done
			flight--
			go update(adjustment, &nodes[i])
			flight++
			i++
		}
		for j := 0; j < flight; j++ {
			<-done
		}

		if g.Verbose {
			fmt.Println("computing delta...")
		}
		Δ, leak = 0, 0
		for source := range nodes {
			node := &nodes[source]
			aa, bb := node.weight[a], node.weight[b]
			if difference := aa - bb; difference < 0 {
				Δ -= difference
			} else {
				Δ += difference
			}

			if node.outbound == 0 {
				leak += bb
			}
			nodes[source].weight[a] = 0
		}

		a, b = b, a

		if g.Verbose {
			fmt.Println(Δ, ε)
		}
	}

	for key, value := range g.index {
		callback(key, nodes[value].weight[a])
	}
}

// Reset clears all the current graph data.
func (g *Graph64) Reset(size ...int) {
	capacity := 8
	if len(size) == 1 {
		capacity = size[0]
	}
	g.count = 0
	g.index = make(map[uint64]uint, capacity)
	g.nodes = make([]Node64, 0, capacity)
}
