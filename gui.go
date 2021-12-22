package main

// This is a small go-based program to generate nice maze using recursive backtracking algorithm.
// It will allow a user to use arrow keys to navigate from a single entrance to an exit door.

// Version  : 1.0
// Author   : Jerome AMON
// Created  : 22 November 2021

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jroimartin/gocui"
)

const (
	OUTPUTS  = "outputs"
	INFOS    = "infos"
	POSITION = "position"
	TIMER    = "timer"
	STATUS   = "status"
	SIZE     = "size"
	HELP     = "help"
	MAZE     = "maze"

	TWIDTH  = 11
	PWIDTH  = 30
	SWIDTH  = 45
	SZWIDTH = 58
	HWIDTH  = 44
	HHEIGHT = 28

	SAVING_INTERVAL_SECS = 15
)

const helpDetails = `

-------------+----------------------------
    CTRL + D | close this help window
-------------+----------------------------
    CTRL + E | edit maze width/height
-------------+----------------------------
    CTRL + N | create a full new maze
-------------+----------------------------
    CTRL + Q | quit existing challenge
-------------+----------------------------
    CTRL + P | pause current challenge
-------------+----------------------------
    CTRL + R | resume from paused game
-------------+----------------------------
    CTRL + S | save current game state
-------------+----------------------------
    CTRL + L | load a saved game state
-------------+----------------------------
    CTRL + F | find & display solution
-------------+----------------------------
    ↕ and ↔  | navigate into the maze
-------------+----------------------------
    CTRL + C | close the whole program
-------------+----------------------------

::::::: Craft with ♥ by Jerome Amon ::::::
`

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
	statusGame   = make(chan uint8, 3)
	isGamePaused = false

	cursorPosition = make(chan string, 10)

	// control goroutines.
	exit = make(chan struct{})
	wg   sync.WaitGroup

	// keep latest coordinates of the cursor in maze.
	latestMazeCursorX, latestMazeCursorY int

	// store formatted current maze infos.
	currentMazeData strings.Builder
	currentMazeID   string
	// used to throttle saving actions.
	lastestSavingTime time.Time
)

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	// on windows only change terminal title.
	if runtime.GOOS == "windows" {
		exec.Command("cmd", "/c", "title [ GoMazes By Jerome Amon ]").Run()
	}

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

	// Size view.
	sizeView, err := g.SetView(SIZE, SWIDTH+1, maxY-3, SZWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create maze size view:", err)
		return
	}
	sizeView.Title = " Size "
	sizeView.FgColor = gocui.ColorGreen
	sizeView.SelBgColor = gocui.ColorBlack
	sizeView.SelFgColor = gocui.ColorYellow
	sizeView.Editable = false
	sizeView.Wrap = false
	fmt.Fprintf(sizeView, center(fmt.Sprintf("%d x %d", MAZEWIDTH, MAZEHEIGHT), SZWIDTH-SWIDTH-1, " "))

	// Infos view.
	infosView, err := g.SetView(INFOS, SZWIDTH+1, maxY-3, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create help view:", err)
		return
	}
	infosView.FgColor = gocui.ColorWhite
	infosView.SelBgColor = gocui.ColorBlack
	infosView.SelFgColor = gocui.ColorYellow
	infosView.Editable = false
	infosView.Wrap = false
	fmt.Fprint(infosView, center("F1 or CTRL+D [Display Help] - CTRL+N [Play New Maze] - CTRL+C [Exit Game]", maxX-SZWIDTH-2, " "))

	// Apply keybindings to program.
	if err = keybindings(g); err != nil {
		log.Println("Failed to setup keybindings:", err)
		return
	}

	// move the focus on the jobs list box.
	if _, err = g.SetCurrentView(OUTPUTS); err != nil {
		log.Println("Failed to set focus on outputs view:", err)
		return
	}

	// adjust maze default size based on outputs view.
	x, y := outputsView.Size()
	if 2*MAZEWIDTH >= x {
		MAZEWIDTH = (x - 2) / 2
	}

	if MAZEHEIGHT >= y {
		MAZEHEIGHT = y - 2
	}

	wg.Add(1)
	go updateTimerView(g)

	wg.Add(1)
	go updatePositionView(g, PWIDTH-TWIDTH-1)

	wg.Add(1)
	go updateStatusView(g)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		close(exit)
		log.Println("Exited from the main loop:", err)
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

	// Maze Size view.
	_, err = g.SetView(SIZE, SWIDTH+1, maxY-3, SZWIDTH, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create maze size view:", err)
		return err
	}

	// Help view.
	_, err = g.SetView(INFOS, SZWIDTH+1, maxY-3, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to create infos view:", err)
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

	// to display help details. We use Ctrl+D or F1.
	// On unix-like platforms, we can also use Ctrl+H.
	if runtime.GOOS != "windows" {
		if err := g.SetKeybinding("", gocui.KeyCtrlH, gocui.ModNone, displayHelpView); err != nil {
			return err
		}
	}

	if err := g.SetKeybinding("", gocui.KeyF1, gocui.ModNone, displayHelpView); err != nil {
		return err
	}

	if err := g.SetKeybinding("", gocui.KeyCtrlD, gocui.ModNone, displayHelpView); err != nil {
		return err
	}

	// generate & display new maze when focus on outputs (maze zone).
	if err := g.SetKeybinding(OUTPUTS, gocui.KeyCtrlN, gocui.ModNone, displayNewMaze); err != nil {
		return err
	}

	// edit current default maze settings (width and height).
	if err := g.SetKeybinding(OUTPUTS, gocui.KeyCtrlE, gocui.ModNone, editMazeSize); err != nil {
		return err
	}

	// when focused on outputs view, display help at H or h key press.
	if err := g.SetKeybinding(OUTPUTS, 'H', gocui.ModNone, displayHelpView); err != nil {
		return err
	}

	if err := g.SetKeybinding(OUTPUTS, 'h', gocui.ModNone, displayHelpView); err != nil {
		return err
	}

	// display all previous saved sessions to load one of them as new maze game.
	if err := g.SetKeybinding(OUTPUTS, gocui.KeyCtrlL, gocui.ModNone, displayExistingMaze); err != nil {
		return err
	}

	return nil
}

// displayExistingMaze displays all saved maze sessions as a list
// and allows to choose one to be loaded for replaying.
func displayExistingMaze(g *gocui.Gui, v *gocui.View) error {

	if _, err := os.Stat("savedsessions"); errors.Is(err, os.ErrNotExist) {
		log.Println("There is no saved maze sessions. No folder <savedsessions>")
		return nil
	}

	folder, err := os.Open("savedsessions")
	if err != nil {
		return err
	}
	defer folder.Close()
	filenames, err := folder.Readdirnames(0)
	if err != nil {
		return err
	}

	if len(filenames) == 0 {
		return nil
	}

	H := len(filenames) + 1

	// constructs the listview.
	const name = "listview"
	maxX, maxY := g.Size()

	if (H + 4) >= maxY {
		H = maxY - 4
	}

	listView, err := g.SetView(name, (maxX-21)/2, (maxY-H)/2, maxX/2+21, (maxY+H)/2)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to display saved sessions listview:", err)
		return err
	}

	listView.Title = " Select A Session To Replay "
	listView.Frame = true
	listView.FgColor = gocui.ColorYellow
	listView.SelBgColor = gocui.ColorGreen
	listView.SelFgColor = gocui.ColorBlack
	listView.Editable = false

	if _, err = g.SetCurrentView(name); err != nil {
		log.Println("Failed to set focus on maze sessions listview:", err)
		return err
	}

	g.Cursor = true
	listView.Highlight = true

	if err = g.SetKeybinding(name, gocui.KeyArrowUp, gocui.ModNone, sessionCursorUp); err != nil {
		log.Println("Failed to bind Arrow Up key to sessions listview:", err)
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyArrowDown, gocui.ModNone, sessionCursorDown); err != nil {
		log.Println("Failed to bind Arrow Down key to sessions listview:", err)
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyEnter, gocui.ModNone, processEnterOnListView); err != nil {
		log.Println("Failed to bind Enter key to sessions listview:", err)
		return err
	}

	// Ctrl+Q and Escape keys to close the input box.
	if err = g.SetKeybinding(name, gocui.KeyCtrlQ, gocui.ModNone, closeListView); err != nil {
		log.Println("Failed to bind CtrlQ key to maze sessions listview:", err)
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyEsc, gocui.ModNone, closeListView); err != nil {
		log.Println("Failed to bind Esc key to maze sessions listview:", err)
		return err
	}

	_, _ = g.SetViewOnTop(name)
	listView.SetCursor(0, 0)

	for _, filename := range filenames {
		fmt.Fprint(listView, " "+strings.ReplaceAll(filename, ".", ":")+" "+"\n")
	}

	return nil
}

// ipsLineBelow returns true if there is data at position y+1.
func lvLineBelow(v *gocui.View) bool {
	_, cy := v.Cursor()
	if l, _ := v.Line(cy + 1); l != "" {
		return true
	}
	return false
}

// sessionCursorDown moves cursor to (currentY + 1) position if there is data there.
func sessionCursorDown(g *gocui.Gui, lv *gocui.View) error {
	if lv != nil && lvLineBelow(lv) == true {
		lv.MoveCursor(0, 1, false)
	}

	return nil
}

// lvLineAbove returns true if there is data at position y-1.
func lvLineAbove(v *gocui.View) bool {
	_, cy := v.Cursor()
	if l, _ := v.Line(cy - 1); l != "" {
		return true
	}
	return false
}

// sessionCursorUp moves cursor to (currentY - 1) position if there is data there.
func sessionCursorUp(g *gocui.Gui, lv *gocui.View) error {
	if lv != nil && lvLineAbove(lv) == true {
		lv.MoveCursor(0, -1, false)
	}

	return nil
}

// closeListView closes temporary maze sessions listview.
func closeListView(g *gocui.Gui, lv *gocui.View) error {

	lv.Clear()
	g.Cursor = false
	g.DeleteKeybindings(lv.Name())
	if err := g.DeleteView(lv.Name()); err != nil {
		log.Println("Failed to delete maze sessions listview:", err)
		return err
	}

	_ = setFocusOnView(g, OUTPUTS)

	return nil
}

// loadMazeData reads the backup maze file content then
// extracts the saved cursor position followed by the
// maze data.
func loadMazeData(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	data, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	data = strings.TrimSpace(data)
	xy := strings.Fields(data)
	if len(xy) != 2 {
		return errors.New("wrong coordinates values")
	}

	if x, err := strconv.Atoi(xy[0]); err != nil {
		return errors.New("wrong X coordinates value")
	} else {
		latestMazeCursorX = x
	}

	if y, err := strconv.Atoi(xy[1]); err != nil {
		return errors.New("wrong Y coordinates value")
	} else {
		latestMazeCursorY = y
	}

	for {
		data, err = reader.ReadString('\n')
		currentMazeData.WriteString(data)
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
	}

	return nil
}

// processEnterOnListView allows to choose an existing saved maze for playing.
func processEnterOnListView(g *gocui.Gui, lv *gocui.View) error {

	_, cy := lv.Cursor()
	session, err := lv.Line(cy)
	if err != nil {
		log.Println("Failed to read current focused session name:", err)
		return nil
	}
	session = strings.ReplaceAll(strings.TrimSpace(session), ":", ".")
	// should not happen but for safety.
	if len(session) == 0 {
		return nil
	}

	if err := closeListView(g, lv); err != nil {
		return err
	}

	currentMazeData.Reset()
	currentMazeID = ""

	if err := loadMazeData("savedsessions" + string(os.PathSeparator) + session); err != nil {
		log.Println("Failed to load existing maze data:", err)
		return err
	}

	// expected to be OUTPTUS view.
	ov := g.CurrentView()
	ov.Clear()

	if err := createMazeView(g, ov); err != nil {
		log.Println("Failed to load & display existing maze:", err)
		return err
	}

	if mv := g.CurrentView(); mv != nil {
		mv.SetCursor(latestMazeCursorX, latestMazeCursorY)
		cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", latestMazeCursorX, latestMazeCursorY)
	}

	currentMazeID = session

	// reset and start timer.
	resetTimer <- struct{}{}
	stopTimer <- struct{}{}
	return nil
}

// displayNewMaze triggers generation of new maze and display it.
func displayNewMaze(g *gocui.Gui, v *gocui.View) error {

	xLines, yLines := v.Size()

	if 2*MAZEWIDTH >= xLines {
		MAZEWIDTH = (xLines - 2) / 2
	}

	if MAZEHEIGHT >= yLines {
		MAZEHEIGHT = yLines - 2
	}

	currentMazeData.Reset()
	currentMazeID = ""
	lastestSavingTime = time.Time{}
	maze := createMaze(MAZEWIDTH, MAZEHEIGHT)
	currentMazeData = formatMaze(maze, MAZEWIDTH, MAZEHEIGHT)
	maze = nil

	v.Clear()

	if err := createMazeView(g, v); err != nil {
		log.Println("Failed to create & display new maze:", err)
		return err
	}

	// reset and start timer.
	resetTimer <- struct{}{}
	stopTimer <- struct{}{}
	return nil
}

// updateTimerView tracks elapsed time since maze is displayed.
func updateTimerView(g *gocui.Gui) {
	defer wg.Done()
	stop, reset := true, false
	secsElapsed, hrs, mins, secs := 0, 0, 0, 0

	timerView, err := g.View(TIMER)
	if err != nil {
		log.Println("Failed to get timer view for updating:", err)
		return
	}

	for {

		select {

		case <-exit:
			return

		case <-stopTimer:
			stop = !stop

		case <-resetTimer:
			secsElapsed = 0
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
				secsElapsed++
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
				return nil
			})
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// updateStatusView displays current game status.
func updateStatusView(g *gocui.Gui) {
	defer wg.Done()
	var sval uint8

	statusView, err := g.View(STATUS)
	if err != nil {
		log.Println("Failed to get status view for updating:", err)
		return
	}

	for {

		select {

		case <-exit:
			return

		case sval = <-statusGame:

			g.Update(func(g *gocui.Gui) error {
				statusView.Clear()
				if sval == 1 {
					fmt.Fprintf(statusView, ":: PAUSE")
				} else if sval == 0 {
					fmt.Fprintf(statusView, ":: READY")
				} else if sval == 3 {
					fmt.Fprintf(statusView, ":: ERROR")
				}

				return nil
			})
		}

		time.Sleep(1 * time.Second)
	}
}

// createMazeView displays a temporary box to contain the new generated maze.
func createMazeView(g *gocui.Gui, v *gocui.View) error {

	vx, vy := v.Size()
	// maze view starting coordinates.
	mx1 := (vx - (2*MAZEWIDTH + 2)) / 2
	my1 := (vy - (MAZEHEIGHT + 2)) / 2
	// maze view ending coordinates.
	mx2 := mx1 + (2*MAZEWIDTH + 2)
	my2 := my1 + (MAZEHEIGHT + 2)

	mazeView, err := g.SetView(MAZE, mx1, my1, mx2, my2)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to display maze view:", err)
		return err
	}

	mazeView.Frame = false
	mazeView.FgColor = gocui.ColorYellow
	mazeView.BgColor = gocui.ColorBlack
	mazeView.SelBgColor = gocui.ColorBlack
	mazeView.SelFgColor = gocui.ColorYellow

	if _, err = g.SetCurrentView(MAZE); err != nil {
		log.Println("Failed to set focus on maze view:", err)
		return err
	}

	_, _ = g.SetViewOnTop(MAZE)

	if err = mazeKeybindings(g, MAZE); err != nil {
		log.Println("Failed to bind keys to maze view:", err)
		return err
	}

	// draw maze.
	fmt.Fprint(mazeView, currentMazeData.String())

	// move cursor to maze entrance.
	x, _ := mazeView.Size()
	if err = mazeView.SetCursor(x/2+1, 0); err != nil {
		log.Println("Failed to set cursor at middle of maze view:", err)
		// just alert for error during setup.
		statusGame <- 3
	}

	g.Cursor = true
	v.Frame = false

	// update status and position.
	isGamePaused = false
	statusGame <- 0
	cx, cy := v.Cursor()
	cursorPosition <- fmt.Sprintf("(X:%d | Y:%d)", cx, cy)

	t := time.Now()
	currentMazeID = fmt.Sprintf("%02d-%02d-%02d %02dH.%02dM.%02dS", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())

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

	if err = g.SetKeybinding(name, gocui.KeyCtrlS, gocui.ModNone, saveGame); err != nil {
		return err
	}

	return nil
}

// saveGame saves current maze on file disk inside savedsessions folder.
// It generates (if not already created) a dedicated file named with the
// current maze session id <currentMazeID>. The first line inside the file
// contains the latest cursor coordinates (x, y) followed by the maze data.
func saveGame(g *gocui.Gui, mv *gocui.View) error {

	// throttle saving action. could be done each <SAVING_INTERVAL_SECS>.
	if (time.Since(lastestSavingTime)).Seconds() < SAVING_INTERVAL_SECS {
		return nil
	}

	if _, err := os.Stat("savedsessions"); errors.Is(err, os.ErrNotExist) {
		// folder does not exist. we create it.
		if err := os.Mkdir("savedsessions", 0755); err != nil {
			log.Println("Failed to create savedsessions folder:", err)
			return nil
		}
	}

	fpath := "savedsessions" + string(os.PathSeparator) + currentMazeID
	file, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Println("Failed to create savedsessions file:", err)
		return nil
	}
	defer file.Close()

	cx, cy := mv.Cursor()

	_, err = fmt.Fprintln(file, cx, cy)
	if err != nil {
		log.Println("Failed to save cursor position in session file:", err)
		return nil
	}
	_, err = fmt.Fprint(file, currentMazeData.String())
	if err != nil {
		log.Println("Failed to save maze data in session file:", err)
		return nil
	}

	lastestSavingTime = time.Now()

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

	if err := setFocusOnView(g, OUTPUTS); err != nil {
		return err
	}

	// stop timer and update game status.
	stopTimer <- struct{}{}
	isGamePaused = false
	statusGame <- 2

	// clean stored maze data.
	currentMazeData.Reset()
	currentMazeID = ""

	return nil
}

// setFocusOnView moves the focus on a given view. In case,
// it is the maze view place the cursor to lastest position.
func setFocusOnView(g *gocui.Gui, name string) error {
	// move back the focus on the jobs list box.
	v, err := g.SetCurrentView(name)
	if err != nil {
		log.Printf("Failed to set focus on %s view:", name, err)
		return err
	}

	v.Frame = true
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
		if _, err := g.SetCurrentView(INFOS); err != nil {
			log.Println("Failed to set focus on help view:", err)
			return err
		}

	case INFOS:
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
		statusGame <- 1
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

	statusGame <- 0
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
	statusGame <- 0
	x, _ := mv.Size()
	g.Cursor = true
	if err := mv.SetCursor(x/2+1, 0); err != nil {
		log.Println("Failed to set cursor at middle of maze view:", err)
		return err
	}

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
		statusGame <- 3
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
		statusGame <- 3
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
		statusGame <- 3
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
		statusGame <- 3
		return false
	}

	if (cy == 0 && l[cx+1] == '_') || l[cx+1] == '|' {
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
		statusGame <- 3
		return false
	}

	if (cy == 0 && l[cx-1] == '_') || l[cx-1] == '|' {
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

// displayHelpView displays help details. But save the current cursor
// position in case the maze is displayed before. Then pause the game.
func displayHelpView(g *gocui.Gui, cv *gocui.View) error {

	if cv.Name() == MAZE {
		latestMazeCursorX, latestMazeCursorY = cv.Cursor()

		// try to pause the game if not yet done. If failure
		// abort the process and flag status with <ERROR>.
		if !isGamePaused {
			if err := pauseResumeGame(g, cv); err != nil {
				log.Println("Failed to pause the game before displaying help view:", err)
				statusGame <- 3
				return err
			}
		}
	}

	maxX, maxY := g.Size()

	// construct the input box and position at the center of the screen.
	if helpView, err := g.SetView(HELP, (maxX-HWIDTH)/2, (maxY-HHEIGHT)/2, maxX/2+HWIDTH, (maxY+HHEIGHT)/2); err != nil {
		if err != gocui.ErrUnknownView {
			log.Println("Failed to create help view:", err)
			return err
		}

		helpView.FgColor = gocui.ColorGreen
		helpView.SelBgColor = gocui.ColorBlack
		helpView.SelFgColor = gocui.ColorYellow
		helpView.Editable = false
		helpView.Autoscroll = true
		helpView.Wrap = true
		helpView.Frame = false

		if _, err := g.SetCurrentView(HELP); err != nil {
			log.Println("Failed to set focus on help view:", err)
			return err
		}
		g.Cursor = false

		// bind Ctrl+Q and Escape and Ctrl+H and F1 and Ctrl+D keys to close the input box.
		if err := g.SetKeybinding(HELP, gocui.KeyCtrlQ, gocui.ModNone, closeHelpView); err != nil {
			log.Println("Failed to bind keys (CtrlQ) to help view:", err)
			return err
		}

		if runtime.GOOS != "windows" {
			if err := g.SetKeybinding(HELP, gocui.KeyCtrlH, gocui.ModNone, closeHelpView); err != nil {
				log.Println("Failed to bind keys (CtrlH) to help view:", err)
				return err
			}
		}

		if err := g.SetKeybinding(HELP, gocui.KeyCtrlD, gocui.ModNone, closeHelpView); err != nil {
			log.Println("Failed to bind keys (CtrlD) to close help view:", err)
			return err
		}

		if err := g.SetKeybinding(HELP, gocui.KeyF1, gocui.ModNone, closeHelpView); err != nil {
			log.Println("Failed to bind keys (F1) to help view:", err)
			return err
		}

		if err := g.SetKeybinding(HELP, gocui.KeyEsc, gocui.ModNone, closeHelpView); err != nil {
			log.Println("Failed to bind keys (Esc) to help view:", err)
			return err
		}

		if err := g.SetKeybinding(HELP, 'H', gocui.ModNone, closeHelpView); err != nil {
			log.Println("Failed to bind keys (H) to help view:", err)
			return err
		}

		if err := g.SetKeybinding(HELP, 'h', gocui.ModNone, closeHelpView); err != nil {
			log.Println("Failed to bind keys (H) to help view:", err)
			return err
		}

		fmt.Fprintf(helpView, helpDetails)

	}
	return nil
}

// closeHelpView closes help view then move the focus on
// maze view in case it exists otherwise set it to output view.
func closeHelpView(g *gocui.Gui, hv *gocui.View) error {

	hv.Clear()
	g.Cursor = false
	g.DeleteKeybindings(hv.Name())
	if err := g.DeleteView(hv.Name()); err != nil {
		log.Println("Failed to delete help view:", err)
		return err
	}

	if _, err := g.View(MAZE); err != gocui.ErrUnknownView {
		mv, err := g.SetCurrentView(MAZE)
		if err != nil {
			log.Printf("Failed to set back focus on maze view:", err)
			statusGame <- 3
			return err
		}

		mv.Frame = false
		mv.SetCursor(latestMazeCursorX, latestMazeCursorY)
		g.Cursor = false
		return nil
	}

	if err := setFocusOnView(g, OUTPUTS); err != nil {
		log.Println("Failed to set back focus on outputs view:", err)
		return err
	}

	return nil
}

// editMazeSize provides a temporary input box to type wanted maze size (width & height).
func editMazeSize(g *gocui.Gui, cv *gocui.View) error {
	maxX, maxY := g.Size()
	const name = "MazeSizeView"

	inputView, err := g.SetView(name, maxX/2-20, maxY/2, maxX/2+20, maxY/2+2)
	if err != nil && err != gocui.ErrUnknownView {
		log.Println("Failed to display maze size input view:", err)
		return err
	}

	inputView.Title = " Edit Maze Size (width x height) "
	inputView.Frame = true
	inputView.FgColor = gocui.ColorYellow
	inputView.SelBgColor = gocui.ColorBlack
	inputView.SelFgColor = gocui.ColorYellow
	inputView.Editable = true

	if _, err = g.SetCurrentView(name); err != nil {
		log.Println("Failed to set focus on maze size input view:", err)
		return err
	}

	g.Cursor = true
	inputView.Highlight = true

	if err = g.SetKeybinding(name, gocui.KeyEnter, gocui.ModNone, copyMazeSizeInput); err != nil {
		log.Println("Failed to bind Enter key to maze size input view:", err)
		return err
	}

	// Ctrl+Q and Escape keys to close the input box.
	if err = g.SetKeybinding(name, gocui.KeyCtrlQ, gocui.ModNone, closeMazeSizeInputView); err != nil {
		log.Println("Failed to bind CtrlQ key to maze size input view:", err)
		return err
	}

	if err = g.SetKeybinding(name, gocui.KeyEsc, gocui.ModNone, closeMazeSizeInputView); err != nil {
		log.Println("Failed to bind Esc key to maze size input view:", err)
		return err
	}

	_, _ = g.SetViewOnTop(name)

	fmt.Fprintf(inputView, "%d x %d", MAZEWIDTH, MAZEHEIGHT)
	inputView.SetCursor(len(fmt.Sprintf("%d x %d", MAZEWIDTH, MAZEHEIGHT)), 0)

	return nil
}

// closeMazeSizeInputView closes current temporary maze view.
func closeMazeSizeInputView(g *gocui.Gui, iv *gocui.View) error {

	iv.Clear()
	g.Cursor = false
	g.DeleteKeybindings(iv.Name())
	if err := g.DeleteView(iv.Name()); err != nil {
		log.Println("Failed to delete maze size input view:", err)
		return err
	}

	_ = setFocusOnView(g, OUTPUTS)

	return nil
}

// copyMazeSizeInput processes the value entered and set default maze size.
func copyMazeSizeInput(g *gocui.Gui, iv *gocui.View) error {
	var err error
	var ov *gocui.View

	iv.Rewind()
	ov, err = g.View(OUTPUTS)
	if err == gocui.ErrUnknownView {
		log.Println("Failed to get outputs view:", err)
	}

	input := strings.TrimSpace(iv.Buffer())

	if input != "" {
		// data typed, add it.
		x, y := ov.Size()
		setupMazeSize(input, x, y)
		g.Update(func(g *gocui.Gui) error {
			sizeView, _ := g.View(SIZE)
			sizeView.Clear()
			fmt.Fprintf(sizeView, center(fmt.Sprintf("%d x %d", MAZEWIDTH, MAZEHEIGHT), SZWIDTH-SWIDTH-1, " "))
			return nil
		})

	} else {
		// no data entered, so go back.
		editMazeSize(g, ov)
		return nil
	}

	iv.Clear()

	// must delete keybindings before the view, or fatal error.
	g.DeleteKeybindings(iv.Name())
	if err = g.DeleteView(iv.Name()); err != nil {
		log.Println("Failed to delete maze size input view:", err)
		return err
	}

	_ = setFocusOnView(g, OUTPUTS)

	return nil
}

// setupMazeSize configures default maze size.
// expect to receive <width x height> format.
func setupMazeSize(size string, x, y int) {
	s := strings.Split(size, "x")
	if len(s) != 2 {
		log.Println("Failed to setup maze size because no valid input data")
		return
	}

	w, err := strconv.Atoi(strings.TrimSpace(s[0]))
	if err != nil {
		log.Println("Failed to setup maze width size because no valid input data")
	} else if w > 15 {
		MAZEWIDTH = w
	}

	h, err := strconv.Atoi(strings.TrimSpace(s[1]))
	if err != nil {
		log.Println("Failed to setup maze height size because no valid input data")
	} else if h > 10 {
		MAZEHEIGHT = h
	}

	// limit to the output size.
	if 2*MAZEWIDTH >= x {
		MAZEWIDTH = (x - 2) / 2
	}

	if MAZEHEIGHT >= y {
		MAZEHEIGHT = y - 2
	}
}
