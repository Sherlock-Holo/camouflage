package client

type baseHeap []*base

func (h baseHeap) Len() int {
	return len(h)
}

func (h baseHeap) Less(i, j int) bool {
	return h[i].count < h[j].count
}

func (h baseHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index, h[j].index = i, j
}

func (h *baseHeap) Push(x interface{}) {
	st := x.(*base)
	st.index = len(*h)
	*h = append(*h, st)
}

func (h *baseHeap) Pop() interface{} {
	old := *h

	status := old[len(old)-1]
	status.index = -1

	*h = old[:len(old)-1]

	return status
}
