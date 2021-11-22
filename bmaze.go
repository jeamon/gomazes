package main

// This is a small go-based program to generate nice maze using recursive backtracking algorithm.
// It has feature to draw the full maze into ascii format thus allowing user to play over gui.

// Version  : 1.0
// Author   : Jerome AMON
// Created  : 22 November 2021

import (
	//	"fmt"
	"math/rand"
	"strings"
	"time"
	//	"github.com/jroimartin/gocui"
)

// assign the 4 directions code to powers of 2.
// directions are : North, South, East, West.
const (
	N = 1 // N : 0001
	S = 2 // S : 0010
	E = 4 // E : 0100
	W = 8 // W : 1000
)

// moveTo returns coordinates (x,y) based on wanted direction.
func moveTo(posX, posY, direction int) (int, int) {

	switch direction {
	case N:
		return posX, posY - 1
	case S:
		return posX, posY + 1
	case E:
		return posX - 1, posY
	case W:
		return posX + 1, posY
	}

	return -1, -1
}

// shuffleDirection shuffles a given array of 4 directions.
func shuffleDirection(directions *[4]int) {
	rand.Shuffle(len(*directions), func(i, j int) {
		(*directions)[i], (*directions)[j] = (*directions)[j], (*directions)[i]
	})
}

// maze constructs the full maze data.
func createMaze(width, height int) *[][]int {
	// seed sourcing for randomness.
	rand.Seed(time.Now().UnixNano())

	// map the 4 directions code to their opposite direction.
	var oppositeDirections = map[int]int{N: S, S: N, E: W, W: E}

	// choose random list of directions.
	var randomDirections = [4]int{N, S, E, W}
	shuffleDirection(&randomDirections)

	// create 2D maze grid (width x height) with 0 for cells.
	maze := make([][]int, height)
	for y := 0; y < height; y++ {
		maze[y] = make([]int, width)
		for x := 0; x < width; x++ {
			maze[y][x] = 0
		}
	}

	// hold all walls. each wall is made of slice of X / Y / D.
	var walls [][3]int
	// choose a random position as starting cell to dig.
	startX, startY := rand.Intn(width), rand.Intn(height)

	// lets fix entrance & outdoor cell position at top/bottom center.
	inX, inY := width/2, 0
	outX, outY := width/2, (height - 1)

	// add all 4 directions (which constitutes the 4 walls) from the starting cell.
	for _, d := range randomDirections {
		walls = append(walls, [3]int{startX, startY, d})
	}

	// add all 4 directions (which constitutes the 4 walls) from the entrace cell.
	for _, d := range randomDirections {
		//walls = append(walls, [3]int{startX, startY, d})
		walls = append(walls, [3]int{inX, inY, d})
	}

	var paths [][2]int
	addPaths := true

	for len(walls) > 0 {
		x, y, d := getWallInfos(&walls)
		// move from (x,y) towards d direction.
		nX, nY := moveTo(x, y, d)

		// new position (nx, ny) must be valid and unvisited cell (value to 0).
		if nY >= 0 && nY < height && nX >= 0 && nX < width && maze[nY][nX] == 0 {

			// bitwise (OR) between initial cell (x,y) value and direction which returns value of direction
			// so something different than 0. This means there is no more wall toward that direction d.
			// same between new cell (moved to) and opposite/backward direction. just to dig that wall.
			maze[y][x] = maze[y][x] | d
			maze[nY][nX] = maze[nY][nX] | oppositeDirections[d]

			if addPaths {
				paths = append(paths, [2]int{nX, nY})
			}

			if nX == outX && nY == outY {
				// reached the outdoor so open the south wall.
				maze[nY][nX] = maze[nY][nX] | S
				// fmt.Println("reached outdoor position")
				// no need to keep track of path solution.
				addPaths = false
				// shuffle the paths entries.
				rand.Shuffle(len(paths), func(i, j int) {
					paths[i], paths[j] = paths[j], paths[i]
				})
				for _, path := range paths {
					// add all 4 directions (which constitutes the 4 walls) from this cell.
					shuffleDirection(&randomDirections)
					for _, d := range randomDirections {
						walls = append(walls, [3]int{path[0], path[1], d})
					}
				}

				// paths could be dumped or saved to build the solution.
				paths = nil
				continue
			}
			// restart digging walls from entrance position but in another directions.
			if (nX >= (width-4) && nX <= (width-2)) && (nY >= (height-4) && nY <= (height-2)) {
				nX, nY = inX, inY
			}

			// add all 4 directions (which constitutes the 4 walls) from the new cell.
			shuffleDirection(&randomDirections)
			for _, d := range randomDirections {
				walls = append(walls, [3]int{nX, nY, d})
			}
		}
	}
	return &maze
	// displayMaze(&maze, width, height)
}

// getWallInfos retrieves/pop infos of last wall added.
func getWallInfos(walls *[][3]int) (int, int, int) {
	wall := (*walls)[len(*walls)-1]
	x, y, d := wall[0], wall[1], wall[2]
	(*walls) = (*walls)[:len(*walls)-1]
	return x, y, d
}

// formatMaze interprets the slice of slice content into ascii.
func formatMaze(maze *[][]int, width, height int) strings.Builder {

	var mazeFormat strings.Builder

	// display first horizontal line. We use 2 underscores since
	// one will stay above vertical wall (East/West)
	// we fix entrance cell position at top center.
	topLine := " " + strings.Repeat("_", (width*2-1))
	topLine = topLine[:width] + "  " + topLine[(width+1):]
	// fmt.Fprintln(v, topLine)
	mazeFormat.WriteString(topLine)
	mazeFormat.WriteString("\n")

	var rowFormat strings.Builder

	// loop over each row
	for y, row := range *maze {
		// construct each line. Left is a vertical bar.
		rowFormat.WriteRune('|')

		// loop over each cell value.
		for x, cell := range row {

			if (cell & S) != 0 {
				// south wall is opened.
				rowFormat.WriteRune(' ')
			} else {
				// south wall is closed.
				rowFormat.WriteRune('_')
			}

			if (cell & W) != 0 {
				// west wall is opened.
				if ((cell | (*maze)[y][x+1]) & S) != 0 {
					// cell and its west neighnor have their south wall opened.
					rowFormat.WriteRune(' ')
				} else {
					// west neighour cell has its south wall closed.
					rowFormat.WriteRune('_')
				}

			} else {
				// west wall is closed.
				rowFormat.WriteRune('|')
			}
		}

		// draw row and wait for animation.
		// fmt.Println(rowFormat.String())
		mazeFormat.WriteString(rowFormat.String())
		rowFormat.Reset()
		mazeFormat.WriteString("\n")
		// rowFormat.Reset()
		// time.Sleep(time.Duration(200) * time.Millisecond)
	}

	return mazeFormat

}
