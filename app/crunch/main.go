package main

import (
	"log"
	"math/big"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
)

var (
	one   = big.NewInt(1)
	two   = big.NewInt(2)
	three = big.NewInt(3)
)

// NodeInfo holds some somewhat arbitrary info about a worker node.
type NodeInfo struct {
	HostInfo host.InfoStat  `json:"hostInfo,omitempty"`
	CPUInfo  []cpu.InfoStat `json:"cpuInfo,omitempty"`
	Workers  int            `json:"workers,omitempty"`
}

// WorkPacket is a message from the server to incidate a work
// item.
type WorkPacket struct {
	// ID is the work packet ID, assigned by the server.
	ID string `json:"id,omitempty"`

	// Nonce is used as a work authenticator.
	Nonce string `json:"nonce,omitempty"`

	// AssignedOn is when this work item was assigned.
	AssignedOn time.Time `json:"assignedOn,omitempty"`

	// Expiry indicates a cutoff time after which
	// this work item may be considered abandoned, and reassigned.
	// This is not exactly a hard cut-off, and if the work is
	// completed after this time, if the evidence is accepted,
	// work will still be considered complete.
	Expiry time.Time `json:"expiry,omitempty"`

	// StartingValue is the first number (inclusive) to check.
	StartingValue *big.Int `json:"startingValue,omitempty"`

	// EndingValue is the last number (inclusive) to check.
	EndingValue *big.Int `json:"endingValue,omitempty"`
}

// WorkProgressReport is a message sent to indicate
// completed work, as well as status updates as work is
// performed, and other status changes.
type WorkProgressReport struct {
	// ID is the work packet ID, assigned by the server.
	ID string `json:"id,omitempty"`

	// Nonce is used as a work authenticator.
	Nonce string `json:"nonce,omitempty"`

	// StartingValue is the first number (inclusive) to check.
	StartingValue *big.Int `json:"startingValue,omitempty"`

	// EndingValue is the last number (inclusive) to check.
	EndingValue *big.Int `json:"endingValue,omitempty"`

	// NodeInfo is the collected node info for where this work
	// was performed.
	NodeInfo NodeInfo `json:"nodeInfo,omitempty"`

	// WorkerID is the specific worker thread which completed this
	// work unit.
	WorkerID int `json:"workerID,omitempty"`

	// Evidence is a base64(sha512("Work.ID:Work.Nonce:StartingValue:EndingValue:$iterHash"))
	// where $iterHash is a hash of the string representation of each iteration count, with a single \n
	// between them.
	// If the work is still in progress, this should be empty.
	Evidence string `json:"evidence,omitempty"`

	// Authenticator is a signed Evidence hash, which is
	//   auth = sha512(UserID:UserSecret:UserSecretVersion:Evidence)
	// base64(sha512(SecretVersion + ":" + base64(auth)))
	// If the work is still in progress, the hash is of an evidence
	// hash where $iterString is set to "in-progress"
	Authenticator string

	// Status indicates why we are sending this report.
	//   PENDING = in our work list, but not yet started.
	//   RUNNING = currently running on a worker.
	//   ABANDONED = we no longer wish to work on this.
	//   COMPLETED = we have completed the work requested.
	// While statuses other than "COMPLETED" can be sent and will
	// update the user's view of work they have in progress,
	// only "COMPLETED" is required to be sent.  Work without
	// any other update will be marked as "PENDING" in the UI.
	Status string

	// StartedOn is the UTC timestamp of when we began working on this specific work packet.
	StartedOn time.Time `json:"startedOn,omitempty"`

	// CompletedOn is when we completed the work.
	CompletedOn time.Time `json:"completedOn,omitempty"`
}

func cpuinfo() (*NodeInfo, error) {
	cpus, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	return &NodeInfo{HostInfo: *hostInfo, CPUInfo: cpus}, nil
}

func main() {
	ni, err := cpuinfo()
	if err != nil {
		log.Fatalf("cannot get node or cpu info: %v", err)
	}
	log.Printf("Node Info: %#v", ni)

	starting := big.NewInt(0)
	starting.SetBit(starting, 67, 1)
	starting.SetBit(starting, 0, 1) // make odd
	counter := 1
	for {
		counter++
		if counter == 10000000 {
			log.Printf("bitlen %d testing %s", starting.BitLen(), starting)
			counter = 1
		}
		iterate(starting)
		starting.Add(starting, two)
	}

}

func iterate(s *big.Int) int {
	n := big.NewInt(0)
	n.Add(n, s)
	iterCount := 0
	for {
		iterCount++
		if n.Bit(0) == 0 {
			n.Rsh(n, 1)
		} else {
			n.Mul(n, three)
			n.Add(n, one)
		}
		c := n.Cmp(s)
		if c == 0 {
			log.Printf("Found a loop back to starting value: %s", n)
		} else if c == -1 {
			return iterCount
		}
	}
}
