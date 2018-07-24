package client

type managerHeap []*managerStatus

func (h *managerHeap) Len() int {
	return len(*h)
}

func (h *managerHeap) Less(i, j int) bool {
	return (*h)[i].count < (*h)[j].count
}

func (h *managerHeap) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
	(*h)[i].index, (*h)[j].index = i, j
}

func (h *managerHeap) Push(x interface{}) {
	st := x.(*managerStatus)
	st.index = len(*h)
	*h = append(*h, st)
}

func (h *managerHeap) Pop() interface{} {
	old := *h

	status := old[len(old)-1]
	status.index = -1

	*h = old[:len(old)-1]

	return status
}
