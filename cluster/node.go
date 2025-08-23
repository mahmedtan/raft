package cluster

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	raftpb "github.com/mahmedtan/raft/proto"
	"google.golang.org/grpc"
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
	role     NodeRole
	leaderId int

	timer           *time.Timer
	heartbeatTicker *time.Ticker

	commitLen       int
	nextIndex       []int
	matchIndex      []int
	persistentState *raftpb.PersistentState

	raftpb.UnimplementedRaftServer
}

func (n *Node) IncrementTerm() {
	*n.persistentState.CurrentTerm = n.persistentState.GetCurrentTerm() + 1

}

func (n *Node) MaybeBecomeFollower(term int64) {
	if term > n.persistentState.GetCurrentTerm() {
		log.Printf("Node %d: Term out of date. CurrentTerm=%d, NewTerm=%d. Becoming Follower.",
			n.persistentState.GetNodeId(),
			n.persistentState.GetCurrentTerm(),
			term)
		*n.persistentState.CurrentTerm = term
		n.persistentState.VotedFor = nil
		n.role = Follower
		n.SavePersistentState()
		n.ResetTimer()
	}

}

func (n *Node) GetLastLogIndex() int64 {

	logEntries := n.persistentState.GetLog()

	if len(logEntries) == 0 {
		return 0
	}

	return int64(len(n.persistentState.GetLog()) - 1)
}

func (n *Node) GetLastLogTerm() int64 {
	logEntries := n.persistentState.GetLog()
	if len(logEntries) == 0 {
		return 0
	}

	return logEntries[len(logEntries)-1].GetTerm()
}

func StartNode(id int) {
	node := &Node{
		role:     Follower,
		leaderId: 0,
		timer:    time.NewTimer(getRandomTimeout()),

		commitLen:  0,
		nextIndex:  make([]int, len(ClusterMembers)+1),
		matchIndex: make([]int, len(ClusterMembers)+1),

		persistentState: &raftpb.PersistentState{
			NodeId:      proto.Int64(int64(id)),
			CurrentTerm: proto.Int64(0),
			VotedFor:    nil,
			Log:         []*raftpb.LogEntry{},
		},
	}

	grpcServer := grpc.NewServer()

	raftpb.RegisterRaftServer(grpcServer, node)

	err := node.LoadPersistentState()

	if err != nil {
		logError(err)
		return
	}

	log.Printf("Node %d: Loaded state: Term=%d, VotedFor=%v, LogEntries=%d",
		id,
		node.persistentState.GetCurrentTerm(),
		node.persistentState.GetVotedFor(),
		len(node.persistentState.GetLog()))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", getPortForNodeId(id)))
	if err != nil {
		logError(err)
	}

	go func() {
		for {
			<-node.timer.C

			if node.role == Leader {
				node.ResetTimer()
				continue
			}

			node.StartElection()

			node.ResetTimer()
		}
	}()

	grpcServer.Serve(lis)
}

func (n *Node) ResetTimer() {
	if n.timer != nil {
		n.timer.Stop()
	}

	log.Printf("Node %v: Timer Reset", n.persistentState.GetNodeId())
	n.timer = time.NewTimer(getRandomTimeout())
}

func (n *Node) SavePersistentState() error {
	fileName := getPersistentStateFileName(int(n.persistentState.GetNodeId()))
	data, err := proto.Marshal(n.persistentState)
	if err != nil {
		logError(err)
		return err
	}

	return os.WriteFile(fileName, data, 0644)
}

func (n *Node) LoadPersistentState() error {

	fileName := getPersistentStateFileName(int(n.persistentState.GetNodeId()))
	data, err := os.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, save the initial state that was set in NewNode
			return n.SavePersistentState()
		} else {
			logError(err)
		}
	} else {
		// File exists, unmarshal the data
		err = proto.Unmarshal(data, n.persistentState)
		if err != nil {
			logError(err)
		}
	}

	return nil
}

const BasePort = 5000
