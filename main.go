package main

import (
	_ "net/http/pprof"

	"github.com/dszqbsm/crawler/cmd"
	_ "github.com/dszqbsm/crawler/tasklib"
)

func main() {
	cmd.Execute()
}
