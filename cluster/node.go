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
	role NodeRole

	votesRecvd []int

	leaderId int
	timer    *time.Timer

	commitLen int

	sentLen []int
	ackLen  []int

	persistentState *raftpb.PersistentState
	raftpb.UnimplementedRaftServer
}

func StartNode(id int) {
	node := &Node{
		role:       Follower,
		leaderId:   0,
		votesRecvd: make([]int, 0),
		timer:      time.NewTimer(getRandomTimeout()),

		commitLen: 0,
		sentLen:   make([]int, len(ClusterMembers)+1),
		ackLen:    make([]int, len(ClusterMembers)+1),

		persistentState: &raftpb.PersistentState{
			NodeId:     proto.Int64(int64(id)),
			Term:       proto.Int64(0),
			VotedFor:   nil,
			LogEntries: make([]*raftpb.LogEntry, 0),
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
		node.persistentState.GetTerm(),
		node.persistentState.GetVotedFor(),
		len(node.persistentState.GetLogEntries()))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", getPortForNodeId(id)))
	if err != nil {
		logError(err)
	}

	go func() {
		for {
			<-node.timer.C

			node.StartVoteRequest()

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
