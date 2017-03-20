# Development Guide

# New to Go?
Quartermaster is written in Go and if you are new to the language, it is *highly*
encouraged you take [A Tour of Go](http://tour.golang.org/welcome/1).

# New to Github?
If you are new to the Github process, please see https://guides.github.com/introduction/flow/index.html.

# Development

## Workspace Setup

1. Fork the quartermaster Github project
1. Download latest Go to your system
1. Setup your [GOPATH](http://www.g33knotes.org/2014/07/60-second-count-down-to-go.html) environment
1. Type: `mkdir -p $GOPATH/src/github.com/coreos`
1. Type: `cd $GOPATH/src/github.com/coreos`
1. Type: `git clone https://github.com/coreos/quartermaster.git`
1. Type: `cd quartermaster`
1. Build: `make`

Now you need to setup your repo where you will be pushing your changes into:

1. `git remote add github <<your forked github repo information>>`
1. `git fetch github`

## Builds
From the top of the quartermaster source tree, type: `make`
