package cluster

import (
	context "context"
	"log"
	"time"

	raftpb "github.com/mahmedtan/raft/proto"
	"google.golang.org/protobuf/proto"
)

// RequestVote handles incoming vote requests from other nodes in the cluster.
func (n *Node) RequestVote(_ context.Context, request *raftpb.RequestVoteRequest) (*raftpb.RequestVoteResponse, error) {

	hasOlderTerm := request.GetTerm() < n.persistentState.GetCurrentTerm()
	alreadyVotedForAnotherCandidate := request.GetTerm() == n.persistentState.GetCurrentTerm() && n.persistentState.VotedFor != nil && n.persistentState.GetVotedFor() != request.GetCandidateId()

	if hasOlderTerm || alreadyVotedForAnotherCandidate {

		return &raftpb.RequestVoteResponse{
			VoteGranted: proto.Bool(false),
			Term:        n.persistentState.CurrentTerm,
		}, nil
	}

	n.MaybeBecomeFollower(*request.Term)

	isLogOkay := request.GetLastLogTerm() > n.GetLastLogTerm() || (request.GetLastLogTerm() == n.GetLastLogTerm() && request.GetLastLogIndex() >= n.GetLastLogIndex())

	if isLogOkay {
		n.persistentState.VotedFor = request.CandidateId
		n.SavePersistentState()
		n.ResetTimer()

		return &raftpb.RequestVoteResponse{
			VoteGranted: proto.Bool(true),
			Term:        n.persistentState.CurrentTerm,
		}, nil

	} else {
		return &raftpb.RequestVoteResponse{
			VoteGranted: proto.Bool(false),
			Term:        n.persistentState.CurrentTerm,
		}, nil
	}

}

// AppendEntries handles incoming log replication requests from the leader node.
func (n *Node) AppendEntries(_ context.Context, request *raftpb.AppendEntriesRequest) (*raftpb.AppendEntriesResponse, error) {

	log.Printf("Node %v, Term %v: Received AppendEntries from Leader %v for Term %v\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm(), request.GetLeaderId(), request.GetTerm())

	if *request.Term < n.persistentState.GetCurrentTerm() {
		return &raftpb.AppendEntriesResponse{
			Success: proto.Bool(false),
			Term:    n.persistentState.CurrentTerm,
		}, nil
	}

	n.MaybeBecomeFollower(*request.Term)

	n.leaderId = int(request.GetLeaderId())

	n.ResetTimer()

	prevLogIndex := int(request.GetPrevLogIndex())
	prevLogTerm := request.GetPrevLogTerm()

	if prevLogIndex >= 0 {
		if prevLogIndex >= len(n.persistentState.Log) {
			return &raftpb.AppendEntriesResponse{
				Success: proto.Bool(false),
				Term:    n.persistentState.CurrentTerm,
			}, nil
		}
		if n.persistentState.Log[prevLogIndex].GetTerm() != prevLogTerm {
			return &raftpb.AppendEntriesResponse{
				Success: proto.Bool(false),
				Term:    n.persistentState.CurrentTerm,
			}, nil
		}
	}

	entries := request.GetEntries()

	if len(entries) > 0 {
		insertAt := prevLogIndex + 1
		for i, entry := range entries {
			targetIndex := insertAt + i
			if targetIndex < len(n.persistentState.Log) {
				if n.persistentState.Log[targetIndex].GetTerm() != entry.GetTerm() {
					n.persistentState.Log = n.persistentState.Log[:targetIndex]
					n.persistentState.Log = append(n.persistentState.Log, entries[i:]...)
					break
				}
				continue
			} else {
				n.persistentState.Log = append(n.persistentState.Log, entries[i:]...)
				break
			}
		}

		n.SavePersistentState()
	}

	if request.LeaderCommit != nil && int(*request.LeaderCommit) > n.commitIndex {
		leaderCommit := int(*request.LeaderCommit)
		lastIndex := len(n.persistentState.Log) - 1
		n.commitIndex = min(leaderCommit, lastIndex)

		log.Printf("Node %v, Term %v: Updated commitIndex to %d\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm(), n.commitIndex)
	}

	return &raftpb.AppendEntriesResponse{
		Success: proto.Bool(true),
		Term:    n.persistentState.CurrentTerm,
	}, nil
}

// StartElection starts the vote request process for the node to become a leader in the cluster.
func (node *Node) StartElection() {

	log.Printf("Node %v, Term %v: Starting Election\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm()+1)

	node.IncrementTerm()
	node.role = Candidate
	votesRecvd := []int64{node.persistentState.GetNodeId()}
	node.persistentState.VotedFor = node.persistentState.NodeId
	node.SavePersistentState()

	for _, member := range ClusterMembers {

		if member == int(node.persistentState.GetNodeId()) {
			continue
		}

		raftClient, err := GetRaftClientForNodeId(member)

		if err != nil {
			continue
		}

		req := &raftpb.RequestVoteRequest{
			CandidateId:  proto.Int64(node.persistentState.GetNodeId()),
			Term:         node.persistentState.CurrentTerm,
			LastLogTerm:  proto.Int64(node.GetLastLogTerm()),
			LastLogIndex: proto.Int64(node.GetLastLogIndex()),
		}

		reply, err := raftClient.RequestVote(context.Background(), req)

		if err != nil {
			continue
		}

		node.MaybeBecomeFollower(*reply.Term)

		if node.role != Candidate {
			log.Printf("Node %v, Term %v: No longer a candidate, stopping election\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm())
			break
		}

		if node.role == Candidate && *reply.VoteGranted == true && *node.persistentState.CurrentTerm == *reply.Term {

			votesRecvd = append(votesRecvd, int64(member))

			if len(votesRecvd) > (len(ClusterMembers) / 2) {
				log.Printf("Node %v, Term %v: Received majority votes, becoming leader\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm())
				node.role = Leader
				node.leaderId = int(node.persistentState.GetNodeId())
				node.ResetTimer()

				lastIndex := len(node.persistentState.Log) - 1
				for _, m := range ClusterMembers {
					if m == int(node.persistentState.GetNodeId()) {
						node.matchIndex[m] = lastIndex
						node.nextIndex[m] = lastIndex + 1
					} else {
						node.matchIndex[m] = -1
						node.nextIndex[m] = lastIndex + 1
					}
				}

				go node.ReplicateLog()

				go node.StartHeartbeat()

				break

			}
		}

	}

}

func (node *Node) ReplicateLog() {
	log.Printf("Node %v, Term %v: Replicating log entries to followers\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm())

	for _, member := range ClusterMembers {
		go func(m int) {
			if member == int(node.persistentState.GetNodeId()) {
				return
			}

			raftClient, err := GetRaftClientForNodeId(member)

			if err != nil {
				return
			}

			prevLogIndex := node.nextIndex[member] - 1
			var prevLogTerm int64 = 0
			if prevLogIndex >= 0 && prevLogIndex < len(node.persistentState.Log) {
				prevLogTerm = node.persistentState.Log[prevLogIndex].GetTerm()
			}

			req := &raftpb.AppendEntriesRequest{
				LeaderId:     node.persistentState.NodeId,
				Term:         node.persistentState.CurrentTerm,
				Entries:      node.persistentState.Log[node.nextIndex[member]:],
				LeaderCommit: proto.Int64(int64(node.commitIndex)),
				PrevLogIndex: proto.Int64(int64(prevLogIndex)),
				PrevLogTerm:  proto.Int64(prevLogTerm),
			}

			log.Printf("Node %v, Term %v: Sending AppendEntries to Node %v\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm(), member)

			reply, err := raftClient.AppendEntries(context.Background(), req)

			if err != nil {
				return
			}

			node.MaybeBecomeFollower(*reply.Term)

			if node.role != Leader {
				log.Printf("Node %v, Term %v: No longer the leader, stopping log replication\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm())
				return
			}

			if *reply.Success == true && *node.persistentState.CurrentTerm == *reply.Term {
				node.matchIndex[member] = len(node.persistentState.Log) - 1
				node.nextIndex[member] = len(node.persistentState.Log)
				node.CommitLogEntries()

				log.Printf("Node %v, Term %v: Log replication to Node %v succeeded. matchIndex=%d, nextIndex=%d\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm(), member, node.matchIndex[member], node.nextIndex[member])
			} else {
				node.nextIndex[member] -= 1
				if node.nextIndex[member] < 0 {
					node.nextIndex[member] = 0
				}

				log.Printf("Node %v, Term %v: Log replication to Node %v failed. Decrementing nextIndex to %d\n", node.persistentState.GetNodeId(), node.persistentState.GetCurrentTerm(), member, node.nextIndex[member])
			}

		}(member)
	}
}

func (n *Node) StartHeartbeat() {

	n.heartbeatTicker = time.NewTicker(HeartbeatInterval)

	log.Printf("Node %v, Term %v: Starting heartbeat\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm())
	for n.role == Leader {
		<-n.heartbeatTicker.C

		n.ReplicateLog()

	}

	log.Printf("Node %v, Term %v: Stopping heartbeat\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm())

	n.heartbeatTicker.Stop()
}

func (n *Node) CommitLogEntries() {

	minAcks := (len(ClusterMembers) / 2) + 1
	for index := n.commitIndex + 1; index < int(len(n.persistentState.Log)); index++ {
		count := 1 // counting self
		for _, member := range ClusterMembers {
			if member == int(n.persistentState.GetNodeId()) {
				continue
			}

			if n.matchIndex[member] >= int(index) {
				count++
			}
		}
		if count >= minAcks && n.persistentState.Log[index].GetTerm() == n.persistentState.GetCurrentTerm() {
			n.commitIndex = index
			log.Printf("Node %v, Term %v: Committed log entry at index %d\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm(), n.commitIndex)
		}
	}
}

func (n *Node) BroadcastMessage(_ context.Context, request *raftpb.BroadcastMessageRequest) (*raftpb.BroadcastMessageResponse, error) {
	if n.role != Leader {
		return &raftpb.BroadcastMessageResponse{
			Success: proto.Bool(false),
			Message: proto.String("Not the leader"),
		}, nil
	}

	newEntry := &raftpb.LogEntry{
		Term:    proto.Int64(n.persistentState.GetCurrentTerm()),
		Command: request.Message,
	}

	n.persistentState.Log = append(n.persistentState.Log, newEntry)

	n.SavePersistentState()

	log.Printf("Node %v, Term %v: Appended new log entry. Log length is now %d\n", n.persistentState.GetNodeId(), n.persistentState.GetCurrentTerm(), len(n.persistentState.Log))

	n.ReplicateLog()

	return &raftpb.BroadcastMessageResponse{
		Success: proto.Bool(true),
		Message: proto.String("Message broadcasted"),
	}, nil
}
