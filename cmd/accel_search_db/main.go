package main

import (
    "flag"
    "log"
    "fmt"
    "time"
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
    clusterRadius = 10000
    lasttime = time.Now().UTC().UnixNano()
    gobLoc = "clusters.gob"
)


func init() {
    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library centers")
    flag.StringVar(&gobLoc, "clusters", gobLoc, "the location of the serialized clusters database")
    flag.StringVar(&searchQuery, "searchQuery", searchQuery, "the search query library as a bowdb")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")
    flag.StringVar(&potentialTargetsLoc, "potentialTargets", potentialTargetsLoc, "the location of the full fragment library database")
    flag.IntVar(&maxRadius, "maxRadius", maxRadius, "maximum radius to search in")
    flag.IntVar(&clusterRadius, "clusterRadius", clusterRadius, "maximum cluster radius in database")

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

func dec_gob_ss_db(name string) [][]bow.Bowed {
    buf_bytes, err := ioutil.ReadFile(name)
    if err != nil {
        log.Fatal("Open file error:", err)
    }
    var buf bytes.Buffer
    buf.Write(buf_bytes)
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

    fmt.Println("Loading query")
    db_query, _ := bowdb.Open(searchQuery)
    db_query.ReadAll()
    var query bow.Bowed
    query = db_query.Entries[0]
    fmt.Println(fmt.Sprintf("\t%d",timer()))

    fmt.Println(fmt.Sprintf("Opening centers library"))
    db_centers, _ :=  bowdb.Open(fragmentLibraryLoc)
    db_centers.ReadAll()
    fmt.Println(fmt.Sprintf("\t%d",timer()))

    fmt.Println("Unserializing gob")
    db_slices := dec_gob_ss_db("clusters.gob")
    var m map[string]int
    m = make(map[string]int)
    for i, center := range db_centers.Entries {
        m[center.Id] = i
    }
    fmt.Println(fmt.Sprintf("\t%d",timer()))

    sortBy := bowdb.SortByEuclid
    if metric == cosineDist {
        sortBy = bowdb.SortByCosine
    }

    var coarse_search = bowdb.SearchOptions{
        Limit:  -1,
        Min:    0.0,
        Max:    (float64(clusterRadius)+float64(maxRadius)),
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

    fmt.Println("Computing coarse results")
    var coarse_results []bowdb.SearchResult
    coarse_results = db_centers.Search(coarse_search, query)
    coarse_results_time := timer()
    fmt.Println(fmt.Sprintf("\t%d",coarse_results_time))
    fmt.Println(fmt.Sprintf("\tCount: %d",len(coarse_results)))


    fmt.Println("Computing fine results")
    var fine_results []bowdb.SearchResult
    for _, center := range coarse_results {
        for _, entry := range db_slices[m[center.Id]] {
            var dist float64
            switch metric {
                case cosineDist:
                    dist = query.Bow.Cosine(entry.Bow)
                case euclideanDist:
                    dist = query.Bow.Euclid(entry.Bow)
            }
            if dist <= float64(maxRadius) {
                result := newSearchResult(query,entry)
                fine_results = append(fine_results, result)
            }
        }
    }
    fine_results_time := timer()
    fmt.Println(fmt.Sprintf("\t%d",fine_results_time))
    fmt.Println(fmt.Sprintf("\tCount: %d",len(fine_results)))

    fmt.Println("Opening long results database")
    db, _ := bowdb.Open(potentialTargetsLoc)
    db.ReadAll()
    fmt.Println(fmt.Sprintf("\t%d",timer()))

    fmt.Println("Computing long results")
    var long_results []bowdb.SearchResult
    long_results = db.Search(fine_search, query)
    long_results_time := timer()
    fmt.Println(fmt.Sprintf("\t%d",long_results_time))
    fmt.Println(fmt.Sprintf("\tCount: %d",len(long_results)))

    fmt.Println("")
    fmt.Println(fmt.Sprintf("Accel:\t%d",coarse_results_time+fine_results_time))
    fmt.Println(fmt.Sprintf("Naive:\t%d",long_results_time))
}
