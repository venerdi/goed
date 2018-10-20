package edGalaxy

import (
	"math"
)

type Point3D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (p *Point3D) Clone() *Point3D {
	return &Point3D{
		X: p.X,
		Y: p.Y,
		Z: p.Z,
	}
}

var Sol *Point3D

func init() {
	Sol = &Point3D{0, 0, 0}
}

func Distance(p1, p2 *Point3D) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	dz := p1.Z - p2.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

func (p1 *Point3D) Distance(p *Point3D) float64 {
	return Distance(p, p1)
}
