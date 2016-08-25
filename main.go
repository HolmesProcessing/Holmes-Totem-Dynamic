package main

import (
	"flag"

	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/check"
	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/feed"
	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/lib"
	"github.com/HolmesProcessing/Holmes-Totem-Dynamic/submit"
)

func main() {
	cPath := flag.String("config", "", "Path to the configuration file")
	flag.Parse()

	ctx := &lib.Ctx{}

	err := ctx.Init(*cPath)
	if err != nil {
		panic(err.Error())
	}

	err = feed.Run(ctx)
	if err != nil {
		panic(err.Error())
	}
	ctx.Info.Println("feed running")

	err = check.Run(ctx)
	if err != nil {
		panic(err.Error())
	}
	ctx.Info.Println("check running")

	err = submit.Run(ctx)
	if err != nil {
		panic(err.Error())
	}
	ctx.Info.Println("submit running")
}
