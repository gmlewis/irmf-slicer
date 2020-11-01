package voxels

// Edge represents the edges of a voxel.
type Edge byte

const (
	Top Edge = 1 << iota
	Left
	Bottom
	Right
)

func (e Edge) AddTop() Edge       { return e | Top }
func (e Edge) AddLeft() Edge      { return e | Left }
func (e Edge) AddBottom() Edge    { return e | Bottom }
func (e Edge) AddRight() Edge     { return e | Right }
func (e Edge) HasTop() bool       { return e&Top == Top }
func (e Edge) HasLeft() bool      { return e&Left == Left }
func (e Edge) HasBottom() bool    { return e&Bottom == Bottom }
func (e Edge) HasRight() bool     { return e&Right == Right }
func (e Edge) RemoveTop() Edge    { return e &^ Top }
func (e Edge) RemoveLeft() Edge   { return e &^ Left }
func (e Edge) RemoveBottom() Edge { return e &^ Bottom }
func (e Edge) RemoveRight() Edge  { return e &^ Right }

// Outline represents a continuous path around a solid region of a slice.
type Outline map[string]Edge

func findEdges(label *Label) Outline {
	edges := Outline{}
	// First, set all the edge flags.
	allEdges := Top | Left | Bottom | Right
	for key := range label.pixels {
		edges[key] = allEdges
	}

	// Next, wherever neightbors have top+bottom or left+right, remove edges.
	toDelete := map[string]struct{}{}
	for key, edge := range edges {
		x, y := parseKey(key)

		if edge.HasTop() {
			upKey := labelKey(x, y-1)
			if up, ok := edges[upKey]; ok && up.HasBottom() {
				edges[key] = edges[key].RemoveTop()
				if edges[key] == 0 {
					toDelete[key] = struct{}{}
				}
				edges[upKey] = edges[upKey].RemoveBottom()
				if edges[upKey] == 0 {
					toDelete[upKey] = struct{}{}
				}
			}
		}

		if edge.HasLeft() {
			leftKey := labelKey(x-1, y)
			if left, ok := edges[leftKey]; ok && left.HasRight() {
				edges[key] = edges[key].RemoveLeft()
				if edges[key] == 0 {
					toDelete[key] = struct{}{}
				}
				edges[leftKey] = edges[leftKey].RemoveRight()
				if edges[leftKey] == 0 {
					toDelete[leftKey] = struct{}{}
				}
			}
		}

		if edge.HasBottom() {
			bottomKey := labelKey(x, y+1)
			if bottom, ok := edges[bottomKey]; ok && bottom.HasTop() {
				edges[key] = edges[key].RemoveBottom()
				if edges[key] == 0 {
					toDelete[key] = struct{}{}
				}
				edges[bottomKey] = edges[bottomKey].RemoveTop()
				if edges[bottomKey] == 0 {
					toDelete[bottomKey] = struct{}{}
				}
			}
		}

		if edge.HasRight() {
			rightKey := labelKey(x+1, y)
			if right, ok := edges[rightKey]; ok && right.HasLeft() {
				edges[key] = edges[key].RemoveRight()
				if edges[key] == 0 {
					toDelete[key] = struct{}{}
				}
				edges[rightKey] = edges[rightKey].RemoveLeft()
				if edges[rightKey] == 0 {
					toDelete[rightKey] = struct{}{}
				}
			}
		}
	}

	for k := range toDelete {
		delete(edges, k)
	}

	return edges
}
