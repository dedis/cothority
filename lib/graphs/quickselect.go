package graphs

func kRecSmallest(A []float64, ind []int, k int) int {
	pos := partition(A, ind)
	// if we have selected k elements return this
	if k == pos+1 {
		return pos
	}
	// if k is on the left hand side of the partition
	if k < pos+1 {
		return kRecSmallest(A[:pos], ind[:pos], k)
	}
	// look on the right half of the array
	// subtract the number of elements on the left hand side
	// from k and the element in the center
	return kRecSmallest(A[pos+1:], ind[pos+1:], k-pos-1)
}

// returns an array of indices of the smallest
// it does not alter the input array
func kSmallest(A []float64, k int) []int {

	ind := make([]int, len(A))
	for i := range ind {
		ind[i] = i
	}
	if k >= len(A) {
		return ind
	}
	ACopy := make([]float64, len(A))
	copy(ACopy, A)
	kRecSmallest(ACopy, ind, k)
	return ind[:k]
}

func partition(A []float64, ind []int) int {
	p := A[len(A)-1] // pivot is last element
	i := 0
	for j := range A {
		if A[i] <= p {
			A[i], A[j] = A[j], A[i]
		}
	}
	A[i], A[len(A)-1] = A[len(A)-1], A[i]
	return i
}
