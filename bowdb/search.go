package bowdb

import (
	"fmt"
	"math"

	"github.com/yunwilliamyu/fragbag/bow"
)

const (
	SortByEuclid = iota
	SortByCosine
)

const (
	OrderAsc = iota
	OrderDesc
)

// SearchOptions corresponds the parameters of a search.
type SearchOptions struct {
	// Limit contrains the number of results returned by the search.
	Limit int

	// Min specifies a minimum score such that any entry with distance
	// to the query below the minimum will not be shown.
	Min float64

	// Max specifies a maximum score such that any entry with distance
	// to the query above the maximum will not be shown.
	Max float64

	// SortBy specifies which metric to sort results by.
	// Currently, only SortByEuclid and SortByCosine are supported.
	SortBy int

	// Order specifies whether the results are returned in ascending (OrderAsc)
	// or descending (OrderDesc) order.
	Order int
}

// SearchDefault provides default search settings. Namely, it restricts the
// result set of a predefined number of hits, and sorts the results by the
// closest distances using Cosine distance.
var SearchDefault = SearchOptions{
	Limit:  25,
	Min:    0.0,
	Max:    math.MaxFloat64,
	SortBy: SortByCosine,
	Order:  OrderAsc,
}

// SearchClose provides search settings that limit results by closeness instead
// of by number.
var SearchClose = SearchOptions{
	Limit:  -1,
	Min:    0.0,
	Max:    0.35,
	SortBy: SortByCosine,
	Order:  OrderAsc,
}

// SearchResult corresponds to a single result returned from a search.
// It embeds a Bowed result (which includes meta data about the entry) along
// with values for all distance metrics.
type SearchResult struct {
	bow.Bowed
	Cosine, Euclid float64
}

func newSearchResult(query, entry bow.Bowed) SearchResult {
	return SearchResult{
		Bowed:  entry,
		Cosine: query.Bow.Cosine(entry.Bow),
		Euclid: query.Bow.Euclid(entry.Bow),
	}
}

// Search performs an exhaustive search against the query entry. The best N
// results are returned with respect to the options given. The query given
// must have been computed with this database's fragment library.
//
// Note that if the ReadAll method hasn't been called before, Search will
// call it for you. (This means that the first search could take longer than
// one would otherwise expect.)
//
// It is safe to call Search on the same database from multiple goroutines.
func (db *DB) Search(opts SearchOptions, query bow.Bowed) []SearchResult {
	tree := new(bst)

	if db.Entries == nil {
		db.ReadAll()
	}
	for _, entry := range db.Entries {
		// Compute the distance between the query and the target.
		var dist float64
		switch opts.SortBy {
		case SortByCosine:
			dist = query.Bow.Cosine(entry.Bow)
		case SortByEuclid:
			dist = query.Bow.Euclid(entry.Bow)
		default:
			panic(fmt.Sprintf("Unrecognized SortBy value: %d", opts.SortBy))
		}

		// If the distance isn't in the min/max thresholds specified, skip it.
		if dist > opts.Max || dist < opts.Min {
			continue
		}

		// If there is a limit and we're already at that limit, then
		// we'll skip inserting this element if it's not better than the
		// worst hit.
		if tree.size == opts.Limit {
			if opts.Order == OrderAsc && dist >= tree.max.distance {
				continue
			} else if opts.Order == OrderDesc && dist <= tree.min.distance {
				continue
			}
		}

		// This target is good enough, add it to our results.
		tree.insert(entry, dist)

		// This element is good enough, so lets throw away the worst
		// result we have.
		if opts.Limit >= 0 && tree.size == opts.Limit+1 {
			if opts.Order == OrderAsc {
				tree.deleteMax()
			} else {
				tree.deleteMin()
			}
		}

		// Sanity check.
		if opts.Limit >= 0 && tree.size > opts.Limit {
			panic(fmt.Sprintf("Tree size (%d) is bigger than limit (%d).",
				tree.size, opts.Limit))
		}
	}

	results := make([]SearchResult, tree.size)
	i := 0
	if opts.Order == OrderAsc {
		tree.root.inorder(func(n *node) {
			results[i] = newSearchResult(query, n.Bowed)
			i += 1
		})
	} else {
		tree.root.inorderReverse(func(n *node) {
			results[i] = newSearchResult(query, n.Bowed)
			i += 1
		})
	}
	return results
}
