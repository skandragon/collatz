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

package main

import (
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/skandragon/collatz/internal"
)

var (
	one       = big.NewInt(1)
	two       = big.NewInt(2)
	three     = big.NewInt(3)
	blocksize = big.NewInt(blocksizeInt)
)

const (
	blocksizeInt = 100000000
)

func main() {
	ni, err := internal.CPUInfo()
	if err != nil {
		log.Fatalf("cannot get node or cpu info: %v", err)
	}
	workers := ni.CPUInfo.Count
	ni.Workers = workers
	log.Printf("Node Info: %#v", ni)

	initial := big.NewInt(0)
	initial.SetBit(initial, 40, 1)
	initial.SetBit(initial, 0, 1) // make odd

	var wg sync.WaitGroup

	for workerID := 0; workerID < workers; workerID++ {
		wg.Add(1)
		starting := big.NewInt(0)
		starting.Add(starting, initial)

		initial.Add(initial, blocksize)

		ending := big.NewInt(0)
		ending.Add(ending, starting)
		ending.Add(ending, blocksize)

		ntests := big.NewInt(0)
		ntests.Sub(ending, starting)
		ntestsInt := ntests.Int64()

		work := &internal.WorkPacket{
			ID:            "id-of-packet",
			Nonce:         "nonce-of-packet",
			AssignedOn:    time.Now().UTC(),
			StartingValue: starting,
			EndingValue:   ending,
		}
		go func(workerID int) {
			defer wg.Done()
			totalInterations, max, found := run(work, workerID)
			log.Printf("%04d: totalIterations: %d", workerID, totalInterations)
			log.Printf("%04d: found: %v", workerID, found)
			log.Printf("%04d: Average iterations per test: %.6f",
				workerID, float64(totalInterations)/float64(ntestsInt))
			log.Printf("%04d:   max %d", workerID, max)
		}(workerID)
	}
	wg.Wait()
}

func run(work *internal.WorkPacket, workerID int) (uint64, uint64, []*big.Int) {
	startTime := time.Now().UTC().UnixMilli()
	counter := 0
	current := big.NewInt(0)
	current.Add(current, work.StartingValue)
	interestingNumbers := []*big.Int{}
	totalIterations := uint64(0)
	maxIterations := uint64(0)
	for {
		counter++
		if counter == 10000000 {
			now := time.Now().UTC().UnixMilli()
			rate := calcRate(work.StartingValue, current, startTime, now)

			log.Printf("%04d: bitlen %d testing %s, totalIterations %d, rate %.5f",
				workerID, current.BitLen(), current, totalIterations, rate)
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
			interestingNumbers = append(interestingNumbers, v)
		}
		shouldEnd := current.Cmp(work.EndingValue)
		if shouldEnd >= 0 {
			break
		}
		current.Add(current, two)
	}
	endTime := time.Now().UTC().UnixMilli()
	rate := calcRate(work.StartingValue, work.EndingValue, startTime, endTime)

	log.Printf("%04d: Block completed.", workerID)
	log.Printf("%04d:    Starting: %s", workerID, work.StartingValue)
	log.Printf("%04d:      Ending: %s", workerID, work.EndingValue)
	log.Printf("%04d:        last: %s", workerID, current)
	log.Printf("%04d:        Rate: %.5f", workerID, rate)
	log.Printf("%04d: Interesting: %v", workerID, interestingNumbers)
	return totalIterations, maxIterations, interestingNumbers
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
