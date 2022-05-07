package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/zeebo/blake3"
)

var (
	one   = big.NewInt(1)
	two   = big.NewInt(2)
	three = big.NewInt(3)
)

const (
	blocksize = 100000000
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

	// StartingValue is the first number (inclusive) to check.
	StartingValue *big.Int `json:"startingValue,omitempty"`

	// EndingValue is the last number (inclusive) to check.
	EndingValue *big.Int `json:"endingValue,omitempty"`

	// AssignedOn is when this work item was assigned.
	AssignedOn time.Time `json:"assignedOn,omitempty"`

	// Expiry indicates a cutoff time after which
	// this work item may be considered abandoned, and reassigned.
	// This is not exactly a hard cut-off, and if the work is
	// completed after this time, if the evidence is accepted,
	// work will still be considered complete.
	Expiry time.Time `json:"expiry,omitempty"`
}

type UserCredentials struct {
	UserID            string `json:"userId,omitempty"`
	UserSecret        string `json:"userSecret,omitempty"`
	UserSecretVersion string `json:"user_secret_version,omitempty"`
}

type WorkAuthenticator struct {
	UserSecretVersion    string `json:"userSecretVersion,omitempty"`
	AuthenticatorVersion string `json:"authenticatorVersion,omitempty"`
	Authenticator        string `json:"authenticator,omitempty"`
}

// WorkEvidence is used for proving work was performed.  For non-"complete" status updates,
// these should be set to zero.
type WorkEvidence struct {
	TotalIterations uint64 `json:"totalIterations,omitempty"`
	MaxIterations   uint64 `json:"maxIterations,omitempty"`
}

// WorkProgressReport is a message sent to indicate
// completed work, as well as status updates as work is
// performed, and other status changes.
type WorkProgressReport struct {
	Work WorkPacket

	// NodeInfo is the collected node info for where this work
	// was performed.
	NodeInfo NodeInfo `json:"nodeInfo,omitempty"`

	// WorkerID is the specific worker thread which completed this
	// work unit.
	WorkerID int `json:"workerID,omitempty"`

	// Status indicates why we are sending this report.
	//   pending = in our work list, but not yet started.
	//   running = currently running on a worker.
	//   abandoned = we no longer wish to work on this.
	//   completed = we have completed the work requested.
	// While statuses other than "completed" can be sent and will
	// update the user's view of work they have in progress,
	// only "completed" is required to be sent.  Work without
	// any other update will be marked as "pending" in the UI.
	Status string `json:"status,omitempty"`

	// StartedOn is the UTC timestamp of when we began working on this specific work packet.
	StartedOn time.Time `json:"startedOn,omitempty"`

	// CompletedOn is when we completed the work.
	CompletedOn time.Time `json:"completedOn,omitempty"`

	Evidence WorkEvidence `json:"evidence,omitempty"`
}

var (
	work = &WorkPacket{
		ID:         "id-of-packet",
		Nonce:      "nonce-of-packet",
		AssignedOn: time.Now().UTC(),
	}
)

// envidenceHash returns a base64 encoded hash for the evidence provided.
func evidenceHash(user UserCredentials, work WorkPacket, evidence WorkEvidence) WorkAuthenticator {
	h := blake3.New()
	s := fmt.Sprintf("%s:%s:%s:%s:%d:%d",
		work.ID, work.Nonce, work.StartingValue, work.EndingValue,
		evidence.TotalIterations, evidence.MaxIterations)
	h.Write([]byte(s))
	sum := h.Sum(nil)
	authenticator := base64.StdEncoding.EncodeToString(sum)
	return WorkAuthenticator{
		UserSecretVersion:    user.UserSecretVersion,
		AuthenticatorVersion: "v1-blake3",
		Authenticator:        authenticator,
	}
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

	ending := big.NewInt(0)
	ending.Add(ending, starting)
	count := big.NewInt(blocksize)
	ending.Add(ending, count)

	work.StartingValue = starting
	work.EndingValue = ending
	totalInterations, max, found := run(work)
	log.Printf("totalIterations: %d", totalInterations)
	log.Printf("found: %v", found)
	log.Printf("Average iterations per test: %.6f", float64(totalInterations)/float64(blocksize))
	log.Printf("  max %d", max)
}

func run(work *WorkPacket) (uint64, uint64, []*big.Int) {
	startTime := time.Now().UTC().UnixMilli()
	counter := 0
	current := big.NewInt(0)
	current.Add(current, work.StartingValue)
	interetingNumbers := []*big.Int{}
	totalIterations := uint64(0)
	maxIterations := uint64(0)
	for {
		counter++
		if counter == 10000000 {
			now := time.Now().UTC().UnixMilli()
			rate := calcRate(work.StartingValue, current, startTime, now)

			log.Printf("bitlen %d testing %s, totalIterations %d, rate %.5f",
				current.BitLen(), current, totalIterations, rate)
			counter = 0
		}
		interesting, iterCount := iterate(current)
		totalIterations += iterCount
		if maxIterations < iterCount {
			maxIterations = iterCount
		}
		if interesting {
			v := big.NewInt(0)
			v.Add(v, current)
			interetingNumbers = append(interetingNumbers, v)
		}
		shouldEnd := current.Cmp(work.EndingValue)
		if shouldEnd > 0 {
			break
		}
		current.Add(current, two)
	}
	endTime := time.Now().UTC().UnixMilli()
	rate := calcRate(work.StartingValue, work.EndingValue, startTime, endTime)

	log.Printf("Block completed.")
	log.Printf("   Starting: %s", work.StartingValue)
	log.Printf("     Ending: %s", work.EndingValue)
	log.Printf("       last: %s", current)
	log.Printf("       Rate: %.5f", rate)
	log.Printf("Interesting: %v", interetingNumbers)
	return totalIterations, maxIterations, interetingNumbers
}

func calcRate(s *big.Int, c *big.Int, startTime int64, endTime int64) float64 {
	duration := float64(endTime-startTime) / 1000.0
	computed := big.NewInt(0)
	computed.Sub(c, s)
	computedi := computed.Int64()
	return float64(computedi) / duration
}

func iterate(s *big.Int) (interesting bool, iterCount uint64) {
	n := big.NewInt(0)
	n.Add(n, s)
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
			return true, iterCount
		} else if c == -1 {
			return false, iterCount
		}
	}
}
