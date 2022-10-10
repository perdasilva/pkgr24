package main

import (
	"fmt"

	"github.com/go-air/gini"
	"github.com/go-air/gini/z"
)

func main() {
	g := gini.New()

	const numDevs = 4
	const numWeeks = 4
	const numBDC = 2

	lit := func(week int, slot int, dev int) z.Lit {
		id := (numBDC*week+slot)*numDevs + dev + 1
		// fmt.Printf("%d ", id)
		return z.Var(id).Pos()
	}

	// a literal for each person / bdc slot / week
	for week := 0; week < numWeeks; week++ {
		for slot := 0; slot < numBDC; slot++ {
			for dev := 0; dev < numDevs; dev++ {
				g.Add(lit(week, slot, dev))
			}
			g.Add(0)
		}
	}

	for dev := 0; dev < numDevs; dev++ {
		for week := 0; week < numWeeks; week++ {
			for slot := 0; slot < numBDC; slot++ {
				devIsInSlotOne := lit(week, slot, dev)
				// can't show up in another slot in the same week
				for nextSlot := slot + 1; nextSlot < numBDC; nextSlot++ {
					devIsInNextSlot := lit(week, nextSlot, dev)
					g.Add(devIsInSlotOne.Not())
					g.Add(devIsInNextSlot.Not())
					g.Add(0)
				}

				// can't show up in the following week
				if week < numWeeks-1 {
					for nextSlot := 0; nextSlot < numBDC; nextSlot++ {
						devIsInNextSlot := lit(week+1, nextSlot, dev)
						g.Add(devIsInSlotOne.Not())
						g.Add(devIsInNextSlot.Not())
						g.Add(0)
					}
				}
			}
		}
	}

	if g.Solve() != 1 {
		fmt.Println("unsolvable")
		return
	}

	for week := 0; week < numWeeks; week++ {
		for slot := 0; slot < numBDC; slot++ {
			for dev := 0; dev < numDevs; dev++ {
				m := lit(week, slot, dev)
				if g.Value(m) {
					fmt.Printf("%d", dev+1)
					break
				}
			}
			fmt.Print("\t")
		}
		fmt.Println()
	}
}
