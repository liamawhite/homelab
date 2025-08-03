package main

import (
	_ "github.com/k8s-gateway/k8s_gateway"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
)

// directives defines the order of plugin execution
// Plugins are processed in the order they appear in this slice
var directives = []string{
	"errors",
	"log",
	"debug",
	"health",
	"ready",
	"metrics",
	"prometheus",
	"reload",
	"loadbalance",
	"bind",
	"hosts",
	"file",
	"loop",

	// Actual processing plugins
	"cache",
	"k8s_gateway", // external
	"forward",
}

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}
