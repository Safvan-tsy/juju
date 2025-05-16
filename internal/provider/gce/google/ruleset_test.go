// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package google

import (
	stdtesting "testing"

	"github.com/juju/collections/set"
	"github.com/juju/tc"
	"google.golang.org/api/compute/v1"

	"github.com/juju/juju/core/network"
	corefirewall "github.com/juju/juju/core/network/firewall"
	"github.com/juju/juju/internal/testing"
)

type RuleSetSuite struct {
	testing.BaseSuite
}

func TestRuleSetSuite(t *stdtesting.T) { tc.Run(t, &RuleSetSuite{}) }
func makeRuleSet() ruleSet {
	return newRuleSetFromRules(corefirewall.IngressRules{
		corefirewall.NewIngressRule(network.MustParsePortRange("8000-8099/tcp")),
		corefirewall.NewIngressRule(network.MustParsePortRange("80/tcp")),
		corefirewall.NewIngressRule(network.MustParsePortRange("79-81/tcp")),
		corefirewall.NewIngressRule(network.MustParsePortRange("5123-8099/udp"), "192.168.1.0/24"),
		corefirewall.NewIngressRule(network.MustParsePortRange("icmp")),
	})
}

func (s *RuleSetSuite) TestNewRuleSetFromRules(c *tc.C) {
	rs := makeRuleSet()
	c.Assert(rs, tc.DeepEquals, ruleSet{
		"b42e18366a": &firewall{
			SourceCIDRs: []string{"0.0.0.0/0"},
			AllowedPorts: protocolPorts{
				"tcp":  []network.PortRange{{8000, 8099, "tcp"}, {80, 80, "tcp"}, {79, 81, "tcp"}},
				"icmp": []network.PortRange{{-1, -1, "icmp"}},
			},
		},
		"d01a825c13": &firewall{
			SourceCIDRs: []string{"192.168.1.0/24"},
			AllowedPorts: protocolPorts{
				"udp": []network.PortRange{{5123, 8099, "udp"}},
			},
		},
	})
}

func newFirewall(name, target string, sourceRanges []string, ports map[string][]string) *compute.Firewall {
	allowed := make([]*compute.FirewallAllowed, len(ports))
	i := 0
	for protocol, ranges := range ports {
		allowed[i] = &compute.FirewallAllowed{
			IPProtocol: protocol,
			Ports:      ranges,
		}
		i++
	}
	return &compute.Firewall{
		Name:         name,
		TargetTags:   []string{target},
		SourceRanges: sourceRanges,
		Allowed:      allowed,
	}
}

func (s *RuleSetSuite) TestNewRuleSetFromFirewalls(c *tc.C) {
	ports := map[string][]string{
		"tcp": {"80", "443", "17070-17073"},
		"udp": {"123"},
	}
	fw1 := newFirewall("weeps", "target", []string{"1.2.3.0/24", "2.3.4.0/24"}, ports)
	fw2 := newFirewall("blackbird", "somewhere", nil, ports)
	ruleset, err := newRuleSetFromFirewalls(fw1, fw2)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(ruleset, tc.DeepEquals, ruleSet{
		"b42e18366a": &firewall{
			Name:        "blackbird",
			Target:      "somewhere",
			SourceCIDRs: []string{"0.0.0.0/0"},
			AllowedPorts: protocolPorts{
				"tcp": []network.PortRange{{80, 80, "tcp"}, {443, 443, "tcp"}, {17070, 17073, "tcp"}},
				"udp": []network.PortRange{{123, 123, "udp"}},
			},
		},
		"0e3c16a771": &firewall{
			Name:        "weeps",
			Target:      "target",
			SourceCIDRs: []string{"1.2.3.0/24", "2.3.4.0/24"},
			AllowedPorts: protocolPorts{
				"tcp": []network.PortRange{{80, 80, "tcp"}, {443, 443, "tcp"}, {17070, 17073, "tcp"}},
				"udp": []network.PortRange{{123, 123, "udp"}},
			},
		},
	})
}

func (s *RuleSetSuite) TestProtocolPortsUnion(c *tc.C) {
	p1 := protocolPorts{"tcp": []network.PortRange{{8000, 8099, "tcp"}, {80, 80, "tcp"}, {79, 81, "tcp"}}}
	p2 := protocolPorts{
		"tcp": []network.PortRange{{80, 80, "tcp"}, {80, 100, "tcp"}, {443, 443, "tcp"}},
		"udp": []network.PortRange{{67, 67, "udp"}},
	}
	result := p1.union(p2)
	c.Assert(result, tc.DeepEquals, protocolPorts{
		"tcp": []network.PortRange{{8000, 8099, "tcp"}, {80, 80, "tcp"}, {79, 81, "tcp"}, {80, 100, "tcp"}, {443, 443, "tcp"}},
		"udp": []network.PortRange{{67, 67, "udp"}},
	})
}

func (s *RuleSetSuite) TestProtocolPortsRemove(c *tc.C) {
	p1 := protocolPorts{"tcp": []network.PortRange{{8000, 8099, "tcp"}, {80, 80, "tcp"}, {79, 81, "tcp"}}}
	p2 := protocolPorts{
		"tcp": []network.PortRange{{80, 100, "tcp"}, {443, 443, "tcp"}, {80, 80, "tcp"}},
	}
	result := p1.remove(p2)
	c.Assert(result, tc.DeepEquals, protocolPorts{
		"tcp": []network.PortRange{{8000, 8099, "tcp"}, {79, 81, "tcp"}},
	})
}

func (s *RuleSetSuite) TestRuleSetToIngressRules(c *tc.C) {
	ports := map[string][]string{
		"tcp": {"80", "443", "17070-17073"},
		"udp": {"123"},
	}
	fw1 := newFirewall("weeps", "target", []string{"1.2.3.0/24", "2.3.4.0/24"}, ports)
	fw2 := newFirewall("blackbird", "somewhere", nil, ports)
	ruleset, err := newRuleSetFromFirewalls(fw1, fw2)
	c.Assert(err, tc.ErrorIsNil)
	rules, err := ruleset.toIngressRules()
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(rules, tc.DeepEquals, corefirewall.IngressRules{
		corefirewall.NewIngressRule(network.MustParsePortRange("80/tcp"), corefirewall.AllNetworksIPV4CIDR),
		corefirewall.NewIngressRule(network.MustParsePortRange("80/tcp"), "1.2.3.0/24", "2.3.4.0/24"),
		corefirewall.NewIngressRule(network.MustParsePortRange("443/tcp"), corefirewall.AllNetworksIPV4CIDR),
		corefirewall.NewIngressRule(network.MustParsePortRange("443/tcp"), "1.2.3.0/24", "2.3.4.0/24"),
		corefirewall.NewIngressRule(network.MustParsePortRange("17070-17073/tcp"), corefirewall.AllNetworksIPV4CIDR),
		corefirewall.NewIngressRule(network.MustParsePortRange("17070-17073/tcp"), "1.2.3.0/24", "2.3.4.0/24"),
		corefirewall.NewIngressRule(network.MustParsePortRange("123/udp"), corefirewall.AllNetworksIPV4CIDR),
		corefirewall.NewIngressRule(network.MustParsePortRange("123/udp"), "1.2.3.0/24", "2.3.4.0/24"),
	})
}

func (s *RuleSetSuite) TestMatchPorts(c *tc.C) {
	ruleset := makeRuleSet()
	fw, ok := ruleset.matchProtocolPorts(protocolPorts{
		"udp": {{5123, 8099, "udp"}},
	})
	c.Assert(ok, tc.IsTrue)
	c.Assert(fw, tc.DeepEquals, &firewall{
		AllowedPorts: protocolPorts{
			"udp": {{5123, 8099, "udp"}},
		},
		SourceCIDRs: []string{"192.168.1.0/24"},
	})
	// No partial matches.
	fw, ok = ruleset.matchProtocolPorts(protocolPorts{
		"tcp": {{80, 80, "tcp"}},
	})
	c.Assert(ok, tc.IsFalse)
	c.Assert(fw, tc.IsNil)
}

func (s *RuleSetSuite) TestMatchSourceCIDRs(c *tc.C) {
	ruleset := makeRuleSet()
	c.Logf("%#v", ruleset)
	c.Logf("%s", sourcecidrs([]string{"0.0.0.0/0"}).key())
	fw, ok := ruleset.matchSourceCIDRs([]string{"0.0.0.0/0"})
	c.Assert(ok, tc.IsTrue)
	c.Assert(fw, tc.DeepEquals, &firewall{
		SourceCIDRs: []string{"0.0.0.0/0"},
		AllowedPorts: protocolPorts{
			"tcp":  []network.PortRange{{8000, 8099, "tcp"}, {80, 80, "tcp"}, {79, 81, "tcp"}},
			"icmp": []network.PortRange{{-1, -1, "icmp"}},
		},
	})
	fw, ok = ruleset.matchSourceCIDRs([]string{"1.2.3.0/24"})
	c.Assert(ok, tc.IsFalse)
	c.Assert(fw, tc.IsNil)
}

func (s *RuleSetSuite) TestAllNames(c *tc.C) {
	ports := map[string][]string{"tcp": {"80"}}
	fw1 := newFirewall("weeps", "target", []string{"1.2.3.0/24", "2.3.4.0/24"}, ports)
	fw2 := newFirewall("blackbird", "somewhere", nil, ports)
	ruleset, err := newRuleSetFromFirewalls(fw1, fw2)
	c.Assert(err, tc.ErrorIsNil)
	c.Assert(ruleset.allNames(), tc.DeepEquals, set.NewStrings("weeps", "blackbird"))
}

func (s *RuleSetSuite) TestDifferentOrdersForCIDRs(c *tc.C) {
	ruleset := NewRuleSetFromRules(corefirewall.IngressRules{
		corefirewall.NewIngressRule(network.MustParsePortRange("8000-8099/tcp"), "1.2.3.0/24", "4.3.2.0/24"),
		corefirewall.NewIngressRule(network.MustParsePortRange("80/tcp"), "4.3.2.0/24", "1.2.3.0/24"),
	})
	// They should be combined into one firewall rule.
	c.Assert(ruleset, tc.HasLen, 1)
	for _, fw := range ruleset {
		c.Assert(fw, tc.DeepEquals, &firewall{
			SourceCIDRs: []string{"1.2.3.0/24", "4.3.2.0/24"},
			AllowedPorts: protocolPorts{
				"tcp": []network.PortRange{
					network.MustParsePortRange("8000-8099/tcp"),
					network.MustParsePortRange("80/tcp"),
				},
			},
		})
	}
}
