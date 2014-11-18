package main

import (
    "flag"
    "log"
    //"fmt"
    //"strconv"
    //"math/rand"

    //"github.com/yunwilliamyu/fragbag/bow"
    "github.com/yunwilliamyu/fragbag/bowdb"
)

type distType int

const (
    cosineDist distType = iota
    euclideanDist = iota
)

var (
    fragmentLibraryLoc = ""
    searchQuery = ""
    metric = cosineDist
    numCenters = -1
    metricFlag = ""
)


func init() {
    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library centers")
    flag.StringVar(&searchQuery, "searchQuery", searchQuery, "the search query as a pdb")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")

    flag.Parse()

    if metricFlag == "cosine" {
        metric = cosineDist
    }
    if metricFlag == "euclidean" {
        metric = euclideanDist
    }
}

func main() {
    db_centers, _ :=  bowdb.Open(fragmentLibraryLoc)
    db_centers.ReadAll()

    db_slices := make([]*bowdb.DB,numCenters,numCenters)
    var m map[string]int
    m = make(map[string]int)
    for i, center := range db_centers.Entries {
        tmp, _ := bowdb.Open(center.Id + ".cluster.db")
        tmp.ReadAll()
        db_slices[i] = tmp
        m[center.Id] = i
    }






}
