package graph

// stronglyConnected assigns a Tarjan component to every node using an explicit DFS stack.
func stronglyConnected(edges [][]int) []int {
	type frame struct {
		node int
		next int
	}
	n := len(edges)
	index, low, component := make([]int, n), make([]int, n), make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	var dfs []frame
	var stack []int
	var tick, nextComponent int
	visit := func(node int) {
		index[node], low[node] = tick, tick
		tick++
		stack = append(stack, node)
		onStack[node] = true
		dfs = append(dfs, frame{node: node})
	}

	for start := range edges {
		if index[start] != -1 {
			continue
		}
		visit(start)
		for len(dfs) != 0 {
			top := &dfs[len(dfs)-1]
			node := top.node
			if top.next < len(edges[node]) {
				child := edges[node][top.next]
				top.next++
				if index[child] == -1 {
					visit(child)
				} else if onStack[child] {
					low[node] = min(low[node], index[child])
				}
				continue
			}
			dfs = dfs[:len(dfs)-1]
			if low[node] == index[node] {
				for {
					last := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					component[last] = nextComponent
					onStack[last] = false
					if last == node {
						break
					}
				}
				nextComponent++
			}
			if len(dfs) != 0 {
				parent := dfs[len(dfs)-1].node
				low[parent] = min(low[parent], low[node])
			}
		}
	}
	return component
}
