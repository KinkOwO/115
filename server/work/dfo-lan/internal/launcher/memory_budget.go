package launcher

// Server and client descendants have separate budgets. These are commit
// limits, not working-set trimming (which would just move pressure to paging).
const (
	serverMemoryBudget uint64 = 4 << 30
	clientMemoryBudget uint64 = 16 << 30
)
