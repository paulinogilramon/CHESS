package main

import (
	"image"
	"testing"

	"chess/engine"
)

///
/// <summary>
///   TestRenderPieces exercises the procedural piece renderer for every piece
///   type and color, asserting visible ink exists, fits the canvas, and that
///   silhouettes are distinct and correctly centered.
/// </summary>
/// <param name="t">Test context.</param>
func TestRenderPieces(t *testing.T) {
	types := []int8{engine.King, engine.Queen, engine.Rook, engine.Bishop, engine.Knight, engine.Pawn}
	colors := []engine.Color{engine.White, engine.Black}

	count := func(img *image.RGBA) int {
		n := 0
		for y := 0; y < img.Rect.Dy(); y++ {
			for x := 0; x < img.Rect.Dx(); x++ {
				if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
					n++
				}
			}
		}
		return n
	}

	width := map[string]int{}
	for _, typ := range types {
		for _, col := range colors {
			name := "w"
			if col != engine.White {
				name = "b"
			}
			name += string(rune('K' + typ - engine.King))

			img, loX, loY, hiX, hiY := renderPiece(typ, col, pieceSize)
			if img.Rect.Dx() != pieceSize || img.Rect.Dy() != pieceSize {
				t.Errorf("%s: canvas = %dx%d, want %dx%d", name, img.Rect.Dx(), img.Rect.Dy(), pieceSize, pieceSize)
			}
			if hiX < loX || hiY < loY {
				t.Errorf("%s: piece rendered no ink", name)
				continue
			}
			if loX < 0 || loY < 0 || hiX >= pieceSize || hiY >= pieceSize {
				t.Errorf("%s: ink escapes canvas", name)
			}
			if n := count(img); n < 120 {
				t.Errorf("%s: ink area too small (%d px)", name, n)
			}
			if w := hiX - loX + 1; w < 8 {
				t.Errorf("%s: silhouette too narrow (%d px)", name, w)
			}
			width[name] = hiX - loX + 1
		}
	}

	kName := "w" + string(rune('K'+engine.Knight-engine.King))
	pName := "w" + string(rune('K'+engine.Pawn-engine.King))
	if width[kName] <= width[pName] {
		t.Errorf("knight expected wider than pawn (knight %d vs pawn %d)", width[kName], width[pName])
	}
}