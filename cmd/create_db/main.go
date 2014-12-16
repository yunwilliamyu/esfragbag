package main

import (
    "flag"
    "log"
    "fmt"
    "time"
    //"strconv"
    "math/rand"
    "runtime"
    "encoding/gob"
    "bytes"
    "os"

    "github.com/yunwilliamyu/fragbag/bow"
    "github.com/yunwilliamyu/fragbag/bowdb"
)

type distType int
type empty struct{}

const (
    cosineDist distType = iota
    euclideanDist = iota
)

type algType int
const (
    randomSelec algType = iota
    metricApprox = iota
    halfhalf = iota
)

var (
    fragmentLibraryLoc = ""
    metric = cosineDist
    numCenters = -1
    metricFlag = ""
    centerType = randomSelec
    kCenterAlg = ""
    maxRadius = -1.0
    lasttime = time.Now().UTC().UnixNano()
)


func init() {
    rand.Seed(time.Now().UTC().UnixNano())

    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library")
    flag.IntVar(&numCenters, "numCenters",  numCenters, "the number of centers to choose for metric k-centers")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")
    flag.StringVar(&kCenterAlg, "kCenterAlg", kCenterAlg, "Choice of which KCenter algorithm to use; valid options are 'metricApprox', 'random', and 'halfhalf'")
    flag.Float64Var(&maxRadius, "maxRadius", maxRadius, "maximum cluster radius as an float; if set, this will supercede numCenters")

    flag.Parse()

    if metricFlag == "cosine" {
        metric = cosineDist
    }
    if metricFlag == "euclidean" {
        metric = euclideanDist
    }

    if kCenterAlg == "metricApprox" {
        centerType = metricApprox
    }
    if kCenterAlg == "random" {
        centerType = randomSelec
    }
    if kCenterAlg == "halfhalf" {
        centerType = halfhalf
    }

}

func maxRadiusKCenter (db []bow.Bowed, optDist distType, r float64) []bow.Bowed {
    results := make([]bow.Bowed, 0, 1000)
    perm := rand.Perm(len(db))
    results = append(results, db[perm[0]])
    radius := float64(r)
    for i:=1; i<len(db); i++ {
        dist, _, _ := distanceFromSet(optDist, db[perm[i]], results)
        if dist>radius {
            results = append(results, db[perm[i]])
        }
    }
    return results
}

// Outputs k random objects from the set
func randomKCenter(db []bow.Bowed, optDist distType, k int) []bow.Bowed {
    results := make([]bow.Bowed, k)
    perm := rand.Perm(len(db))
    for i := 0; i<k; i++ {
        results[i] = db[perm[i]]
    }
    return results
}

// Outputs k additional cluster centers for a bowdb using the greedy metric k-center
// approximation algorithm of iteratively choosing the furthest away point
// Starts from start_centers
func metricKCenter(db []bow.Bowed, optDist distType, k int, start_centers []bow.Bowed) []bow.Bowed {
    results := make([]bow.Bowed, k + len(start_centers))
    var start int
    if len(start_centers)==0 {
        results[0] = db[rand.Intn(len(db))]
        start = 1
    } else {
        start  = len(start_centers)
    }
    for i := start; i<k+len(start_centers); i++ {
        results[i] = newKCenter(db, optDist, results[0:i])
    }
    return results
}

// Compute the point that's the maximum distance from any center
func newKCenter(db []bow.Bowed, optDist distType, prevKCenters []bow.Bowed) bow.Bowed {
    var bestResult bow.Bowed
    distances := make([]float64,len(db))
    sem := make(chan empty, len(db))
    for i, _ := range db {
        go func (i int) {
           distances[i], _, _ = distanceFromSet (optDist, db[i], prevKCenters)
        } (i);
        sem <- empty{}
    }
    for i := 0; i < len(db); i++ { <-sem }

    var maxDist float64
    for i := 0; i < len(db); i++ {
//        var dist float64
        if (distances[i] > maxDist)&&(db[i].Bow.String()!="{}") {
            maxDist = distances[i]
            bestResult = db[i]
        }
    }

    return bestResult
}

// Computes the distance of a point from a set, the nearest set point, and the index of that point
func distanceFromSet (optDist distType, query bow.Bowed, set []bow.Bowed) (float64, bow.Bowed, int) {
    distances := make([]float64,len(set))
    var bestResult bow.Bowed
    var bestIndex int
 //   sem := make (chan empty, len(set))
    for i, _ := range set {
        xi := set[i]
        switch optDist {
            case cosineDist:
                distances[i] = xi.Bow.Cosine(query.Bow)
            case euclideanDist:
                distances[i] = xi.Bow.Euclid(query.Bow)
            default:
                panic(fmt.Sprintf("Unrecognized distType value: %d", optDist))
        }
    }

    var minDist float64
    minDist = 1000000
    for i, dist := range distances {
        if dist < minDist {
            minDist = dist
            bestIndex = i
        }
    }
    bestResult = set[bestIndex]
    return minDist, bestResult, bestIndex
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
    var kCenters []bow.Bowed
    fmt.Println(fmt.Sprintf("%d: Generating cluster centers", timer()))
    if maxRadius > 0 {
        kCenters = maxRadiusKCenter (db.Entries, metric, maxRadius)
        numCenters = len(kCenters)
    } else if centerType == randomSelec {
        kCenters = randomKCenter(db.Entries, metric, numCenters)
    } else if centerType == metricApprox {
        var start_centers []bow.Bowed
        kCenters = metricKCenter(db.Entries, metric, numCenters, start_centers)
    } else if centerType == halfhalf {
        start_centers := randomKCenter(db.Entries, metric, numCenters/2)
        kCenters = metricKCenter(db.Entries,metric, numCenters - numCenters/2, start_centers)
    }
//    for i, center := range kCenters {
//        fmt.Println(center.Id + fmt.Sprintf(": %d",i))
//    }

    runtime.GOMAXPROCS(20)
    fmt.Println(fmt.Sprintf("%d: Computing distances from cluster centers",timer()))
    db_codes := make([]int,len(db.Entries))
    distances := make([]float64,len(db.Entries))
    sem := make (chan empty, len(db.Entries))
    for j, _ := range db.Entries {
        go func (j int) {
            distances[j], _, db_codes[j] = distanceFromSet (metric, db.Entries[j], kCenters)
            //fmt.Println(strconv.Itoa(i) + " " + strconv.Itoa(j) + " " + strconv.FormatFloat(dist,'f',5,32))
        } (j);
        sem <- empty{}
    }
    for i := 0; i < len(db.Entries); i++ {
        <-sem
    }
    runtime.GOMAXPROCS(20)

    fmt.Println(fmt.Sprintf("%d: Writing out centers.cluster.db", timer()))
    db_centers, _ := bowdb.Create(db.Lib, "centers.cluster.db")
    for _, center := range kCenters {
        db_centers.Add(center)
    }
    db_centers.Close()

    fmt.Println(fmt.Sprintf("%d: Opening centers library",timer()))
    db_centers2, _ :=  bowdb.Open("centers.cluster.db")
    db_centers2.ReadAll()
    var mr map[string]int
    mr = make(map[string]int)
    for i, center := range db_centers2.Entries {
        mr[center.Id]=i
    }

    db_slices := make([][]bow.Bowed,numCenters,numCenters)

    fmt.Println(fmt.Sprintf("%d: Computing individual cluster dbs",timer()))
    for i := 0; i <len(kCenters); i++ {
        //curr_cluster, _ := bowdb.Create(db.Lib, kCenters[i].Id + ".cluster.db")
        //curr_cluster := db_slices[mr[kCenters[i].Id]]
        for j, entry := range db.Entries {
            if (i==db_codes[j]) {
                db_slices[mr[kCenters[i].Id]] = append(db_slices[mr[kCenters[i].Id]],entry)
            }
        }
        //curr_cluster.Close()
    }

    gobLoc := "clusters.gob"
    fmt.Println(fmt.Sprintf("%d: Serializing gob",timer()))
    enc_gob_ss_db(db_slices,gobLoc)

    fmt.Println(fmt.Sprintf("%d: computing cluster radii",timer()))
    cluster_radii := make([]float64,numCenters)
    cluster_count := make([]int,numCenters)
    for j, _ := range db.Entries {
        if distances[j] > cluster_radii[db_codes[j]] {
            cluster_radii[db_codes[j]] = distances[j]
        }
        cluster_count[db_codes[j]]++
    }


    for j, entry := range kCenters {
        fmt.Println(entry.Id + fmt.Sprintf("\t%f\t%d", cluster_radii[j], cluster_count[j] ))
    }
    fmt.Println(fmt.Sprintf("%d: Finished!!",timer()))




}
