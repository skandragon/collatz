package main

import (
	"encoding/json"
	"log"
	"math/big"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
)

var (
	one   = big.NewInt(1)
	two   = big.NewInt(2)
	three = big.NewInt(3)
)

func cpuinfo() {
	cpus, err := cpu.Info()
	if err != nil {
		log.Fatal(err)
	}
	j, err := json.Marshal(cpus)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("CPU Info: %s", string(j))
}

func showHostname() {
	info, err := host.Info()
	if err != nil {
		log.Fatal(err)
	}

	j, err := json.Marshal(info)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Host Info: %s", string(j))
}

func main() {
	cpuinfo()
	showHostname()
	starting := big.NewInt(0)
	starting.SetBit(starting, 67, 1)
	starting.SetBit(starting, 0, 1) // make odd
	counter := 1
	for {
		counter++
		if counter == 1000000 {
			log.Printf("bitlen %d testing %s", starting.BitLen(), starting)
			counter = 1
		}
		iterate(starting)
		starting.Add(starting, two)
	}

}

func iterate(s *big.Int) (iterCount int, n *big.Int) {
	n = big.NewInt(0)
	n.Add(n, s)
	for {
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
			return
		}
	}
}
