package main

type NodeState int

const (
	Leader NodeState = iota
	Follower
	Candidate
)

func (s NodeState) String() string {
	switch s {
	case Leader:
		return "Leader"
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	default:
		return "Unknown"
	}
}

type Node struct {
	id    int
	state NodeState

	leaderId   int
	votesRecvd []int
}
