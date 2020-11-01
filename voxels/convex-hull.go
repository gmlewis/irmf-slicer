package voxels

import (
	"math"
	"sort"
)

func convexHull(path Path, reverse bool) Path {
	if len(path) == 0 {
		return nil
	}
	pts := sortByAngle(path)

	var stack []*keyWithAngle
	for _, pt := range pts {
		if sl := len(stack); sl >= 2 && ccw(stack[sl-2], stack[sl-1], pt) < 0 {
			// log.Printf("pop stack (%q) due to ccw<0 at i=%v pt=%#v", stack[sl-1].key, i, *pt)
			stack = stack[:sl-1]
		}
		for len(stack) >= 2 && ccw(stack[len(stack)-2], stack[len(stack)-1], pt) <= 0 {
			// log.Printf("pop stack (%q) due to ccw<=0 at i=%v pt=%#v", stack[len(stack)-1].key, i, *pt)
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, pt)
	}

	result := make(Path, 0, len(path))
	for _, pt := range stack {
		result = append(result, pt.key)
	}
	result = append(result, pts[0].key)

	if reverse {
		for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
			result[left], result[right] = result[right], result[left]
		}
	}

	return result
}

type keyWithAngle struct {
	key      string
	x, y     int
	angle    float64
	distance int
}

func ccw(p1, p2, p3 *keyWithAngle) int {
	result := (p2.x-p1.x)*(p3.y-p1.y) - (p2.y-p1.y)*(p3.x-p1.x)
	// log.Printf("ccw (%v,%v)-(%v,%v)-(%v,%v) = %v", p1.x, p1.y, p2.x, p2.y, p3.x, p3.y, result)
	return result
}

func sortByAngle(path Path) []*keyWithAngle {
	if len(path) == 0 {
		return nil
	}
	sort.Strings(path)
	startX, startY := parseKey(path[0])

	angles := make([]*keyWithAngle, 0, len(path))

	for _, key := range path[1:] {
		x, y := parseKey(key)
		angle := math.Atan2(float64(y-startY), float64(x-startX))
		distance := (y-startY)*(y-startY) + (x-startX)*(x-startX)
		angles = append(angles, &keyWithAngle{
			key:      key,
			x:        x,
			y:        y,
			angle:    angle,
			distance: distance,
		})
	}
	sort.Slice(angles, func(a, b int) bool {
		if angles[a].key == path[0] {
			return true
		}
		if angles[b].key == path[0] {
			return false
		}
		if angles[a].angle == angles[b].angle {
			return angles[a].distance < angles[b].distance
		}
		return angles[a].angle < angles[b].angle
	})

	return angles
}
