package voxels

type concavityChecker struct {
	hullPath  Path
	fullPath  Path
	finalPath Path
}

func correctConcavity(hullPath, fullPath Path, threshold float64) Path {
	if len(hullPath) <= 3 {
		return hullPath
	}

	cc := &concavityChecker{
		hullPath:  hullPath,
		fullPath:  fullPath,
		finalPath: make(Path, 0, len(fullPath)),
	}
	inner := 1
	for outer, label := range hullPath {
		if outer < 1 {
			cc.finalPath = append(cc.finalPath, label)
			continue
		}

		inner = cc.check(inner, outer, threshold)
	}

	return cc.finalPath
}

func (cc *concavityChecker) check(inner, outer int, threshold float64) int {
	label2 := cc.hullPath[outer]
	p2 := toXY(label2)

	for i, label0 := range cc.fullPath[inner:] {
		if label0 == label2 {
			cc.finalPath = append(cc.finalPath, label0)
			return inner + i + 1
		}

		// log.Printf("len(cc.finalPath)=%v", len(cc.finalPath))
		label1 := cc.finalPath[len(cc.finalPath)-1]
		error := distanceUsingLabels(label1, label0, p2)
		// log.Printf("distanceUsingLabels(%q,%q,%q) = %v", label1, label0, label2, error)
		if error >= threshold {
			cc.finalPath = append(cc.finalPath, label0)
		}
	}
	return inner
}

func distanceUsingLabels(l1, l0 string, p2 *keyWithAngle) float64 {
	p1 := toXY(l1)
	p0 := toXY(l0)
	return distance(p1, p0, p2)
}

func toXY(label string) *keyWithAngle {
	x, y := parseKey(label)
	return &keyWithAngle{key: label, x: x, y: y}
}
