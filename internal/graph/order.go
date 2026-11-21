package graph

import (
	"cmp"
	"container/heap"
	"slices"

	"github.com/pkieltyka/pbrain/internal/item"
)

func oldest(a, b *item.Item) int { return byDate(a, b, false) }
func newest(a, b *item.Item) int { return byDate(a, b, true) }

func byDate(a, b *item.Item, descending bool) int {
	if a.Header.CreatedKnown != b.Header.CreatedKnown {
		if a.Header.CreatedKnown {
			return -1
		}
		return 1
	}
	if a.Header.CreatedKnown {
		order := a.Header.Created.Compare(b.Header.Created)
		if order != 0 {
			if descending {
				return -order
			}
			return order
		}
	}
	return cmp.Or(cmp.Compare(a.Key, b.Key), cmp.Compare(a.Path, b.Path))
}

func (g *Graph) nodeOrder(a, b int) int {
	return oldest(g.items[a], g.items[b])
}

func (g *Graph) deriveReplacementFacts() {
	n := len(g.items)
	waiting := make([]int, n)
	order := make([]int, 0, n) // topological order with replacers after their targets
	for i := range g.items {
		waiting[i] = len(g.out[i])
		if waiting[i] == 0 {
			order = append(order, i)
		}
		slices.SortFunc(g.out[i], g.nodeOrder)
		slices.SortFunc(g.in[i], g.nodeOrder)
	}
	for pos := 0; pos < len(order); pos++ {
		for _, j := range g.in[order[pos]] {
			waiting[j]--
			if waiting[j] == 0 {
				order = append(order, j)
			}
		}
	}
	for pos := len(order) - 1; pos >= 0; pos-- {
		i := order[pos]
		switch len(g.in[i]) {
		case 0:
			g.succ[i] = order[pos : pos+1 : pos+1]
		case 1:
			g.succ[i] = g.succ[g.in[i][0]]
		default:
			seen := make(map[int]struct{})
			for _, replacer := range g.in[i] {
				for _, successor := range g.succ[replacer] {
					if _, exists := seen[successor]; !exists {
						seen[successor] = struct{}{}
						g.succ[i] = append(g.succ[i], successor)
					}
				}
			}
			slices.SortFunc(g.succ[i], g.nodeOrder)
		}
		if len(g.succ[i]) > 1 {
			g.facts = append(g.facts, LinkFinding{
				Code: "replace_fork", Item: g.items[i], Field: FieldReplaces, Successors: g.references(g.succ[i]),
			})
		}
	}
	g.indexLineages()
}

func (g *Graph) indexLineages() {
	n := len(g.items)
	parent, size := make([]int, n), make([]int, n)
	for i := range parent {
		parent[i], size[i] = i, 1
	}
	root := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i, targets := range g.out {
		for _, j := range targets {
			a, b := root(i), root(j)
			if a == b {
				continue
			}
			if size[a] < size[b] {
				a, b = b, a
			}
			parent[b] = a
			size[a] += size[b]
		}
	}
	g.group = make([]int, n)
	g.groups = make([][]int, n)
	for i := range g.items {
		a := root(i)
		g.group[i] = a
		g.groups[a] = append(g.groups[a], i)
	}
}

// History lists the connected lineage with targets before replacers.
func (g *Graph) History(it *item.Item) []*item.Item {
	i, ok := g.nodes[it]
	if !ok {
		return nil
	}
	group := g.groups[g.group[i]]
	if len(group) == 1 {
		return []*item.Item{it}
	}
	remaining := make(map[int]int, len(group))
	ready := &historyQueue{graph: g}
	for _, node := range group {
		remaining[node] = len(g.out[node])
		if remaining[node] == 0 {
			ready.nodes = append(ready.nodes, node)
		}
	}
	heap.Init(ready)
	out := make([]*item.Item, 0, len(group))
	for ready.Len() > 0 {
		node := heap.Pop(ready).(int)
		out = append(out, g.items[node])
		for _, next := range g.in[node] {
			remaining[next]--
			if remaining[next] == 0 {
				heap.Push(ready, next)
			}
		}
	}
	return out
}

type historyQueue struct {
	graph *Graph
	nodes []int
}

func (q *historyQueue) Len() int           { return len(q.nodes) }
func (q *historyQueue) Less(i, j int) bool { return q.graph.nodeOrder(q.nodes[i], q.nodes[j]) < 0 }
func (q *historyQueue) Swap(i, j int)      { q.nodes[i], q.nodes[j] = q.nodes[j], q.nodes[i] }
func (q *historyQueue) Push(x any)         { q.nodes = append(q.nodes, x.(int)) }
func (q *historyQueue) Pop() any {
	i := len(q.nodes) - 1
	node := q.nodes[i]
	q.nodes = q.nodes[:i]
	return node
}
