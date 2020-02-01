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
    sort.Interface
    Idx []int
}

func (s Slice) Swap(i, j int) {
    s.Interface.Swap(i, j)
    s.Idx[i], s.Idx[j] = s.Idx[j], s.Idx[i]
}

func NewSlice(n ...float64) *Slice {
	s := &Slice{Interface: sort.Float64Slice(n), Idx: make([]int, len(n))}
	for i := range s.Idx {
		s.Idx[i] = i
	}
	return s
}

type SliceI struct {
    sort.IntSlice
    Idx []int
}

func (s SliceI) Swap(i, j int) {
    s.IntSlice.Swap(i, j)
    s.Idx[i], s.Idx[j] = s.Idx[j], s.Idx[i]
}

func NewSliceI(n ...int) *SliceI {
	s := &SliceI{IntSlice: sort.IntSlice(n), Idx: make([]int, len(n))}
	for i := range s.Idx {
		s.Idx[i] = i
	}
	return s
}

//===========================

// Creates an metric-index hash of x, based on anchor points given in db
// Note that len(db) <= 16
func MIndexHash(db []bow.Bowed, x bow.Bowed, optDist DistType) []int {
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
    s_Idx := make([]int, len(s.Idx))
    copy(s_Idx, s.Idx)
    return s_Idx
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
func MIndexTable(db []bow.Bowed, anchors []bow.Bowed, optDist DistType) map[int64][]int {
    table := make(map[int64][]int)

    for i, xi := range(db) {
        full_hash := MIndexHash(anchors, xi, optDist)
        h := array2scalar(full_hash)
        table[h] = append(table[h], i)
    }
    return table
}

func MIndexHashes(db []bow.Bowed, anchors[]bow.Bowed, optDist DistType) [][]int {
    hashes := make([][]int, len(db))
    for i, xi := range(db) {
        full_hash := MIndexHash(anchors, xi, optDist)
        hashes[i] = full_hash
    }
    return hashes
}

type M_index_table struct {
    Anchors []bow.Bowed
    Elements []bow.Bowed
    Hashes [][]int
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

// Find indexes equal to a value
func ExactIndexes(r int, db []int, start int) []int {
    indexes := make([]int, 0, len(db))
    for i, v := range db{
        if v == r {
            indexes = append(indexes, i)
        }
    }
    return indexes
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


// Kendall-Tau distance between two permutations (note, the permutations must be 0-indexed and the same length)
func KendallTau(A []int, B []int) (inv int) {
    key := make([]int, len(A))
    for i, v := range A {
        key[v]=i
    }
    renamedB := make([]int, len(B))
    for i, v := range B{
        renamedB[i] = key[v]
    }
    _, inv = mergeSort(renamedB)
    return
}

func mergeSort(items []int) (list []int, inv int) {
    inv = 0
    var num = len(items)
    if num == 1 {
        return items, inv
    }
    middle := int(num/2)
    var (
        left = make([]int, middle)
        right = make([]int, num-middle)
    )
    for i := 0; i<num; i++ {
        if i < middle {
            left[i] = items[i]
        } else {
            right[i-middle] = items[i]
        }
    }
    list_left, inv_left := mergeSort(left)
    list_right, inv_right := mergeSort(right)
    //list := merge(mergeSort(left), mergeSort(right))
    list, inv_center := merge(list_left, list_right)
    inv = inv_left + inv_right + inv_center
    return
}

func merge(left, right []int) (result []int, inv int) {
    result = make([]int, len(left) + len(right))
    inv = 0
    i := 0
    for len(left) > 0 && len(right) > 0 {
        if left[0] < right[0] {
            result[i] = left[0]
            left = left[1:]
        } else {
            result[i] = right[0]
            right = right[1:]
            inv = inv + len(left)
        }
        i++
    }

    for j := 0; j<len(left); j++ {
        result[i] = left[j]
        i++
    }
    for j := 0; j<len(right); j++ {
        result[i] = right[j]
        i++
    }
    return
}


