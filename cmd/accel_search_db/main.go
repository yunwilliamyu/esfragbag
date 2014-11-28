package main

import (
    "flag"
    "log"
    "fmt"
    "time"
    "os"
    "bytes"
    "io/ioutil"
    //"strconv"
    "encoding/gob"
    "math/rand"

    "github.com/yunwilliamyu/fragbag/bow"
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
    metricFlag = ""
    potentialTargetsLoc = ""
    maxRadius = 0
    lasttime = time.Now().UTC().UnixNano()
)


func init() {
    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library centers")
    flag.StringVar(&searchQuery, "searchQuery", searchQuery, "the search query library as a bowdb")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")
    flag.StringVar(&potentialTargetsLoc, "potentialTargets", potentialTargetsLoc, "the location of the full fragment library database")
    flag.IntVar(&maxRadius, "maxRadius", maxRadius, "maximum radius to search in")

    flag.Parse()

    if metricFlag == "cosine" {
        metric = cosineDist
    }
    if metricFlag == "euclidean" {
        metric = euclideanDist
    }
}

func newSearchResult(query, entry bow.Bowed) bowdb.SearchResult {
    return bowdb.SearchResult{
        Bowed:  entry,
        Cosine: query.Bow.Cosine(entry.Bow),
        Euclid: query.Bow.Euclid(entry.Bow),
    }
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
    //buf := io.NewWriter(f)
    enc := gob.NewEncoder(&buf)
    err = enc.Encode(db_slices)
    if err != nil {
        log.Fatal("encode error:", err)
    }
    _, err = f.Write(buf.Bytes())
}

func dec_gob_ss_db(name string) [][]bow.Bowed {
    buf_bytes, err := ioutil.ReadFile(name)
    if err != nil {
        log.Fatal("Open file error:", err)
    }
    var buf bytes.Buffer
    buf.Write(buf_bytes)
    //buf := io.NewWriter(f)
    var db_slices [][]bow.Bowed
    dec := gob.NewDecoder(&buf)
    err = dec.Decode(&db_slices)
    if err != nil {
        log.Fatal("decode error:", err)
    }
    return db_slices
}

func main() {
    rand.Seed(1)

    db_query, _ := bowdb.Open(searchQuery)
    db_query.ReadAll()
    var query bow.Bowed
    query = db_query.Entries[0]

    fmt.Println(fmt.Sprintf("%d: Opening centers library",timer()))
    db_centers, _ :=  bowdb.Open(fragmentLibraryLoc)
    db_centers.ReadAll()

    fmt.Println(fmt.Sprintf("%d: Opening cluster libraries",timer()))
    /*numCenters := len(db_centers.Entries)
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
    enc_gob_ss_db(db_slices,"clusters.gob")
    */
    fmt.Println(fmt.Sprintf("%d: Unserializing gob",timer()))
    db_slices := dec_gob_ss_db("clusters.gob")
    var m map[string]int
    m = make(map[string]int)
    for i, center := range db_centers.Entries {
        m[center.Id] = i
    }

    sortBy := bowdb.SortByEuclid
    if metric == cosineDist {
        sortBy = bowdb.SortByCosine
    }
    var coarse_search = bowdb.SearchOptions{
        Limit:  -1,
        Min:    0.0,
        Max:    (25+float64(maxRadius)),
        SortBy: sortBy,
        Order:  bowdb.OrderAsc,
    }

    var fine_search = bowdb.SearchOptions{
        Limit:  -1,
        Min:    0.0,
        Max:    float64(maxRadius),
        SortBy: bowdb.SortByEuclid,
        Order:  bowdb.OrderAsc,
    }

    fmt.Println(fmt.Sprintf("%d: Computing coarse results",timer()))
    var coarse_results []bowdb.SearchResult
    coarse_results = db_centers.Search(coarse_search, query)
    fmt.Println(len(coarse_results))

    fmt.Println(fmt.Sprintf("%d: Computing fine results",timer()))
    var fine_results []bowdb.SearchResult
    for _, center := range coarse_results {
        //tmp := db_slices[m[center.Id]].Search(fine_search, query)
        //fine_results = append(fine_results,tmp...)
        //center_db.Close()
        for _, entry := range db_slices[m[center.Id]] {
            var dist float64
            result := newSearchResult(query,entry)
            switch metric {
                case cosineDist:
                    dist = result.Cosine
                case euclideanDist:
                    dist = result.Euclid
            }
            if dist <= float64(maxRadius) {
                fine_results = append(fine_results, result)
            }
        }
    }
    fmt.Println(len(fine_results))
/*
    for _, entry := range fine_results {
        fmt.Println(entry.Bowed.Id + fmt.Sprintf(" %f", entry.Euclid))
    }
*/
    fmt.Println(fmt.Sprintf("%d: Opening long results database",timer()))
    db, _ := bowdb.Open(potentialTargetsLoc)
    fmt.Println(fmt.Sprintf("%d: Computing long results",timer()))
    var long_results []bowdb.SearchResult
    long_results = db.Search(fine_search, query)
    fmt.Println(len(long_results))
    fmt.Println(fmt.Sprintf("%d: Finished",timer()))
/*
    for _, entry := range long_results {
        fmt.Println(entry.Bowed.Id + fmt.Sprintf(" %f", entry.Euclid))
    }
*/



}
