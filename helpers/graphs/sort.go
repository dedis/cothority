package graphs

import "sort"

type Float64Slice struct {
	A []float64
	I []int // indices
}

func (p Float64Slice) Len() int           { return len(p.A) }
func (p Float64Slice) Less(i, j int) bool { return p.A[i] < p.A[j] || isNaN(p.A[i]) && !isNaN(p.A[j]) }
func (p Float64Slice) Swap(i, j int)      { p.A[i], p.A[j], p.I[i], p.I[j] = p.A[j], p.A[i], p.I[j], p.I[i] }

func (p Float64Slice) Sort() { sort.Sort(p) }

func isNaN(f float64) bool {
	return f != f
}

func MakeFloat64Slice(A []float64) Float64Slice {
	ACopy := make([]float64, len(A))
	I := make([]int, len(A))
	for i := range ACopy {
		I[i] = i
	}
	copy(ACopy, A)
	var fs Float64Slice
	fs.A = ACopy
	fs.I = I
	return fs
}

func sortFloats(A []float64) Float64Slice {
	fs := MakeFloat64Slice(A)
	fs.Sort()
	return fs
}
