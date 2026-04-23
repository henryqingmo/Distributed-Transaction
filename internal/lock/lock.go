package lock

type Lock struct {
	State   LockState
	Holders map[string]struct{}
}

type LockState int

const (
	READ LockState = iota
	WRITE
	UNLOCKED
)
