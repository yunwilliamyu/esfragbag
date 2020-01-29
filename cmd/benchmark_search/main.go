package main

import (
    "flag"
    "log"
    "fmt"
    "time"
//    "os"
    "bytes"
    "io/ioutil"
    //"strconv"
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
    searchQuery = ""
    metric = cosineDist
    metricFlag = ""
    potentialTargetsLoc = ""
    clusterRadius = 10000.0
    lasttime = time.Now().UTC().UnixNano()
    gobLoc = "clusters.gob"
    repeatNum = 10
)


func init() {
    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library centers")
    flag.StringVar(&gobLoc, "clusters", gobLoc, "the location of the serialized clusters database")
    flag.StringVar(&searchQuery, "searchQuery", searchQuery, "the search query library as a bowdb")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")
    flag.StringVar(&potentialTargetsLoc, "potentialTargets", potentialTargetsLoc, "the location of the full fragment library database")
    flag.Float64Var(&clusterRadius, "clusterRadius", clusterRadius, "maximum cluster radius in database")
    flag.IntVar(&repeatNum, "repeatNum", repeatNum, "number of trials for each data point (default 10)")

    flag.Parse()

    if metricFlag == "cosine" {
        metric = cosineDist
    }
    if metricFlag == "euclidean" {
        metric = euclideanDist
    }
}

type m_index_table struct {
    Anchors []bow.Bowed
    Table map[int64][]int
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

func dec_gob_mindex(name string) m_index_table {
    buf_bytes, err := ioutil.ReadFile(name)
    if err != nil {
        log.Fatal("Open file error:", err)
    }
    var buf bytes.Buffer
    buf.Write(buf_bytes)
    var mindex m_index_table
    dec := gob.NewDecoder(&buf)
    err = dec.Decode(&mindex)
    if err != nil {
        log.Fatal("decode error:", err)
    }
    return mindex
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

func averageInt2F64(xs []int) float64 {
    total := float64(0)
    for _, x := range xs {
        total += float64(x)
    }
    return total / float64(len(xs))
}

func averageInt642F64(xs []int64) float64 {
    total := float64(0)
    for _, x := range xs {
        total += float64(x)
    }
    return total / float64(len(xs))
}

func main() {
    rand.Seed(1)

    db_query, _ := bowdb.Open(searchQuery)
    db_query.ReadAll()
    var query bow.Bowed
    query = db_query.Entries[0]

    db_centers, _ :=  bowdb.Open(fragmentLibraryLoc)
    db_centers.ReadAll()

    db_slices := dec_gob_ss_db("clusters.gob")
    var m map[string]int
    m = make(map[string]int)
    for i, center := range db_centers.Entries {
        m[center.Id] = i
    }
/*
    sortBy := bowdb.SortByEuclid
    if metric == cosineDist {
        sortBy = bowdb.SortByCosine
    }
*/

    db, _ := bowdb.Open(potentialTargetsLoc)
    db.ReadAll()

    //repeatNum := 10 // How many times to repeat each run for timing purposes
    fmt.Println("Radius\tAccelCount\tLongCount\tAccel\tNaive\tSpeedup\tSensitivity\tFineCandidates")
    for maxR := 0; maxR < 50; maxR=maxR+1 {
        maxRadius := 0.0
        if metric==cosineDist {
            maxRadius = float64(maxR) / 100.0
        } else {
            maxRadius = float64(maxR)
        }
        coarse_radius := float64(clusterRadius)+float64(maxRadius)

        accelCount := make([]int, repeatNum)
        longCount := make([]int, repeatNum)
        accelTime := make([]int64, repeatNum)
        naiveTime := make([]int64, repeatNum)

        fineCandidates := make([]int, repeatNum)

        for rep := 0; rep < repeatNum; rep++ {
            timer()
            var coarse_results []bowdb.SearchResult
            //coarse_results = db_centers.Search(coarse_search, query)
            for _, entry := range db_centers.Entries {
                var dist float64
                switch metric {
                    case cosineDist:
                        dist = query.Bow.Cosine(entry.Bow)
                    case euclideanDist:
                        dist = query.Bow.Euclid(entry.Bow)
                }
                if dist <= coarse_radius {
                    result := newSearchResult(query,entry)
                    coarse_results = append(coarse_results, result)
                }
            }
            coarse_results_time := timer()

            var fine_results []bowdb.SearchResult
            fine_candidates := 0
            for _, center := range coarse_results {
                fine_candidates += len(db_slices[m[center.Id]])
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

            var long_results []bowdb.SearchResult
            //long_results = db.Search(fine_search, query)
            for _, entry := range db.Entries {
                var dist float64
                switch metric {
                    case cosineDist:
                        dist = query.Bow.Cosine(entry.Bow)
                    case euclideanDist:
                        dist = query.Bow.Euclid(entry.Bow)
                }
                if dist <= float64(maxRadius) {
                    result := newSearchResult(query,entry)
                    long_results = append(long_results, result)
                }
            }

            long_results_time := timer()
            /*if (len(long_results)!=len(fine_results)) {
                err := "Fine and long searches did not match."
                fmt.Fprintf(os.Stderr, "error: %v\n", err)
                fmt.Fprintf(os.Stderr, "Fine: %v\n", len(fine_results))
                fmt.Fprintf(os.Stderr, "Long: %v\n", len(long_results))
                os.Exit(1)
            } */
            long_count := len(long_results)
            fine_count := len(fine_results)
            accel_time := coarse_results_time + fine_results_time

            accelCount[rep] = fine_count
            longCount[rep] = long_count
            accelTime[rep] = accel_time
            naiveTime[rep] = long_results_time
            fineCandidates[rep] = fine_candidates
        }
        accelCountAvg := averageInt2F64(accelCount)
        naiveCountAvg := averageInt2F64(longCount)
        accelTimeAvg := averageInt642F64(accelTime)
        naiveTimeAvg := averageInt642F64(naiveTime)
        sensitivity := accelCountAvg / naiveCountAvg
        speedup := naiveTimeAvg / accelTimeAvg
        fineSearchCount := averageInt2F64(fineCandidates)
        fmt.Println(fmt.Sprintf("%f\t%f\t%f\t%f\t%f\t%f\t%f\t%f",maxRadius,accelCountAvg,naiveCountAvg,accelTimeAvg,naiveTimeAvg,speedup,sensitivity,fineSearchCount))
    }

}
