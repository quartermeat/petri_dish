package core

import "math"

type Vec3 struct {
	X float64
	Y float64
	Z float64
}

type Mat3 struct {
	M [3][3]float64
}

func (v Vec3) Add(o Vec3) Vec3 {
	return Vec3{X: v.X + o.X, Y: v.Y + o.Y, Z: v.Z + o.Z}
}

func (v Vec3) Sub(o Vec3) Vec3 {
	return Vec3{X: v.X - o.X, Y: v.Y - o.Y, Z: v.Z - o.Z}
}

func (v Vec3) Mul(s float64) Vec3 {
	return Vec3{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

func (v Vec3) Dot(o Vec3) float64 {
	return v.X*o.X + v.Y*o.Y + v.Z*o.Z
}

func (v Vec3) Cross(o Vec3) Vec3 {
	return Vec3{
		X: v.Y*o.Z - v.Z*o.Y,
		Y: v.Z*o.X - v.X*o.Z,
		Z: v.X*o.Y - v.Y*o.X,
	}
}

func (v Vec3) Length() float64 {
	return math.Sqrt(v.Dot(v))
}

func (v Vec3) Normalize() Vec3 {
	length := v.Length()
	if length == 0 {
		return Vec3{}
	}
	return v.Mul(1 / length)
}

func RotateX(v Vec3, angle float64) Vec3 {
	s, c := math.Sin(angle), math.Cos(angle)
	return Vec3{
		X: v.X,
		Y: v.Y*c - v.Z*s,
		Z: v.Y*s + v.Z*c,
	}
}

func RotateY(v Vec3, angle float64) Vec3 {
	s, c := math.Sin(angle), math.Cos(angle)
	return Vec3{
		X: v.X*c + v.Z*s,
		Y: v.Y,
		Z: -v.X*s + v.Z*c,
	}
}

func IdentityMat3() Mat3 {
	return Mat3{
		M: [3][3]float64{
			{1, 0, 0},
			{0, 1, 0},
			{0, 0, 1},
		},
	}
}

func RotationXMat3(angle float64) Mat3 {
	s, c := math.Sin(angle), math.Cos(angle)
	return Mat3{
		M: [3][3]float64{
			{1, 0, 0},
			{0, c, -s},
			{0, s, c},
		},
	}
}

func RotationYMat3(angle float64) Mat3 {
	s, c := math.Sin(angle), math.Cos(angle)
	return Mat3{
		M: [3][3]float64{
			{c, 0, s},
			{0, 1, 0},
			{-s, 0, c},
		},
	}
}

func (m Mat3) Mul(n Mat3) Mat3 {
	var out Mat3
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out.M[r][c] = m.M[r][0]*n.M[0][c] + m.M[r][1]*n.M[1][c] + m.M[r][2]*n.M[2][c]
		}
	}
	return out
}

func (m Mat3) MulVec3(v Vec3) Vec3 {
	return Vec3{
		X: m.M[0][0]*v.X + m.M[0][1]*v.Y + m.M[0][2]*v.Z,
		Y: m.M[1][0]*v.X + m.M[1][1]*v.Y + m.M[1][2]*v.Z,
		Z: m.M[2][0]*v.X + m.M[2][1]*v.Y + m.M[2][2]*v.Z,
	}
}

func (m Mat3) Transpose() Mat3 {
	var out Mat3
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out.M[r][c] = m.M[c][r]
		}
	}
	return out
}

func Clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func SmootherStep(edge0, edge1, x float64) float64 {
	if edge0 == edge1 {
		return 0
	}
	t := Clamp01((x - edge0) / (edge1 - edge0))
	return t * t * t * (t*(t*6-15) + 10)
}
