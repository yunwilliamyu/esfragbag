package main

import (
    "flag"
    "log"
    "fmt"
    "time"
    "os"
    "bytes"
    "encoding/gob"
    "math/rand"

    "github.com/yunwilliamyu/esfragbag/bow"
    "github.com/yunwilliamyu/esfragbag/bowdb"

)

type distType int

const (
    cosineDist distType = iota
    euclideanDist = iota
)

var (
    fragmentLibraryLoc = ""
    gobLoc = ""
    lasttime = time.Now().UTC().UnixNano()
)


func init() {
    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library centers")
    flag.StringVar(&gobLoc, "gobLoc", gobLoc, "output location for serialized clusters library")

    flag.Parse()
}

func timer() int64 {
    old := lasttime
    lasttime = time.Now().UTC().UnixNano()
    return lasttime - old
}

func enc_gob_ss_db(db_slices [][]bow.Bowed, name string) {
    f, err := os.Create(name)
    defer f.Close()
    if err != nil {
        log.Fatal("Create file error:", err)
    }
    var buf bytes.Buffer
    enc := gob.NewEncoder(&buf)
    err = enc.Encode(db_slices)
    if err != nil {
        log.Fatal("encode error:", err)
    }
    _, err = f.Write(buf.Bytes())
}

func main() {
    rand.Seed(1)


    fmt.Println(fmt.Sprintf("%d: Opening centers library",timer()))
    db_centers, _ :=  bowdb.Open(fragmentLibraryLoc)
    db_centers.ReadAll()

    fmt.Println(fmt.Sprintf("%d: Opening cluster libraries",timer()))
    numCenters := len(db_centers.Entries)
    db_slices := make([][]bow.Bowed,numCenters,numCenters)
    for i, center := range db_centers.Entries {
        tmp, err := bowdb.Open(center.Id + ".cluster.db")
        if (err!=nil) {
            fmt.Println(err)
        }
        tmp.ReadAll()
        db_slices[i] = tmp.Entries
        tmp.Close()
    }

    fmt.Println(fmt.Sprintf("%d: Serializing gob",timer()))
    enc_gob_ss_db(db_slices,gobLoc)


}
