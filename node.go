package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"google.golang.org/protobuf/proto"
)

type NodeRole int

const (
	Leader NodeRole = iota
	Follower
	Candidate
)

func (s NodeRole) String() string {
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
	id              int
	role            NodeRole
	timer           *time.Timer
	leaderId        int
	votesRecvd      []int
	persistantState *PersistentState
	UnimplementedRaftServer
}

func NewNode(id int) *Node {
	node := &Node{
		id:         id,
		role:       Follower,
		leaderId:   0,
		votesRecvd: make([]int, 0),
		timer:      time.NewTimer(getRandomTimeout()),

		persistantState: &PersistentState{
			NodeId:     proto.Int64(int64(id)),
			Term:       proto.Int64(0),
			LogEntries: make([]*LogEntry, 0),
			VotedFor:   nil,
		},
	}

	err := node.LoadState()
	if err != nil {
		fmt.Printf("Error loading state for node %d: %v\n", id, err)
	}

	return node
}

func (n *Node) ResetTimer() {
	if n.timer != nil {
		n.timer.Stop()
	}

	log.Printf("Node %v: Timer Reset", n.id)
	n.timer = time.NewTimer(getRandomTimeout())
}

func (n *Node) SaveState() error {
	fileName := getPersistantStateFileName(n.id)
	data, err := proto.Marshal(n.persistantState)
	if err != nil {
		return err
	}

	return os.WriteFile(fileName, data, 0644)
}

func (n *Node) LoadState() error {

	fileName := getPersistantStateFileName(n.id)
	data, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {

			n.SaveState()
		} else {
			return fmt.Errorf("error reading state file: %v", err)
		}
	}

	err = proto.Unmarshal(data, n.persistantState)
	if err != nil {
		return fmt.Errorf("error unmarshalling persistent state: %v", err)
	}

	return nil
}

func getPersistantStateFileName(id int) string {
	return fmt.Sprintf("state/persistant_state_%d.pb", id)
}

func getRandomTimeout() time.Duration {
	return time.Duration(rand.Intn(100)+1000) * time.Millisecond
}
