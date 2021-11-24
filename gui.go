package main

// This is a small go-based program to generate nice maze using recursive backtracking algorithm.
// It will allow a user to use arrow keys to navigate from a single entrance to an exit door.

// Version  : 1.0
// Author   : Jerome AMON
// Created  : 22 November 2021

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jroimartin/gocui"
)

const (
	OUTPUTS  = "outputs"
	HELP     = "help"
	POSITION = "position"
	TIMER    = "timer"
	STATUS   = "status"

	TWIDTH = 11
	PWIDTH = 30
	SWIDTH = 45
)

var (
	// default maze size.
	MAZEHEIGHT int = 10
	MAZEWIDTH  int = 15

	// control timer in updateTimerView.
	stopTimer  = make(chan struct{})
	resetTimer = make(chan struct{})
	// control game status. 1 means paused.
	// 0 means ready to play, 2 means empty.
	// 3 means error so need to restart game.
	pauseGame    = make(chan uint8, 3)
	isGamePaused = false

	cursorPosition = make(chan string, 10)

	// control goroutines.
	exit = make(chan struct{})
	wg   sync.WaitGroup
)

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	f, err := os.OpenFile("logs.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Println("failed to create logs file.")
	}
	defer f.Close()
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(f)

	// setup default minimum maze size.
	if len(os.Args) == 3 {
		if w, err := strconv.Atoi(os.Args[1]); err == nil {
			if w > MAZEWIDTH {
				MAZEWIDTH = w
			}
		}

		if h, err := strconv.Atoi(os.Args[2]); err == nil {
			if h > MAZEHEIGHT {
				MAZEHEIGHT = h
			}
		}
	}

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Highlight = true
	g.SelFgColor = gocui.ColorRed
	g.BgColor = gocui.ColorBlack
	g.FgColor = gocui.ColorWhite
	g.Cursor = false
	g.InputEsc = true
	g.Mouse = false

	g.SetManagerFunc(layout)

	err = g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit)
	if err != nil {
		log.Println("Could not set key binding:", err)
		return
	}

	maxX, maxY := g.Size()

	// Outputs view.
	outputsView, err := g.SetView(OUTPUTS, 0, 0, maxX-1, maxY-4)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create outputs view:", err)
		return
	}
	outputsView.Title = " The Maze "
	outputsView.FgColor = gocui.ColorWhite
	outputsView.SelBgColor = gocui.ColorGreen
	outputsView.SelFgColor = gocui.ColorBlack
	outputsView.Autoscroll = true
	outputsView.Editable = false
	outputsView.Wrap = false

	// Timer view.
	timerView, err := g.SetView(TIMER, 0, maxY-3, TWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create timer view:", err)
		return
	}
	timerView.Title = " Timer "
	timerView.FgColor = gocui.ColorGreen
	timerView.SelBgColor = gocui.ColorBlack
	timerView.SelFgColor = gocui.ColorYellow
	timerView.Editable = false
	timerView.Wrap = false
	fmt.Fprint(timerView, " 00:00:00 ")

	// Position view.
	positionView, err := g.SetView(POSITION, TWIDTH+1, maxY-3, PWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create position view:", err)
		return
	}
	positionView.Title = " Position "
	positionView.FgColor = gocui.ColorGreen
	positionView.SelBgColor = gocui.ColorBlack
	positionView.SelFgColor = gocui.ColorYellow
	positionView.Editable = false
	positionView.Wrap = false

	// Status view.
	statusView, err := g.SetView(STATUS, PWIDTH+1, maxY-3, SWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create status view:", err)
		return
	}
	statusView.Title = " Status "
	statusView.FgColor = gocui.ColorRed
	statusView.SelBgColor = gocui.ColorBlack
	statusView.SelFgColor = gocui.ColorRed
	statusView.Editable = false
	statusView.Wrap = false

	// Help view.
	helpView, err := g.SetView(HELP, SWIDTH+1, maxY-3, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create help view:", err)
		return
	}
	helpView.Title = " Instructions "
	helpView.FgColor = gocui.ColorWhite
	helpView.SelBgColor = gocui.ColorBlack
	helpView.SelFgColor = gocui.ColorYellow
	helpView.Editable = false
	helpView.Wrap = false
	fmt.Fprint(helpView, "CTRL+N [New Maze] - CTRL+R [Reset Game] - CTRL+P [Pause/Resume] - CTRL+Q [Close Maze]")

	// Apply keybindings to program.
	if err = keybindings(g); err != nil {
		log.Panicln(err)
	}

	// move the focus on the jobs list box.
	if _, err = g.SetCurrentView(OUTPUTS); err != nil {
		log.Println("Failed to set focus on outputs view:", err)
		return
	}

	wg.Add(1)
	go updateTimerView(g)

	wg.Add(1)
	go updatePositionView(g, PWIDTH-TWIDTH-1)

	wg.Add(1)
	go updateStatusView(g)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		// close(exit)
		log.Println(err)
	}

	wg.Wait()
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// Outputs view.
	_, err := g.SetView(OUTPUTS, 0, 0, maxX-1, maxY-4)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create outputs view:", err)
		return err
	}

	// Timer view.
	_, err = g.SetView(TIMER, 0, maxY-3, TWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create timer view:", err)
		return err
	}

	// Position view.
	_, err = g.SetView(POSITION, TWIDTH+1, maxY-3, PWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create position view:", err)
		return err
	}

	// Status view.
	_, err = g.SetView(STATUS, PWIDTH+1, maxY-3, SWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create status view:", err)
		return err
	}

	// Help view.
	_, err = g.SetView(HELP, SWIDTH+1, maxY-3, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create help view:", err)
		return err
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	close(exit)
	return gocui.ErrQuit
}

// keybindings binds multiple keys to views.
func keybindings(g *gocui.Gui) error {

	// keys binding on global terminal itself.
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}

	// navigate between views.
	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		return err
	}

	// generate & display new maze when focus on outputs (maze zone).
	if err := g.SetKeybinding(OUTPUTS, gocui.KeyCtrlN, gocui.ModNone, displayNewMaze); err != nil {
		return err
	}

	return nil
}

// displayNewMaze triggers generation of new maze and display it.
func displayNewMaze(g *gocui.Gui, v *gocui.View) error {

	xLines, yLines := v.Size()

	if MAZEWIDTH >= xLines {
		MAZEWIDTH = xLines - 2
	}

	if MAZEHEIGHT >= yLines {
		MAZEHEIGHT = yLines - 2
	}

	maze := createMaze(MAZEWIDTH, MAZEHEIGHT)
	formatMaze := formatMaze(maze, MAZEWIDTH, MAZEHEIGHT)
	maze = nil

	v.Clear()

	if err := createMazeView(g, v, formatMaze); err != nil {
		log.Println("Failed to create & display new maze:", err)
		return err
	}

	formatMaze.Reset()
	// reset and start timer.
	resetTimer <- struct{}{}
	stopTimer <- struct{}{}
	return nil
}

// updateTimerView tracks elapsed time since maze display.
func updateTimerView(g *gocui.Gui) {
	defer wg.Done()
	stop, reset := true, false
	// var currentTime time.Time
	var diff time.Duration
	secsElapsed, hrs, mins, secs := 0, 0, 0, 0

	timerView, err := g.View(TIMER)
	if err != nil {
		log.Println("Failed to get timer view for updating:", err)
		return
	}

	startTime := time.Now()

	for {

		select {

		case <-exit:
			return

		case <-stopTimer:
			stop = !stop

		case <-resetTimer:
			startTime = time.Now()
			reset = !reset
			g.Update(func(g *gocui.Gui) error {
				timerView.Clear()
				fmt.Fprintf(timerView, " 00:00:00 ")
				return nil
			})

		case <-time.After(1 * time.Second):
			if stop {
				continue
			}

			g.Update(func(g *gocui.Gui) error {
				diff = time.Now().Sub(startTime)
				secsElapsed = int(diff.Seconds())
				hrs = int(secsElapsed / 3600)
				mins = int(secsElapsed / 60)
				secs = int(secsElapsed % 60)
				timerView.Clear()
				fmt.Fprintf(timerView, " %02d:%02d:%02d ", hrs, mins, secs)
				return nil
			})
		}
	}
}

// centers a given string within a width by padding.
func center(s string, width int, fill string) string {
	return strings.Repeat(fill, (width-len(s))/2) + s + strings.Repeat(fill, (width-len(s))/2)
}

// updatePositionView displays current cursor coordinates.
func updatePositionView(g *gocui.Gui, pwidth int) {
	defer wg.Done()
	var pos string
	positionView, err := g.View(POSITION)
	if err != nil {
		log.Println("Failed to get position view for updating:", err)
		return
	}

	for {

		select {

		case <-exit:
			return

		case pos = <-cursorPosition:

			g.Update(func(g *gocui.Gui) error {
				positionView.Clear()
				fmt.Fprint(positionView, center(pos, pwidth, " "))
				log.Println(center(pos, pwidth, " "))
				return nil
			})
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// updateStatusView displays current game status.
func updateStatusView(g *gocui.Gui) {
	defer wg.Done()
	var pval uint8

	statusView, err := g.View(STATUS)
	if err != nil {
		log.Println("Failed to get status view for updating:", err)
		return
	}

	for {

		select {

		case <-exit:
			return

		case pval = <-pauseGame:

			g.Update(func(g *gocui.Gui) error {
				statusView.Clear()
				if pval == 1 {
					fmt.Fprintf(statusView, ":: PAUSE")
				} else if pval == 0 {
					fmt.Fprintf(statusView, ":: READY")
				} else if pval == 3 {
					fmt.Fprintf(statusView, ":: ERROR")
				}

				return nil
			})
		}

		time.Sleep(1 * time.Second)
	}
}

// createMazeView displays a temporary box to contain the new generated maze.
func createMazeView(g *gocui.Gui, v *gocui.View, data strings.Builder) error {

	vx, vy := v.Size()
	// maze view starting coordinates.
	mx1 := (vx - (2*MAZEWIDTH + 2)) / 2
	my1 := (vy - (MAZEHEIGHT + 2)) / 2
	// maze view ending coordinates.
	mx2 := mx1 + (2*MAZEWIDTH + 2)
	my2 := my1 + (MAZEHEIGHT + 2)

	const name = "mazeView"
	mazeView, err := g.SetView(name, mx1, my1, mx2, my2)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to display maze view:", err)
		return err
	}

	mazeView.Frame = false
	mazeView.FgColor = gocui.ColorYellow
	mazeView.SelBgColor = gocui.ColorBlack
	mazeView.SelFgColor = gocui.ColorYellow
	//inputView.Editable = true

	if _, err = g.SetCurrentView(name); err != nil {
		log.Println("Failed to set focus on maze view:", err)
		return err
	}

	_, _ = g.SetViewOnTop(name)

	// g.Cursor = true

	if err = mazeKeybindings(g, name); err != nil {
		log.Println("Failed to bind keys to maze view:", err)
		return err
	}

	// draw maze.
	fmt.Fprint(mazeView, data.String())
	data.Reset()

	// move cursor to maze entrance.
	x, _ := mazeView.Size()
	if err = mazeView.SetCursor(x/2+1, 0); err != nil {
		log.Println("Failed to set cursor at middle of maze view:", err)
	}

	// display the rat at the entrance.
	g.Cursor = true
	v.EditWrite('♣')
	v.Frame = false

	// update status and position.
	isGamePaused = false
	pauseGame <- 0
	cx, cy := v.Cursor()
	cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", cx, cy)
	return nil
}

// mazeKeybindings binds multiple keys to maze view.
func mazeKeybindings(g *gocui.Gui, name string) error {
	var err error

	if err = g.SetKeybinding(name, gocui.KeyCtrlQ, gocui.ModNone, closeMazeView); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyEsc, gocui.ModNone, closeMazeView); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyCtrlP, gocui.ModNone, pauseResumeGame); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeySpace, gocui.ModNone, pauseResumeGame); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyCtrlR, gocui.ModNone, resetGame); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyArrowUp, gocui.ModNone, moveUp); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyArrowDown, gocui.ModNone, moveDown); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyArrowLeft, gocui.ModNone, moveLeft); err != nil {
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyArrowRight, gocui.ModNone, moveRight); err != nil {
		return err
	}

	return nil
}

// closeMazeView closes current temporary maze view.
func closeMazeView(g *gocui.Gui, mv *gocui.View) error {

	mv.Clear()
	g.Cursor = false
	g.DeleteKeybindings(mv.Name())
	if err := g.DeleteView(mv.Name()); err != nil {
		log.Println("Failed to delete maze view:", err)
		return err
	}

	if err := setCurrentDefaultView(g); err != nil {
		return err
	}

	// stop timer and update game status.
	stopTimer <- struct{}{}
	isGamePaused = false
	pauseGame <- 2

	return nil
}

// setCurrentDefaultView moves the focus on default view.
func setCurrentDefaultView(g *gocui.Gui) error {
	// move back the focus on the jobs list box.
	v, err := g.SetCurrentView(OUTPUTS)
	if err != nil {
		log.Println("Failed to set focus on outputs view:", err)
		return err
	}
	v.Frame = true
	// g.Cursor = true
	v.SetCursor(0, 0)
	return nil
}

// nextView moves the focus to another view.
func nextView(g *gocui.Gui, v *gocui.View) error {

	cv := g.CurrentView()

	if cv == nil {
		if _, err := g.SetCurrentView(OUTPUTS); err != nil {
			log.Printf("Failed to set focus on default (%v) view: %v", OUTPUTS, err)
			return err
		}
		return nil
	}

	switch cv.Name() {

	case OUTPUTS:
		// move the focus on Timer view.
		if _, err := g.SetCurrentView(TIMER); err != nil {
			log.Println("Failed to set focus on timer view:", err)
			return err
		}

	case TIMER:
		// move the focus on Position view.
		if _, err := g.SetCurrentView(POSITION); err != nil {
			log.Println("Failed to set focus on position view:", err)
			return err
		}

	case POSITION:
		// move the focus on Status view.
		if _, err := g.SetCurrentView(STATUS); err != nil {
			log.Println("Failed to set focus on status view:", err)
			return err
		}

	case STATUS:
		// move the focus on Help view.
		if _, err := g.SetCurrentView(HELP); err != nil {
			log.Println("Failed to set focus on help view:", err)
			return err
		}

	case HELP:
		// move the focus on Outputs view.
		if _, err := g.SetCurrentView(OUTPUTS); err != nil {
			log.Println("Failed to set focus on maze view:", err)
			return err
		}
	}

	return nil
}

// pauseResumeGame stops/resumes the timer and disable/enable navigation and reset keys.
func pauseResumeGame(g *gocui.Gui, mv *gocui.View) error {
	var err error

	stopTimer <- struct{}{}

	// inverse the game status.
	isGamePaused = !isGamePaused

	if isGamePaused {
		pauseGame <- 1
		g.Cursor = false
		// game paused so disable controls keys bindings.
		for _, key := range []gocui.Key{gocui.KeyCtrlR, gocui.KeyArrowUp, gocui.KeyArrowDown, gocui.KeyArrowLeft, gocui.KeyArrowRight} {
			if err = g.DeleteKeybinding(mv.Name(), key, gocui.ModNone); err != nil {
				log.Printf("Failed to pause the game. error disabling key %v on maze view: %v", key, err)
				return err
			}
		}

		return nil
	}

	pauseGame <- 0
	g.Cursor = true
	// game resumed so enable controls keys bindings.
	if err = g.SetKeybinding(mv.Name(), gocui.KeyCtrlR, gocui.ModNone, resetGame); err != nil {
		log.Println("Failed to resume the game. error enabling keys on maze view:", err)
		return err
	}

	if err = g.SetKeybinding(mv.Name(), gocui.KeyArrowDown, gocui.ModNone, moveDown); err != nil {
		log.Println("Failed to resume the game. error enabling keys on maze view:", err)
		return err
	}

	if err = g.SetKeybinding(mv.Name(), gocui.KeyArrowUp, gocui.ModNone, moveUp); err != nil {
		log.Println("Failed to resume the game. error enabling keys on maze view:", err)
		return err
	}

	if err = g.SetKeybinding(mv.Name(), gocui.KeyArrowRight, gocui.ModNone, moveRight); err != nil {
		log.Println("Failed to resume the game. error enabling keys on maze view:", err)
		return err
	}

	if err = g.SetKeybinding(mv.Name(), gocui.KeyArrowLeft, gocui.ModNone, moveLeft); err != nil {
		log.Println("Failed to resume the game. error enabling keys on maze view:", err)
		return err
	}

	return nil
}

// resetGame reinitialize the timer and move to entrance position.
func resetGame(g *gocui.Gui, mv *gocui.View) error {
	resetTimer <- struct{}{}
	pauseGame <- 0
	x, _ := mv.Size()
	g.Cursor = true
	if err := mv.SetCursor(x/2+1, 0); err != nil {
		log.Println("Failed to set cursor at middle of maze view:", err)
		return err
	}

	mv.EditWrite('♣')
	cx, cy := mv.Cursor()
	cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", cx, cy)
	return nil
}

// noWallBelow returns true if there is only space at position (x,y+1).
func noWallBelow(v *gocui.View) bool {
	cx, cy := v.Cursor()

	// check for underscore-based south wall at current position.
	// if there is any error, we notify user to quit the program.
	l, err := v.Line(cy)
	if err != nil {
		log.Printf("Failed to check maze bottom direction (%d,%d). err: %v", cx, cy, err)
		pauseGame <- 3
		return false
	}

	if l[cx] == '_' {
		return false
	}

	// check if we are still into the grid.
	if (cy + 1) > MAZEHEIGHT {
		return false
	}

	// check for pipe-based south wall at next position.
	l, err = v.Line(cy + 1)
	if err != nil {
		log.Printf("Failed to check maze bottom direction (%d,%d). err: %v", cx, cy+1, err)
		pauseGame <- 3
		return false
	}

	if l[cx] == '|' {
		return false
	}

	return true
}

// moveDown moves cursor to currentX, (currentY + 1) position if there is no wall there.
func moveDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil && noWallBelow(v) == true {
		v.MoveCursor(0, 1, false)
		cx, cy := v.Cursor()
		cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", cx, cy)
	}

	return nil
}

// noWallAbove returns true if there is only space at position (x,y-1).
func noWallAbove(v *gocui.View) bool {

	// check if we are still into the grid.
	cx, cy := v.Cursor()
	if (cy - 1) < 0 {
		return false
	}

	l, err := v.Line(cy - 1)
	if err != nil {
		log.Printf("Failed to check maze up direction (%d,%d). err: %v", cx, cy-1, err)
		// signal/status to quit the program.
		pauseGame <- 3
		return false
	}

	if l[cx] == '_' || l[cx] == '|' {
		return false
	}

	return true
}

// moveUp moves cursor to currentX, (currentY - 1) position if there is no wall there.
func moveUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil && noWallAbove(v) == true {
		v.MoveCursor(0, -1, false)
		cx, cy := v.Cursor()
		cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", cx, cy)
	}

	return nil
}

// noWallOnRight returns true if there is no wall at position (x+1,y).
func noWallOnRight(v *gocui.View) bool {

	// check if we are still into the grid.
	cx, cy := v.Cursor()
	if (cx + 1) > (2*MAZEWIDTH)-1 {
		return false
	}

	l, err := v.Line(cy)
	if err != nil {
		log.Printf("Failed to check maze up direction (%d,%d). err: %v", cx, cy, err)
		// signal/status to quit the program.
		pauseGame <- 3
		return false
	}

	if l[cx+1] == '|' {
		return false
	}

	return true
}

// moveRight moves cursor to (currentX+1, currentY) position if there is no wall there.
func moveRight(g *gocui.Gui, v *gocui.View) error {
	if v != nil && noWallOnRight(v) == true {
		// there is data to next line.
		v.MoveCursor(1, 0, false)
		cx, cy := v.Cursor()
		cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", cx, cy)
	}

	return nil
}

// noWallOnLeft returns true if there is no wall at position (x-1,y).
func noWallOnLeft(v *gocui.View) bool {

	// check if we are still into the grid.
	cx, cy := v.Cursor()
	if (cx - 1) < 0 {
		return false
	}

	l, err := v.Line(cy)
	if err != nil {
		log.Printf("Failed to check maze up direction (%d,%d). err: %v", cx, cy, err)
		pauseGame <- 3
		return false
	}

	if l[cx-1] == '|' {
		return false
	}

	return true
}

// moveLeft moves cursor to (currentX-1, currentY) position if there is no wall there.
func moveLeft(g *gocui.Gui, v *gocui.View) error {
	if v != nil && noWallOnLeft(v) == true {
		// there is data to next line.
		v.MoveCursor(-1, 0, false)
		cx, cy := v.Cursor()
		cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", cx, cy)
	}

	return nil
}
