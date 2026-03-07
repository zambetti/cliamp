package ui

import (
	"math"
	"strings"
)

// renderSnow draws falling snowflakes using Braille dots. A scrolling random
// field creates the illusion of individual flakes drifting downward at varying
// speeds per column. Wind oscillates laterally, with deeper flakes drifting
// further to simulate a natural arc. Band energy controls flake density —
// louder music produces heavier snowfall.
func (v *Visualizer) renderSnow(bands [numBands]float64) string {
	height := v.Rows
	dotRows := height * 4
	dotCols := panelWidth * 2

	var totalEnergy float64
	for _, e := range bands {
		totalEnergy += e
	}
	avgEnergy := totalEnergy / float64(numBands)

	// Wind: gentle oscillation, amplitude grows with energy.
	windStrength := math.Sin(float64(v.frame)*0.03) * (0.5 + avgEnergy*2.0)

	lines := make([]string, height)

	for row := range height {
		var sb strings.Builder

		for ch := range panelWidth {
			var braille rune = '\u2800'

			for dr := range 4 {
				for dc := range 2 {
					dotRow := row*4 + dr
					dotCol := ch*2 + dc

					// Map dot column to the nearest frequency band.
					bandIdx := dotCol * numBands / dotCols
					if bandIdx >= numBands {
						bandIdx = numBands - 1
					}
					energy := bands[bandIdx]

					// Per-column fall speed (1-4), derived from column position
					// so each column's flakes fall at a consistent, unique rate.
					colSpeed := 1 + int(uint64(dotCol)*7919%4)

					// Scroll the row lookup upward over time = flakes fall down.
					adjustedRow := dotRow - int(v.frame)*colSpeed/3

					// Wind drift: proportional to depth (rows from top), so
					// flakes near the top barely move while lower ones arc further.
					windDrift := int(windStrength * float64(dotRow) / float64(max(1, dotRows)))
					adjustedCol := dotCol - windDrift

					// Deterministic hash at the scrolled position. Using frame=0
					// keeps the random field stable — motion comes from the scroll.
					h := scatterHash(bandIdx, adjustedRow, adjustedCol, 0)

					// Sparse threshold: a thin base layer + energy-driven density.
					threshold := 0.015 + energy*0.05
					if h < threshold {
						braille |= brailleBit[dr][dc]
					}
				}
			}

			// Bright at top (fresh flakes catching light), dimmer at bottom.
			rowNorm := float64(height-1-row) / float64(height)
			sb.WriteString(specStyle(rowNorm).Render(string(braille)))
		}

		lines[row] = sb.String()
	}

	return strings.Join(lines, "\n")
}
