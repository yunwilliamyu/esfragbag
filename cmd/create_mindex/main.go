package main

import (
    "flag"
    "log"
    "fmt"
    "time"
    //"strconv"
    "math"
    "math/rand"
    "runtime"

    "github.com/yunwilliamyu/esfragbag/bow"
    "github.com/yunwilliamyu/esfragbag/bowdb"
)

type empty struct{}


type algType int
const (
    randomSelec algType = iota
    metricApprox = iota
    halfhalf = iota
)

var (
    fragmentLibraryLoc = ""
    metric = bowdb.CosineDist
    numCenters = -1
    metricFlag = ""
    centerType = randomSelec
    kCenterAlg = ""
    maxRadius = -1.0
    lasttime = time.Now().UTC().UnixNano()
)


func init() {
    //rand.Seed(time.Now().UTC().UnixNano())
    rand.Seed(314159)

    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")

    flag.Parse()

    if metricFlag == "cosine" {
        metric = bowdb.CosineDist
    }
    if metricFlag == "euclidean" {
        metric = bowdb.EuclideanDist
    }
}

func timer() int64 {
    old := lasttime
    lasttime = time.Now().UTC().UnixNano()
    return lasttime - old
}

func main() {
    timer()
    runtime.GOMAXPROCS(20)

    db, _ :=  bowdb.Open(fragmentLibraryLoc)
    db.ReadAll()
    //Assert(err, "Could not open BOW database '%s'", path) 
    var anchorPoints []bow.Bowed
    //var anchorPoints2 []bow.Bowed
    //var anchorPoints3 []bow.Bowed
    fmt.Println(len(db.Entries))
    var numAnchors = int(math.Log2(float64(len(db.Entries)))) / 2
    if numAnchors > 16 {
        numAnchors = 16
    }
    fmt.Println(fmt.Sprintf("%d: Randomly selecting %d anchor points", timer(), numAnchors))
    anchorPoints = bowdb.RandomKSample(db.Entries, metric, numAnchors)
    hashes := bowdb.MIndexHashes(db.Entries, anchorPoints, metric)
    fmt.Printf("\n")
    table_gob := bowdb.M_index_table{anchorPoints, db.Entries, hashes}
    gobLoc := "mindex.gob"
    fmt.Println(fmt.Sprintf("%d: Serializing gob for mindex",timer()))
    bowdb.Enc_gob_mindex(table_gob,gobLoc)





}
