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

	TWIDTH = 11
	PWIDTH = 30
)

var (
	// default maze size.
	MAZEHEIGHT int = 20
	MAZEWIDTH  int = 25

	// control timer in updateTimerView.
	stopTimer  = make(chan struct{})
	resetTimer = make(chan struct{})

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
	timerView.FgColor = gocui.ColorYellow
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
	positionView.FgColor = gocui.ColorYellow
	positionView.SelBgColor = gocui.ColorBlack
	positionView.SelFgColor = gocui.ColorYellow
	positionView.Editable = false
	positionView.Wrap = false
	fmt.Fprint(positionView, "X : 0 | Y : 0")

	// Help view.
	helpView, err := g.SetView(HELP, PWIDTH+1, maxY-3, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create help view:", err)
		return
	}
	helpView.Title = " Instructions "
	helpView.FgColor = gocui.ColorYellow
	helpView.SelBgColor = gocui.ColorBlack
	helpView.SelFgColor = gocui.ColorYellow
	helpView.Editable = false
	helpView.Wrap = false
	fmt.Fprint(helpView, "CTRL+N [Generate new maze] - CTRL+R [Reset position] - CTRL+Q [Close Maze]")

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

	// Help view.
	_, err = g.SetView(HELP, PWIDTH+1, maxY-3, maxX-1, maxY-1)
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

// createMazeView displays a temporary box to contain the new generated maze.
func createMazeView(g *gocui.Gui, v *gocui.View, data strings.Builder) error {

	vx, vy := v.Size()
	// maze view starting coordinates.
	mx1 := (vx - (2*MAZEWIDTH + 2)) / 2
	my1 := (vy - (MAZEHEIGHT + 2)) / 2
	// maze view ending coordinates.
	mx2 := mx1 + (2*MAZEWIDTH + 2)
	my2 := my1 + (MAZEHEIGHT + 2)
	// log.Println(vx, vy, MAZEWIDTH, MAZEHEIGHT)

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

	// bind Ctrl+Q and Escape keys to close the input box.
	if err = g.SetKeybinding(name, gocui.KeyCtrlQ, gocui.ModNone, closeMazeView); err != nil {
		log.Println(err)
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyEsc, gocui.ModNone, closeMazeView); err != nil {
		log.Println(err)
		return err
	}

	// draw maze.
	fmt.Fprint(mazeView, data.String())
	data.Reset()

	// move cursor to maze entrance.
	x, _ := mazeView.Size()
	g.Cursor = false
	if err = mazeView.SetCursor(x/2+1, 0); err != nil {
		log.Println("Failed to set cursor at middle of maze view:", err)
	}
	// display the rat at the entrance.
	mazeView.EditWrite('♣')
	v.Frame = false

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

	// stop timer.
	stopTimer <- struct{}{}

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
		// move the focus on Help view.
		if _, err := g.SetCurrentView(HELP); err != nil {
			log.Println("Failed to set focus on stats view:", err)
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
