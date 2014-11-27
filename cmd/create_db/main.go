package main

import (
    "flag"
    "log"
    "fmt"
    "time"
    //"strconv"
    "math/rand"
    "runtime"

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
)

var (
    fragmentLibraryLoc = ""
    metric = cosineDist
    numCenters = -1
    metricFlag = ""
    centerType = randomSelec
    kCenterAlg = ""
)


func init() {
    rand.Seed(time.Now().UTC().UnixNano())

    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library")
    flag.IntVar(&numCenters, "numCenters",  numCenters, "the number of centers to choose for metric k-centers")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")
    flag.StringVar(&kCenterAlg, "kCenterAlg", kCenterAlg, "Choice of which KCenter algorithm to use; valid options are 'metricApprox' and 'random'")

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

}

// Outputs k cluster centers for a bowdb using the greedy metric k-center
// approximation algorithm of iteratively choosing the furthest away point
func metricKCenter(db []bow.Bowed, optDist distType, k int) []bow.Bowed {
    results := make([]bow.Bowed, k)
    results[0] = db[rand.Intn(len(db))]
    for i := 1; i<k; i++ {
        results[i] = newKCenter(db, optDist, results[0:i])
    }
    return results
}

func randomKCenter(db []bow.Bowed, optDist distType, k int) []bow.Bowed {
    results := make([]bow.Bowed, k)
    perm := rand.Perm(len(db))
    for i := 0; i<k; i++ {
        results[i] = db[perm[i]]
    }
    return results
}

// Compute the point that's the maximum distance from any center
func newKCenter(db []bow.Bowed, optDist distType, prevKCenters []bow.Bowed) bow.Bowed {
    var maxDist float64
    var bestResult bow.Bowed
    for _, entry := range db {
        var dist float64
        dist, _, _ = distanceFromSet (optDist, entry, prevKCenters)
        if (dist > maxDist)&&(entry.Bow.String()!="{}") {
            maxDist = dist
            bestResult = entry
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

func main() {
    runtime.GOMAXPROCS(8)

    db, _ :=  bowdb.Open(fragmentLibraryLoc)
    db.ReadAll()
    //Assert(err, "Could not open BOW database '%s'", path) 
    var kCenters []bow.Bowed
    fmt.Println("Generating cluster centers")
    if centerType == randomSelec {
        kCenters = randomKCenter(db.Entries, metric, numCenters)
    } else {
        kCenters = metricKCenter(db.Entries, metric, numCenters)
    }

    fmt.Println("Computing distances from cluster centers")
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
        if i%1000 == 0 {
            fmt.Println(fmt.Sprintf("%i", i))
        }
    }

    fmt.Println("Writing out centers.cluster.db")
    db_centers, _ := bowdb.Create(db.Lib, "centers.cluster.db")
    db_slices := make([]*bowdb.DB,numCenters,numCenters)
    for i, center := range kCenters {
        tmp, _ := bowdb.Create(db.Lib, center.Id + ".cluster.db")
        db_slices[i] = tmp
        db_centers.Add(center)
    }
    db_centers.Close()

    fmt.Println("Writing out individual cluster dbs")
    cluster_radii := make([]float64,numCenters)
    for j, entry := range db.Entries {
        db_slices[db_codes[j]].Add(entry)
        if distances[j] > cluster_radii[db_codes[j]] {
            cluster_radii[db_codes[j]] = distances[j]
        }
    }

    for _, slice := range db_slices {
        slice.Close()
    }

    for j, entry := range kCenters {
        fmt.Println(entry.Id + fmt.Sprintf(": %f", cluster_radii[j]))
    }




}
