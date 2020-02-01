package main

import (
    "flag"
    "log"
    "sort"
    "fmt"
    "time"
    "bytes"
    "io/ioutil"
    "encoding/gob"
    "math/rand"

    "github.com/yunwilliamyu/esfragbag/bow"
    "github.com/yunwilliamyu/esfragbag/bowdb"

)

var (
    fragmentLibraryLoc = ""
    searchQuery = ""
    metric = bowdb.CosineDist
    metricFlag = ""
    potentialTargetsLoc = ""
    maxRadius = 0.0
    clusterRadius = 10000
    lasttime = time.Now().UTC().UnixNano()
    gobLoc = "clusters.gob"
    mindexLoc = "mindex.gob"
)


func init() {
    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library centers")
    flag.StringVar(&gobLoc, "clusters", gobLoc, "the location of the serialized clusters database")
    flag.StringVar(&mindexLoc, "mIndex", mindexLoc, "the location of the mIndex of clusters")
    flag.StringVar(&searchQuery, "searchQuery", searchQuery, "the search query library as a bowdb")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")
    flag.StringVar(&potentialTargetsLoc, "potentialTargets", potentialTargetsLoc, "the location of the full fragment library database")
    flag.Float64Var(&maxRadius, "maxRadius", maxRadius, "maximum radius to search in")
    flag.IntVar(&clusterRadius, "clusterRadius", clusterRadius, "maximum cluster radius in database")

    flag.Parse()

    if metricFlag == "cosine" {
        metric = bowdb.CosineDist
    }
    if metricFlag == "euclidean" {
        metric = bowdb.EuclideanDist
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

type m_index_table struct {
    Anchors []bow.Bowed
    Table map[int64][]bow.Bowed
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
    db_slices := dec_gob_ss_db(gobLoc)
    mindex := bowdb.Dec_gob_mindex(mindexLoc)
    var m map[string]int
    m = make(map[string]int)
    for i, center := range db_centers.Entries {
        m[center.Id] = i
    }
    fmt.Println(fmt.Sprintf("\t%d",timer()))

    sortBy := bowdb.SortByEuclid
    if metric == bowdb.CosineDist {
        sortBy = bowdb.SortByCosine
    }
/*
    var coarse_search = bowdb.SearchOptions{
        Limit:  -1,
        Min:    0.0,
        Max:    (float64(clusterRadius)+float64(maxRadius)),
        SortBy: sortBy,
        Order:  bowdb.OrderAsc,
    }
    */

    var fine_search = bowdb.SearchOptions{
        Limit:  -1,
        Min:    0.0,
        Max:    float64(maxRadius),
        SortBy: sortBy,
        Order:  bowdb.OrderAsc,
    }

    fmt.Println("Computing mindex hash")
    var coarse_results []bow.Bowed

    coarse_results_time := timer()
    //a_num := len(mindex.Anchors)
    not_break := true
    fmt.Println("Computing coarse results")
    h := bowdb.MIndexHash(mindex.Anchors, query, metric)
    kt_dists := make([]int, len(mindex.Elements))
    for i, v := range mindex.Hashes {
        kt_dists[i] = bowdb.KendallTau(h, v)
    }
	s := bowdb.NewSliceI(sort.IntSlice(kt_dists)...)
	sort.Sort(s)
    r := 0
    i := 0
    start := 0
    for not_break {
        for s.IntSlice[i] == r {
            i = i + 1
        }
        candidates := bowdb.IndexBySlice(mindex.Elements, s.Idx[start:i])
        start = i
        candidates_filtered := bowdb.RangeQuery(float64(clusterRadius)+float64(maxRadius), query, candidates, metric)
        fmt.Println(fmt.Sprintf("\tlevel %d, %d candidates, %d filtered", r, len(candidates), len(candidates_filtered)) )
        if len(candidates_filtered)<len(coarse_results)/25 {
            if len(coarse_results)>0 {
                not_break = false
            }
        }
        coarse_results = append(coarse_results, candidates_filtered...)
        r = r+1
    }
    /*
    i := 0
    for not_break {
        var candidates []bow.Bowed
        indexes := bowdb.ExactIndexes(i, kt_dists)
        candidates = bowdb.IndexBySlice(mindex.Elements, indexes)
        candidates_filtered := bowdb.RangeQuery(float64(clusterRadius)+float64(maxRadius), query, candidates, metric)
        fmt.Println(fmt.Sprintf("\tlevel %d, %d candidates, %d filtered", i, len(candidates), len(candidates_filtered)) )
        if len(candidates_filtered)<len(coarse_results)/100 {
            if len(coarse_results)>0 {
                not_break = false
            }
        }
        coarse_results = append(coarse_results, candidates_filtered...)
        i = i+1
    }
    */
    fmt.Println(fmt.Sprintf("\t%d",coarse_results_time))
    fmt.Println(fmt.Sprintf("\tCount: %d",len(coarse_results)))


    fmt.Println("Computing fine results")
    var fine_results []bowdb.SearchResult
    for _, center := range coarse_results {
        for _, entry := range db_slices[m[center.Id]] {
            var dist float64
            switch metric {
                case bowdb.CosineDist:
                    dist = query.Bow.Cosine(entry.Bow)
                case bowdb.EuclideanDist:
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
