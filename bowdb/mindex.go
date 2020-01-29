package bowdb

import (
    "bytes"
    "encoding/gob"
    "fmt"
    "io/ioutil"
    "log"
    "math/rand"
    "os"
    "sort"

    "github.com/yunwilliamyu/esfragbag/bow"
)

type DistType int
const (
    CosineDist DistType = iota
    EuclideanDist = iota
)

// Outputs k random objects from the set
func RandomKSample(db []bow.Bowed, optDist DistType, k int) []bow.Bowed {
    results := make([]bow.Bowed, k)
    perm := rand.Perm(len(db))
    for i := 0; i<k; i++ {
        results[i] = db[perm[i]]
    }
    return results
}

// Convert <=16 entry int4 array into int64
func array2scalar(array []int) int64 {
    val := int64(0)
    for _, e := range array {
        val = val*16 + int64(e)
    }
    return val
}


//===========================
// Argsort implementation from
// https://stackoverflow.com/questions/31141202/get-the-indices-of-the-array-after-sorting-in-golang
type Slice struct {
    sort.Float64Slice
    idx []int
}

func (s Slice) Swap(i, j int) {
    s.Float64Slice.Swap(i, j)
    s.idx[i], s.idx[j] = s.idx[j], s.idx[i]
}

func NewSlice(n ...float64) *Slice {
	s := &Slice{Float64Slice: sort.Float64Slice(n), idx: make([]int, len(n))}
	for i := range s.idx {
		s.idx[i] = i
	}
	return s
}

//===========================

// Creates an metric-index hash of x, based on anchor points given in db
// Note that len(db) <= 16
func MIndexHash(db []bow.Bowed, x bow.Bowed, optDist DistType) []int64 {
    dists := make([]float64, len(db))
    for i := 0; i<len(db); i++ {
        xi := db[i]
        switch optDist {
            case CosineDist:
                dists[i] = xi.Bow.Cosine(x.Bow)
            case EuclideanDist:
                dists[i] = xi.Bow.Euclid(x.Bow)
            default:
                panic(fmt.Sprintf("Unrecognized DistType value: %d", optDist))
        }
    }
	s := NewSlice(sort.Float64Slice(dists)...)
	sort.Sort(s)
    s_idx := make([]int, len(s.idx))
    // Forward
    copy(s_idx, s.idx)
    full_hash := make([]int64, len(db)*2)
    for j :=0; j<len(db); j++ {
        s_idx = remove_val(s_idx, len(db)-j)
        full_hash[len(db) - j - 1] = array2scalar(s_idx)
    }
    // Reverse
    for j :=0; j<len(db); j++ {
        s.idx = remove_val(s.idx, j-1)
        full_hash[2*len(db) - j - 1] = array2scalar(s.idx)
    }
    //fmt.Println(s.Float64Slice, s.idx)
    return full_hash
}

// Removes first instance of val in slice
// Does nothing if val does not exist in slice
func remove_val(slice []int, val int) []int {
    s := -1
    for i, v := range slice {
        if v==val {
            s = i
            break
        }
    }
    var newslice []int
    if s > -1 {
        newslice = append(slice[:s], slice[s+1:]...)
    } else {
        newslice = slice
    }
    return newslice
}


// Creates a hash table indexed by the m-index into all cluster centers that match
func MIndexTable(db []bow.Bowed, anchors []bow.Bowed, optDist DistType) []map[int64][]int {
    table := make([]map[int64][]int, len(anchors)*2)
    for j:=0; j<len(anchors)*2; j++ {
        table[j] = make(map[int64][]int)
    }

    for i, xi := range(db) {
        full_hash := MIndexHash(anchors, xi, optDist)
        for j:=0; j <len(anchors)*2; j++ {
            h := full_hash[j]
            table[j][h] = append(table[j][h], i)
        }
    }
    return table
}

type M_index_table struct {
    Anchors []bow.Bowed
    Elements []bow.Bowed
    Table []map[int64][]int
}

func Enc_gob_mindex(mindex M_index_table, name string) {
    f, err := os.Create(name)
    defer f.Close()
    if err != nil {
        log.Fatal("Create file error:", err)
    }
    var buf bytes.Buffer
    enc := gob.NewEncoder(&buf)
    err = enc.Encode(mindex)
    if err != nil {
        log.Fatal("encode error:", err)
    }
    _, err = f.Write(buf.Bytes())
}

func Dec_gob_mindex(name string) M_index_table {
    buf_bytes, err := ioutil.ReadFile(name)
    if err != nil {
        log.Fatal("Open file error:", err)
    }
    var buf bytes.Buffer
    buf.Write(buf_bytes)
    var mindex M_index_table
    dec := gob.NewDecoder(&buf)
    err = dec.Decode(&mindex)
    if err != nil {
        log.Fatal("decode error:", err)
    }
    return mindex
}

// Finds all items within radius of query
func RangeQuery(r float64, query bow.Bowed, db []bow.Bowed, metric DistType) []bow.Bowed {
    var results []bow.Bowed
    for _, v := range db {
        var dist float64
        switch metric {
            case CosineDist:
                dist = query.Bow.Cosine(v.Bow)
            case EuclideanDist:
                dist = query.Bow.Euclid(v.Bow)
        }
        if dist <= float64(r) {
            results = append(results, v)
        }
    }
    return results
}

// Index by slice. i.e. give a new db with candidates given by the indexes
func IndexBySlice(db []bow.Bowed, indexes []int) []bow.Bowed {
    candidates := make([]bow.Bowed, len(indexes))
    for i, v := range indexes {
        candidates[i] = db[v]
    }
    return candidates
}

// Combine two unique int arrays into sorted unique array
func MergeUnique(A []int, B []int) []int {
    sort.Ints(A)
    sort.Ints(B)
    C := make([]int, 0, len(A) + len(B))
    ai := 0
    bi := 0
    for (ai < len(A) && bi < len(B)) {
        if A[ai] < B[bi] {
            C = append(C, A[ai])
            ai = ai + 1
        } else if A[ai] > B[bi] {
            C = append(C, B[bi])
            bi = bi + 1
        } else {
            C = append(C, A[ai])
            ai = ai + 1
            bi = bi + 1
        }
    }
    return C
}
