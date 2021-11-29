# gomazes

Have fun time with kids and family and friends at playing awesome 2D maze-based games while feeling like a programmer on the computer console/terminal. Enjoy as well.


![live minimal maze](https://github.com/jeamon/gomazes/blob/master/gomazes-demo-01.gif?raw=true)


## Features / Goals

* define the default size (width & height) of the maze
* auto adjust the provided maze size based on screen size
* use keyboard (CTRL+E) to edit default maze size (w x h) 
* use keyboard (CTRL+N) to generate new maze at any time
* use keyboard (CTRL+Q) to cancel current displayed maze
* use keyboard (CTRL+R) to go back to the initial position
* use keyboard (CTRL+F) to find/display the path of the maze
* use keyboard (CTRL+P) to pause/resume the current challenge
* use keyboard (CTRL+S) to save the current maze challenge
* use keyboard (CTRL+L) to load any past saved maze challenge
* use keyboard (CTRL+C) to close immediately the whole game
* use keyboard (CTRL+D) to display or close the help details
* timer to view the time elapsed since the maze get displayed
* view in real-time the exact coordinates of your position
* view in real-time the game status (pause or ready or loading)
* replay the same maze by moving back the cursor to entrance
* use keyboard (ESC) to quit the maze and SPACE to pause/resume
* auto pause the game when help is displayed (via F1 or CTRL+D)


## Demo

Preview on my youtube channel. [click here](https://youtu.be/ZkUj3ya-hw0) or [use this link](https://youtu.be/cirojskoPBw)

![overview of maze](https://github.com/jeamon/gomazes/blob/master/maze-demo-01.PNG?raw=true)

## Installation

* **Download executables files**

Please check later on [releases page](https://github.com/jeamon/gomazes/releases)

* **From source on windows**

```shell
$ git clone https://github.com/jeamon/gomazes.git
$ cd gomazes
$ go build -o gomazes.exe .
```
* **From source on linux/macos**

```shell
$ git clone https://github.com/jeamon/gomazes.git
$ cd gomazes
$ go build -o gomazes .
$ chmod +x ./gomazes
```

## Getting started

* Start the game with default (width, height) of (20,15)

```
$ gomazes.exe 20 15
```

```
$ ./gomazes 20 15
```

## License

Please check & read [the license details](https://github.com/jeamon/gomazes/blob/master/LICENSE) 


## Contact
---

Feel free to [reach out to me](https://blog.cloudmentor-scale.com/contact) before any action. Feel free to connect on [Twitter](https://twitter.com/jerome_amon) or [linkedin](https://www.linkedin.com/in/jeromeamon/)