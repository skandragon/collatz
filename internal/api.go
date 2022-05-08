/*
 * Copyright 2022 Michael Graff.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package internal

import (
	"encoding/base64"
	"fmt"
	"math/big"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/zeebo/blake3"
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

// UserCredentials hold the userid, secret, and secret version we will use
// to authenticate.  The UserSecret is the only part that is strictly
// confidential.  Using a UserSecretVersion allows rotation of the secret
// if one is compromised, while maintaining some amount of trust in
// what was already submitted.
type UserCredentials struct {
	UserID            string `json:"userID,omitempty"`
	UserSecretVersion string `json:"userSecretVersion,omitempty"`
	UserSecret        string `json:"userSecret,omitempty"`
}

// WorkAuthenticator is a signature on the work we performed.
type WorkAuthenticator struct {
	AuthenticatorVersion string `json:"authenticatorVersion,omitempty"`
	UserSecretVersion    string `json:"userSecretVersion,omitempty"`
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
	Work WorkPacket `json:"work,omitempty"`

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

	Evidence      WorkEvidence      `json:"evidence,omitempty"`
	Authenticator WorkAuthenticator `json:"authenticator,omitempty"`
}

// envidenceHash returns a base64 encoded hash for the evidence provided.
func evidenceHash(user UserCredentials, work WorkPacket, evidence WorkEvidence) WorkAuthenticator {
	h := blake3.New()
	s := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%d:%d",
		work.ID, work.Nonce, work.StartingValue, work.EndingValue,
		user.UserID, user.UserSecretVersion, user.UserSecret,
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

// CPUInfo returns the data about this specific node, to be used in reports as-is.
func CPUInfo(workers int) (*NodeInfo, error) {
	cpus, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	return &NodeInfo{HostInfo: *hostInfo, CPUInfo: cpus, Workers: workers}, nil
}
