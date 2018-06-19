package sabakan

import (
	"net"
	"reflect"
	"testing"
)

var (
	testIPAMConfig = &IPAMConfig{
		MaxNodesInRack:  28,
		NodeIPv4Pool:    "10.69.0.0/20",
		NodeRangeSize:   6,
		NodeRangeMask:   26,
		NodeIndexOffset: 3,
		NodeIPPerNode:   3,
		BMCIPv4Pool:     "10.72.17.0/20",
		BMCRangeSize:    5,
		BMCRangeMask:    20,
	}
)

func testGenerateIP(t *testing.T) {
	t.Parallel()

	cases := []struct {
		machine       *Machine
		nodeAddresses []string
		bmcAddress    string
	}{
		{
			NewMachine(MachineSpec{
				Serial:      "1234",
				Rack:        1,
				IndexInRack: 3,
			}),
			[]string{
				"10.69.0.195",
				"10.69.1.3",
				"10.69.1.67",
			},
			"10.72.17.35",
		},
		{
			NewMachine(MachineSpec{
				Serial:      "5678",
				Rack:        0,
				IndexInRack: 5,
			}),
			[]string{
				"10.69.0.5",
				"10.69.0.69",
				"10.69.0.133",
			},
			"10.72.17.5",
		},
	}

	for _, c := range cases {
		testIPAMConfig.GenerateIP(c.machine)
		spec := c.machine.Spec

		if len(spec.IPv4) != int(testIPAMConfig.NodeIPPerNode) {
			t.Fatal("wrong number of networks")
		}
		if !reflect.DeepEqual(c.nodeAddresses, spec.IPv4) {
			t.Error("wrong IP addresses: ", spec.IPv4)
		}
		if spec.BMC.IPv4 != c.bmcAddress {
			t.Errorf("wrong IP Address: %v", spec.BMC.IPv4)
		}
	}
}

func testLeaseRange(t *testing.T) {
	t.Parallel()

	r := testIPAMConfig.LeaseRange(net.ParseIP("10.68.10.20"))
	if r != nil {
		t.Error("lease range for 10.68.10.20 should be nil")
	}

	r = testIPAMConfig.LeaseRange(net.ParseIP("10.69.10.20"))
	if r == nil {
		t.Fatal("lease range for 10.69.10.20 must not be nil")
	}

	if r.BeginAddress.String() != "10.69.10.32" {
		t.Error(`r.BeginAddress.String() != "10.69.10.32:"`, r.BeginAddress.String())
	}
	if r.Count != 31 {
		t.Error(`r.Count != 31:`, r.Count)
	}
	if r.IP(3).String() != "10.69.10.35" {
		t.Error(`r.IP(3).String() != "10.69.10.35"`, r.IP(3).String())
	}
	if r.Key() != "10.69.10.32" {
		t.Error(`r.Key() != "10.69.10.32:"`, r.Key())
	}
}

func TestIPAM(t *testing.T) {
	t.Run("GenerateIP", testGenerateIP)
	t.Run("LeaseRange", testLeaseRange)
}
