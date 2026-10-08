//go:build !wasip1

package main

import (
	"context"
	"log"

	"github.com/mavioai/mavio/libs/plugin/guest/process"
)

func main() {
	if err := process.Serve(context.Background(), ID); err != nil {
		log.Fatal(err)
	}
}
