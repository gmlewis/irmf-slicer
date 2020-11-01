package voxels

import (
	"log"
	"sort"
)

// Path represents a trace around the edges of a component.
// Each string in the path represents a connection at the upper left hand corner
// of the provided label (in u,v space). Each label is the same as labelKey.
type Path []string

func sortEdges(edges Outline) []string {
	keys := make([]string, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	log.Printf("keys=%#v", keys)
	return keys
}

func edgesToPaths(edges Outline) []Path {
	keys := sortEdges(edges)
	if len(keys) == 0 {
		return nil
	}

	var result []Path
	currentPath := Path{keys[0]}
	lastEdgeKey := keys[0]
	if edge := edges[lastEdgeKey]; !edge.HasTop() {
		log.Fatalf("unexpected starting edge at %q: %v", keys[0], edge)
	}

	checkPathCompletion := func() {
		if currentPath[len(currentPath)-1] == currentPath[0] {
			log.Printf("Completed path, length=%v: %#v", len(currentPath), currentPath)
			result = append(result, currentPath)
			currentPath = nil
			keys = sortEdges(edges)
			if len(keys) > 0 {
				edge := edges[keys[0]]
				if edge != Bottom {
					log.Fatalf("unexpected edge at %q: %v", keys[0], edge)
				}
				x, y := parseKey(keys[0])
				lastEdgeKey = labelKey(x, y+1)
				currentPath = Path{lastEdgeKey}
			}
		}
	}

	lastEdge := Top

	checkTop := func(edge Edge, x, y int) bool {
		if !edge.HasTop() {
			return false
		}
		lastEdge = Top
		edge = edge.RemoveTop()
		if edge == 0 {
			delete(edges, lastEdgeKey)
		} else {
			edges[lastEdgeKey] = edge
		}
		lastEdgeKey = labelKey(x+1, y)
		currentPath = append(currentPath, lastEdgeKey)
		log.Printf("Added %q as top edge: len(currentPath)=%v", lastEdgeKey, len(currentPath))
		checkPathCompletion()
		return true
	}

	checkLeft := func(edge Edge, x, y int) bool {
		upKey := labelKey(x, y-1)
		edge = edges[upKey]
		if !edge.HasLeft() {
			return false
		}
		lastEdge = Left
		edge = edge.RemoveLeft()
		if edge == 0 {
			delete(edges, upKey)
		} else {
			edges[upKey] = edge
		}
		lastEdgeKey = upKey
		currentPath = append(currentPath, lastEdgeKey)
		log.Printf("Added %q as left edge: len(currentPath)=%v", lastEdgeKey, len(currentPath))
		checkPathCompletion()
		return true
	}

	checkBottom := func(edge Edge, x, y int) bool {
		upLeftKey := labelKey(x-1, y-1)
		edge = edges[upLeftKey]
		if !edge.HasBottom() {
			return false
		}
		lastEdge = Bottom
		edge = edge.RemoveBottom()
		if edge == 0 {
			delete(edges, upLeftKey)
		} else {
			edges[upLeftKey] = edge
		}
		lastEdgeKey = labelKey(x-1, y)
		currentPath = append(currentPath, lastEdgeKey)
		log.Printf("Added %q as bottom edge: len(currentPath)=%v", lastEdgeKey, len(currentPath))
		checkPathCompletion()
		return true
	}

	checkRight := func(edge Edge, x, y int) bool {
		leftKey := labelKey(x-1, y)
		edge = edges[leftKey]
		if !edge.HasRight() {
			return false
		}
		lastEdge = Right
		edge = edge.RemoveRight()
		if edge == 0 {
			delete(edges, leftKey)
		} else {
			edges[leftKey] = edge
		}
		lastEdgeKey = labelKey(x, y+1)
		currentPath = append(currentPath, lastEdgeKey)
		log.Printf("Added %q as right edge: len(currentPath)=%v", lastEdgeKey, len(currentPath))
		checkPathCompletion()
		return true
	}

	for len(edges) > 0 {
		edge := edges[lastEdgeKey]
		x, y := parseKey(lastEdgeKey)
		log.Printf("edges[%q]=%v (%v,%v)", lastEdgeKey, edge, x, y)

		if lastEdge != Bottom {
			if checkTop(edge, x, y) {
				continue
			}
			if checkLeft(edge, x, y) {
				continue
			}
			if checkBottom(edge, x, y) {
				continue
			}
			if checkRight(edge, x, y) {
				continue
			}
		} else {
			if checkRight(edge, x, y) {
				continue
			}
			if checkBottom(edge, x, y) {
				continue
			}
			if checkLeft(edge, x, y) {
				continue
			}
			if checkTop(edge, x, y) {
				continue
			}
		}

		log.Fatalf("should not reach here")
	}

	if len(currentPath) > 0 {
		result = append(result, currentPath)
	}

	return result
}
