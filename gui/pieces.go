package main

import (
	"image"
	"image/color"

	"chess/engine"
)

///
/// <summary>
///   pieceSize is the side of the square canvas each piece is rendered on.
/// </summary>
const pieceSize = 44

///
/// <summary>
///   vec is a point in the normalized piece drawing space (0..1, y down).
/// </summary>
type vec struct {
	x, y float64
}

///
/// <summary>
///   insideRect reports whether a normalized point lies in an axis-aligned box.
/// </summary>
/// <param name="px">Point x.</param>
/// <param name="py">Point y.</param>
/// <param name="x0">Box left.</param>
/// <param name="y0">Box top.</param>
/// <param name="x1">Box right.</param>
/// <param name="y1">Box bottom.</param>
/// <returns>True when inside.</returns>
func insideRect(px, py, x0, y0, x1, y1 float64) bool {
	return px >= x0 && px <= x1 && py >= y0 && py <= y1
}

///
/// <summary>
///   insideCircle reports whether a normalized point lies in a disc.
/// </summary>
/// <param name="px">Point x.</param>
/// <param name="py">Point y.</param>
/// <param name="cx">Circle center x.</param>
/// <param name="cy">Circle center y.</param>
/// <param name="r">Circle radius.</param>
/// <returns>True when inside.</returns>
func insideCircle(px, py, cx, cy, r float64) bool {
	dx, dy := px-cx, py-cy
	return dx*dx+dy*dy <= r*r
}

///
/// <summary>
///   insideRoundRect reports whether a normalized point lies in a rounded box.
/// </summary>
/// <param name="px">Point x.</param>
/// <param name="py">Point y.</param>
/// <param name="x0">Box left.</param>
/// <param name="y0">Box top.</param>
/// <param name="x1">Box right.</param>
/// <param name="y1">Box bottom.</param>
/// <param name="r">Corner radius.</param>
/// <returns>True when inside.</returns>
func insideRoundRect(px, py, x0, y0, x1, y1, r float64) bool {
	qx := px
	switch {
	case qx < x0+r:
		qx = x0 + r
	case qx > x1-r:
		qx = x1 - r
	}
	qy := py
	switch {
	case qy < y0+r:
		qy = y0 + r
	case qy > y1-r:
		qy = y1 - r
	}
	dx, dy := px-qx, py-qy
	return dx*dx+dy*dy <= r*r
}

///
/// <summary>
///   cross computes the 2D cross product of OA x OB.
/// </summary>
/// <param name="o">Origin.</param>
/// <param name="a">First arm.</param>
/// <param name="b">Second arm.</param>
/// <returns>The signed cross product.</returns>
func cross(o, a, b vec) float64 {
	return (a.x-o.x)*(b.y-o.y) - (a.y-o.y)*(b.x-o.x)
}

///
/// <summary>
///   insideTri reports whether a normalized point lies in a triangle.
/// </summary>
/// <param name="px">Point x.</param>
/// <param name="py">Point y.</param>
/// <param name="a">Triangle vertex.</param>
/// <param name="b">Triangle vertex.</param>
/// <param name="c">Triangle vertex.</param>
/// <returns>True when inside.</returns>
func insideTri(px, py float64, a, b, c vec) bool {
	p := vec{px, py}
	sa := cross(a, b, p) >= 0
	sb := cross(b, c, p) >= 0
	sc := cross(c, a, p) >= 0
	return sa && sb && sc
}

///
/// <summary>
///   insidePoly reports whether a normalized point lies in a polygon using the
///   even-odd rule.
/// </summary>
/// <param name="px">Point x.</param>
/// <param name="py">Point y.</param>
/// <param name="pts">Polygon vertices in order.</param>
/// <returns>True when inside.</returns>
func insidePoly(px, py float64, pts []vec) bool {
	inside := false
	n := len(pts)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		a, b := pts[i], pts[j]
		if (a.y > py) != (b.y > py) {
			xi := (b.x-a.x)*(py-a.y)/(b.y-a.y) + a.x
			if px < xi {
				inside = !inside
			}
		}
	}
	return inside
}

///
/// <summary>
///   or folds several predicates into one that is true when any is true.
/// </summary>
/// <param name="fs">Predicates over normalized coordinates.</param>
/// <returns>The combined predicate.</returns>
func or(fs ...func(x, y float64) bool) func(x, y float64) bool {
	return func(x, y float64) bool {
		for _, f := range fs {
			if f(x, y) {
				return true
			}
		}
		return false
	}
}

///
/// <summary>
///   shapeFor returns the fill and accent predicates for a piece type. Accents
///   are painted with the outline color to add internal detail.
/// </summary>
/// <param name="typ">Piece type.</param>
/// <returns>The fill and accent predicates.</returns>
func shapeFor(typ int8) (fill, accent func(x, y float64) bool) {
	base := func() func(x, y float64) bool {
		return func(x, y float64) bool {
			return insidePoly(x, y, []vec{
				{0.33, 0.885}, {0.67, 0.885}, {0.61, 1.0}, {0.39, 1.0},
			})
		}
	}
	switch typ {
	case engine.King:
		fill = or(
			base(),
			func(x, y float64) bool { return insideRoundRect(x, y, 0.46, 0.40, 0.54, 0.94, 0.02) },
			func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.50, 0.10) },
			func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.50, 0.10) },
			func(x, y float64) bool {
				return insidePoly(x, y, []vec{
					{0.135, 0.60}, {0.155, 0.365}, {0.255, 0.50}, {0.355, 0.30},
					{0.50, 0.47}, {0.645, 0.30}, {0.745, 0.50}, {0.845, 0.365}, {0.865, 0.60},
				})
			},
			func(x, y float64) bool { return insideRect(x, y, 0.472, 0.045, 0.528, 0.28) },
			func(x, y float64) bool { return insideRect(x, y, 0.445, 0.105, 0.555, 0.155) },
		)
		accent = func(x, y float64) bool { return false }
	case engine.Queen:
		fill = or(
			base(),
			func(x, y float64) bool { return insideRoundRect(x, y, 0.465, 0.36, 0.535, 0.94, 0.02) },
			func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.42, 0.085) },
			func(x, y float64) bool {
				return insidePoly(x, y, []vec{
					{0.15, 0.52}, {0.165, 0.245}, {0.25, 0.42}, {0.325, 0.30},
					{0.425, 0.42}, {0.50, 0.13}, {0.575, 0.42}, {0.675, 0.30},
					{0.75, 0.42}, {0.835, 0.245}, {0.85, 0.52},
				})
			},
			func(x, y float64) bool { return insideCircle(x, y, 0.16, 0.22, 0.05) },
			func(x, y float64) bool { return insideCircle(x, y, 0.325, 0.27, 0.05) },
			func(x, y float64) bool { return insideCircle(x, y, 0.50, 0.10, 0.05) },
			func(x, y float64) bool { return insideCircle(x, y, 0.675, 0.27, 0.05) },
			func(x, y float64) bool { return insideCircle(x, y, 0.84, 0.22, 0.05) },
		)
		accent = func(x, y float64) bool { return insideRect(x, y, 0.475, 0.50, 0.525, 0.55) }
	case engine.Rook:
		fill = or(
			base(),
			func(x, y float64) bool { return insideRect(x, y, 0.40, 0.12, 0.46, 0.28) },
			func(x, y float64) bool { return insideRect(x, y, 0.464, 0.12, 0.536, 0.28) },
			func(x, y float64) bool { return insideRect(x, y, 0.54, 0.12, 0.60, 0.28) },
			func(x, y float64) bool { return insideRect(x, y, 0.412, 0.28, 0.588, 0.33) },
			func(x, y float64) bool { return insideRoundRect(x, y, 0.435, 0.33, 0.565, 0.905, 0.025) },
			func(x, y float64) bool { return insideRect(x, y, 0.43, 0.50, 0.57, 0.56) },
		)
		accent = func(x, y float64) bool { return insideRect(x, y, 0.43, 0.285, 0.57, 0.31) }
	case engine.Bishop:
		fill = or(
			base(),
			func(x, y float64) bool {
				return insidePoly(x, y, []vec{
					{0.50, 0.10}, {0.275, 0.50}, {0.395, 0.53}, {0.395, 0.615},
					{0.605, 0.615}, {0.605, 0.53}, {0.725, 0.50},
				})
			},
			func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.075, 0.07) },
			func(x, y float64) bool { return insideRoundRect(x, y, 0.455, 0.44, 0.545, 1.0, 0.015) },
			func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.46, 0.075) },
			func(x, y float64) bool { return insideRect(x, y, 0.40, 0.615, 0.60, 0.655) },
		)
		accent = or(
			func(x, y float64) bool {
				return insidePoly(x, y, []vec{
					{0.48, 0.17}, {0.52, 0.17}, {0.525, 0.46}, {0.475, 0.46},
				})
			},
			func(x, y float64) bool { return insideRect(x, y, 0.435, 0.52, 0.565, 0.56) },
		)
	case engine.Knight:
		fill = or(
			base(),
			func(x, y float64) bool { return insideCircle(x, y, 0.52, 0.44, 0.20) },
			func(x, y float64) bool { return insideRoundRect(x, y, 0.51, 0.335, 0.885, 0.50, 0.055) },
			func(x, y float64) bool { return insideRoundRect(x, y, 0.47, 0.50, 0.835, 0.63, 0.045) },
			func(x, y float64) bool { return insideTri(x, y, vec{0.435, 0.235}, vec{0.50, 0.055}, vec{0.575, 0.21}) },
			func(x, y float64) bool { return insideTri(x, y, vec{0.30, 0.66}, vec{0.565, 0.63}, vec{0.28, 0.96}) },
		)
		accent = or(
			func(x, y float64) bool { return insideCircle(x, y, 0.585, 0.43, 0.032) },
			func(x, y float64) bool { return insideCircle(x, y, 0.84, 0.42, 0.026) },
			func(x, y float64) bool { return insideRect(x, y, 0.585, 0.545, 0.805, 0.56) },
		)
	case engine.Pawn:
		fill = or(
			base(),
			func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.415, 0.235) },
			func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.60, 0.126) },
			func(x, y float64) bool { return insideRoundRect(x, y, 0.414, 0.59, 0.586, 0.91, 0.02) },
		)
		accent = func(x, y float64) bool { return false }
	default:
		fill = func(x, y float64) bool { return insideCircle(x, y, 0.5, 0.5, 0.35) }
		accent = func(x, y float64) bool { return false }
	}
	return fill, accent
}

///
/// <summary>
///   piecePalette returns the fill and outline colors for a piece color.
/// </summary>
/// <param name="col">Piece color.</param>
/// <returns>The fill and outline colors.</returns>
func piecePalette(col engine.Color) (fill, outline color.RGBA) {
	if col == engine.White {
		return color.RGBA{0xFB, 0xF8, 0xF2, 0xFF}, color.RGBA{0x15, 0x17, 0x1B, 0xFF}
	}
	return color.RGBA{0x2B, 0x31, 0x3A, 0xFF}, color.RGBA{0x0A, 0x0C, 0x10, 0xFF}
}

///
/// <summary>
///   renderPiece rasterizes a procedurally drawn chess piece onto a square
///   canvas with a thin outline, returning the image and the bounding box of
///   its visible ink.
/// </summary>
/// <param name="typ">Piece type.</param>
/// <param name="col">Piece color.</param>
/// <param name="size">Canvas side length in pixels.</param>
/// <returns>The alpha-blended image and ink bounds.</returns>
func renderPiece(typ int8, col engine.Color, size int) (*image.RGBA, int, int, int, int) {
	fill, accent := shapeFor(typ)
	fillClr, outClr := piecePalette(col)

	k := 3
	cov := make([][]float64, size)
	acc := make([][]float64, size)
	for py := 0; py < size; py++ {
		cov[py] = make([]float64, size)
		acc[py] = make([]float64, size)
		for px := 0; px < size; px++ {
			c, a := 0.0, 0.0
			for i := 0; i < k; i++ {
				for j := 0; j < k; j++ {
					nx := (float64(px) + (float64(j)+0.5)/float64(k)) / float64(size)
					ny := (float64(py) + (float64(i)+0.5)/float64(k)) / float64(size)
					f := fill(nx, ny)
					ac := accent(nx, ny)
					if f || ac {
						c++
					}
					if ac {
						a++
					}
				}
			}
			cov[py][px] = c / float64(k*k)
			acc[py][px] = a / float64(k*k)
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	loX, loY := size, size
	hiX, hiY := -1, -1
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			c := cov[py][px]
			cr, alpha := fillClr, 0.0
			if c > 0 {
				alpha = c
				if acc[py][px] >= 0.5 {
					cr = outClr
				}
			} else {
				d := 3
				for dy2 := -2; dy2 <= 2; dy2++ {
					for dx2 := -2; dx2 <= 2; dx2++ {
						qx, qy := px+dx2, py+dy2
						if qx < 0 || qy < 0 || qx >= size || qy >= size || cov[qy][qx] <= 0 {
							continue
						}
						adx := dx2
						if adx < 0 {
							adx = -adx
						}
						ady := dy2
						if ady < 0 {
							ady = -ady
						}
						dd := adx
						if ady > dd {
							dd = ady
						}
						if dd < d {
							d = dd
						}
					}
				}
				if d == 1 {
					cr = outClr
					alpha = 0.85
				} else if d == 2 {
					cr = outClr
					alpha = 0.45
				}
			}
			if alpha <= 0 {
				continue
			}
			a8 := uint8(alpha*255 + 0.5)
			img.SetRGBA(px, py, color.RGBA{cr.R, cr.G, cr.B, a8})
			if px < loX {
				loX = px
			}
			if px > hiX {
				hiX = px
			}
			if py < loY {
				loY = py
			}
			if py > hiY {
				hiY = py
			}
		}
	}
	return img, loX, loY, hiX, hiY
}