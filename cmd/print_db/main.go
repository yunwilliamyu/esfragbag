package main

import (
	"flag"
	"fmt"
	"log"

	//"github.com/yunwilliamyu/esfragbag/bow"
	"github.com/yunwilliamyu/esfragbag/bowdb"
)

type empty struct{}

var (
	fragmentLibraryLoc = ""
)

func init() {
	log.SetFlags(0)

	flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library")

	flag.Parse()

}

func main() {
	db, _ := bowdb.Open(fragmentLibraryLoc)
	db.ReadAll()
	for _, item := range db.Entries {
		fmt.Println(item.Id + ": " + fmt.Sprintf("%v", item.Bow.Freqs))
	}

}
