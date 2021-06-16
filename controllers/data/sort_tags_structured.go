package data

import "sort"

// from https://golang.org/pkg/sort/#pkg-overview

type lessFunc func(p1, p2 map[string]string) bool

// multiSorter implements the Sort interface, sorting the tags within.
type multiSorter struct {
	tags []map[string]string
	less []lessFunc
}

// Sort sorts the argument slice according to the less functions passed to OrderedBy.
func (ms *multiSorter) Sort(tags []map[string]string) {
	ms.tags = tags
	sort.Sort(ms)
}

// OrderedBy returns a Sorter that sorts using the less functions, in order.
// Call its Sort method to sort the data.
func OrderedBy(less ...lessFunc) *multiSorter {
	return &multiSorter{
		less: less,
	}
}

// Len is part of sort.Interface.
func (ms *multiSorter) Len() int {
	return len(ms.tags)
}

// Swap is part of sort.Interface.
func (ms *multiSorter) Swap(i, j int) {
	ms.tags[i], ms.tags[j] = ms.tags[j], ms.tags[i]
}

// Less is part of sort.Interface. It is implemented by looping along the
// less functions until it finds a comparison that discriminates between
// the two items (one is less than the other). Note that it can call the
// less functions twice per call. We could change the functions to return
// -1, 0, 1 and reduce the number of calls for greater efficiency: an
// exercise for the reader.
func (ms *multiSorter) Less(i, j int) bool {
	p, q := ms.tags[i], ms.tags[j]
	// Try all but the last comparison.
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
		switch {
		case less(p, q):
			// p < q, so we have a decision.
			return true
		case less(q, p):
			// p > q, so we have a decision.
			return false
		}
		// p == q; try the next comparison.
	}
	// All comparisons to here said "equal", so just return whatever
	// the final comparison reports.
	return ms.less[k](p, q)
}

func NamespaceComparator(c1, c2 map[string]string) bool {
	return c1["namespace"] < c2["namespace"]
}

func KeyComparator(c1, c2 map[string]string) bool {
	return c1["key"] < c2["key"]
}

func ValueComparator(c1, c2 map[string]string) bool {
	return c1["value"] < c2["value"]
}
