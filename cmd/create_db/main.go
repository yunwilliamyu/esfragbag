package main

import (
    "flag"
    "log"
    "fmt"
    //"strconv"
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
    metric = cosineDist
    numCenters = -1
    metricFlag = ""
)


func init() {
    log.SetFlags(0)

    flag.StringVar(&fragmentLibraryLoc, "fragLib", fragmentLibraryLoc, "the location of the fragment library")
    flag.IntVar(&numCenters, "numCenters",  numCenters, "the number of centers to choose for metric k-centers")
    flag.StringVar(&metricFlag, "metricFlag", metricFlag, "Choice of metric to use; valid options are 'cosine' and 'euclidean'")

    flag.Parse()

    if metricFlag == "cosine" {
        metric = cosineDist
    }
    if metricFlag == "euclidean" {
        metric = euclideanDist
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
    var minDist float64
    minDist = 1000000
    var bestResult bow.Bowed
    var bestIndex int
    for i, center := range set {
            var dist float64
            // Compute the distance between the query and the target.
            switch optDist {
            case cosineDist:
                dist = center.Bow.Cosine(query.Bow)
            case euclideanDist:
                dist = center.Bow.Euclid(query.Bow)
            default:
                panic(fmt.Sprintf("Unrecognized distType value: %d", optDist))
            }
            //fmt.Println(fmt.Sprintf("%f",dist))
            if dist < minDist {
                minDist = dist
                bestResult = center
                bestIndex = i
            }
    }
    return minDist, bestResult, bestIndex
}

func main() {
    db, _ :=  bowdb.Open(fragmentLibraryLoc)
    db.ReadAll()
    //Assert(err, "Could not open BOW database '%s'", path) 
    kCenters := metricKCenter(db.Entries, metric, numCenters)

    db_codes := make([]int,len(db.Entries))
    distances := make([]float64,len(db.Entries))
    for j, entry := range db.Entries {
        dist, _, i := distanceFromSet (metric, entry, kCenters)
        db_codes[j]=i
        distances[j]=dist
        //fmt.Println(strconv.Itoa(i) + " " + strconv.Itoa(j) + " " + strconv.FormatFloat(dist,'f',5,32))
    }

    db_centers, _ := bowdb.Create(db.Lib, "centers.cluster.db")
    db_slices := make([]*bowdb.DB,numCenters,numCenters)
    for i, center := range kCenters {
        tmp, _ := bowdb.Create(db.Lib, center.Id + ".cluster.db")
        db_slices[i] = tmp
        db_centers.Add(center)
    }
    db_centers.Close()

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
