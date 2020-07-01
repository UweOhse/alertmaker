package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	GPLv2 = "https://www.ohse.de/uwe/licenses/GPL-2"
)

var (
	flagVersion = flag.Bool("version", false, "show version information and exit.")
	flagLicense = flag.Bool("license", false, "show license information and exit.")
	flagConfig  = flag.String("config", "config.yml", "the path to the configuration file to use.")
)

type testconfig struct {
	Name        string
	For         string
	Expr        string
	Summary     string
	Description string
	Source      string
	Notice      string
	Warning     string
	Critical    string
	Annotations map[string]string
	Labels      map[string]string
	Selector    string
}

type classconfig struct {
	Name         string
	Inherits     []string
	ServiceLevel string
	Annotations  map[string]string
	Labels       map[string]string
	Tests        []testconfig
}
type hostconfig struct {
	Name         string
	Classes      []string
	ServiceLevel string
	Instances    map[string]string
	Annotations  map[string]string
	Labels       map[string]string
	Tests        []testconfig
}
type defaultconfig struct {
	For          string
	ServiceLevel string
	Labels       map[string]string
}

type config struct {
	Defaults defaultconfig
	Load     []string
	Tests    []testconfig
	Classes  []classconfig
	Hosts    []hostconfig
}

var cfg config

func findTest(name string) *testconfig {
	for _, d := range cfg.Tests {
		if d.Name == name {
			return &d
		}
	}
	return nil
}
func findTestX(name string) *testconfig {
	t := findTest(name)
	if t == nil {
		log.Fatalf("cannot find referenced test: %s\n", name)
	}
	return t
}

func findClass(name string) *classconfig {
	for _, d := range cfg.Classes {
		if d.Name == name {
			return &d
		}
	}
	return nil
}
func findClassX(name string) *classconfig {
	t := findClass(name)
	if t == nil {
		for idx, d := range cfg.Classes {
			log.Printf("cl #%d = %s\n", idx, d.Name)
		}
		log.Fatalf("cannot find referenced class: %s\n", name)
	}
	return t
}

func fillTest(to, from *testconfig) *testconfig {
	if to.For == "" {
		to.For = from.For
	}
	if to.Expr == "" {
		to.Expr = from.Expr
	}
	if to.Summary == "" {
		to.Summary = from.Summary
	}
	if to.Description == "" {
		to.Description = from.Description
	}
	if to.Source == "" {
		to.Source = from.Source
	}
	if to.Selector == "" {
		to.Selector = from.Selector
	}
	if to.Notice == "" {
		to.Notice = from.Notice
	}
	if to.Warning == "" {
		to.Warning = from.Warning
	}
	if to.Critical == "" {
		to.Critical = from.Critical
	}
	for k, v := range from.Annotations {
		if _, ok := to.Annotations[k]; ok {
			continue
		}
		if to.Annotations == nil {
			to.Annotations = make(map[string]string)
		}
		to.Annotations[k] = v
	}
	for k, v := range from.Labels {
		if _, ok := to.Labels[k]; ok {
			continue
		}
		if to.Labels == nil {
			to.Labels = make(map[string]string)
		}
		to.Labels[k] = v
	}
	return to
}
func fillOneHostFromOneClass(h *hostconfig, cd *classconfig) *hostconfig {

	if h.ServiceLevel == "" {
		h.ServiceLevel = cd.ServiceLevel
	}
	for k, v := range cd.Annotations {
		if _, ok := h.Annotations[k]; ok {
			continue
		}
		if h.Annotations == nil {
			h.Annotations = make(map[string]string)
		}
		h.Annotations[k] = v
	}
	for k, v := range cd.Labels {
		if _, ok := h.Labels[k]; ok {
			continue
		}
		if h.Labels == nil {
			h.Labels = make(map[string]string)
		}
		h.Labels[k] = v
	}

	// include class tests into hosts
	for _, t := range cd.Tests {
		td := findTestX(t.Name)
		found := -1
		for idx, t2 := range h.Tests {
			if t2.Name == t.Name {
				found = idx
			}
		}
		if found == -1 {
			// simple append class test to host.
			h.Tests = append(h.Tests, t)
		} else {
			h.Tests[found] = *fillTest(&h.Tests[found], td)
		}
	}
	// include test defaults into this hosts tests
	for idx, t := range h.Tests {
		td := findTestX(t.Name)
		h.Tests[idx] = *fillTest(&h.Tests[idx], td)
	}

	return h
}

func fillOneHost(h *hostconfig) *hostconfig {
	for _, cn := range h.Classes {
		cd := findClassX(cn)
		fillOneHostFromOneClass(h, cd)

	}
	return h
}
func fillHosts() {
	for idx, d := range cfg.Hosts {
		cfg.Hosts[idx] = *fillOneHost(&d)
	}
}
func fillTests() {
	for idx, d := range cfg.Tests {
		if d.For == "" {
			d.For = cfg.Defaults.For
		}
		if d.For == "" {
			d.For = "5m"
		}
		if d.Expr == "" {
			log.Fatalf("bad expr in test %s", d.Name)
		}
		if d.Source == "" {
			log.Fatalf("bad source in test %s", d.Name)
		}
		cfg.Tests[idx] = d
	}
}

func fillOneClass(c *classconfig) *classconfig {

	if c.ServiceLevel == "" {
		c.ServiceLevel = cfg.Defaults.ServiceLevel
	}

	for idx, t := range c.Tests {
		td := findTestX(t.Name)
		c.Tests[idx] = *fillTest(&c.Tests[idx], td)
	}
	return c
}
func fillClasses() {
	for idx, d := range cfg.Classes {
		cfg.Classes[idx] = *fillOneClass(&d)
	}
}

func inheritOneClass(to, from *classconfig) *classconfig {
	if to.ServiceLevel == "" {
		to.ServiceLevel = from.ServiceLevel
	}
	for k, v := range from.Annotations {
		if _, ok := to.Annotations[k]; ok {
			continue
		}
		to.Annotations[k] = v
	}
	for k, v := range from.Labels {
		if _, ok := to.Labels[k]; ok {
			continue
		}
		to.Labels[k] = v
	}
	for _, ft := range from.Tests {
		found := false
		for _, tt := range to.Tests {
			if tt.Name == ft.Name {
				found = true
			}
		}
		if !found {
			to.Tests = append(to.Tests, ft)
		}
	}
	return to
}
func inheritClasses() {
	for idx, d := range cfg.Classes {
		for _, iname := range d.Inherits {
			cd := findClassX(iname)
			log.Printf("%s is %#v\n", d.Name, d)
			log.Printf("  inherits %#v\n", cd)
			cfg.Classes[idx] = *inheritOneClass(&d, cd)
			log.Printf("  is now %#v\n", cfg.Classes[idx])
		}
	}
}
func findSourceInstance(h *hostconfig, s, sel string) string {
	val, ok := h.Instances[s]
	if !ok {
		log.Fatalf("source instance for %s undefined for hosts %s", s, h.Name)
	}
	str := "instance='" + val + "'"
	if sel > "" {
		str += "," + sel
	}
	return str

}
func outputOneRuleLevel(h *hostconfig, t *testconfig, lv string) {
	var thres string
	switch lv {
	case "notice":
		if t.Notice > "" {
			thres = t.Notice
		}
	case "warning":
		if t.Warning > "" {
			thres = t.Warning
		}
	case "critical":
		if t.Critical > "" {
			thres = t.Critical
		}
	default:
		log.Fatalf("unknown level %s", lv)
	}
	if thres == "" {
		return
	}
	fmt.Printf("    - alert: %s\n", t.Name)
	identity := findSourceInstance(h, t.Source, t.Selector)
	s := strings.ReplaceAll(t.Expr, "{}", "{"+identity+"}")
	s = strings.ReplaceAll(s, "@SELECTOR", identity)
	fmt.Printf("      expr: %s\n", strconv.Quote("("+s+") "+thres))
	fmt.Printf("      for: %s\n", strconv.Quote(t.For))
	fmt.Printf("      labels:\n")
	fmt.Printf("        servicelevel: %s\n", strconv.Quote(h.ServiceLevel))
	fmt.Printf("        host: %s\n", strconv.Quote(h.Name))
	fmt.Printf("        test: %s\n", strconv.Quote(t.Name))
	for k, v := range h.Labels {
		fmt.Printf("        %s: %s\n", k, strconv.Quote(v))
	}
	for k, v := range t.Labels {
		if _, ok := h.Labels[k]; ok {
			continue
		}
		fmt.Printf("        %s: %s\n", k, strconv.Quote(v))
	}
	for k, v := range cfg.Defaults.Labels {
		if _, ok := t.Labels[k]; ok {
			continue
		}
		if _, ok := h.Labels[k]; ok {
			continue
		}
		fmt.Printf("        %s: %s\n", k, strconv.Quote(v))
	}
	fmt.Printf("      annotations:\n")
	fmt.Printf("        summary: %s\n",
		strconv.Quote(lv+":"+t.Summary+" (instance {{$labels.instance}})"))
	fmt.Printf("        description: %s\n",
		strconv.Quote(t.Description+"\\n  Value: {{$value}}\\n  Labels: {{$labels}}"))
	for k, v := range t.Annotations {
		fmt.Printf("        %s: %s\n", k, strconv.Quote(v))
	}
	for k, v := range h.Annotations {
		if _, ok := t.Annotations[k]; ok {
			continue
		}
		fmt.Printf("        %s: %s\n", k, strconv.Quote(v))
	}
}
func outputOne(h *hostconfig, t *testconfig) {
	/*
	groups:
	- name: x7.cfg
	  rules:
	# ./x7 node_load1: vars=a:7:{s:7:"summary";s:9:"high load";s:7:"warning";s:5:">= 10";s:5:"class";s:8:"CPUUsage";s:8:"severity";s:4:"mail";s:3:"for";s:2:"5m";s:8:"pve_host";s:2:"x7";s:8:"instance";s:15:"x7.ohse.de:9100";}
	# ./x7 node_load1: query=s:33:"node_load1{instance='$instance$'}";
	    - alert: Warning_node_load1
	      expr: ((node_load1{instance='x7.ohse.de:9100'}) >= 10)
	      for: 5m
	      labels:
	        host: 'x7'
	        service: 'node_load1'
	        severity: mail
	        class: 'CPUUsage'
	      annotations:
	        summary: 'high load'

	*/
	outputOneRuleLevel(h, t, "notice")
	outputOneRuleLevel(h, t, "warning")
	outputOneRuleLevel(h, t, "critical")

}

func outputIt() {
	fmt.Printf("groups:\n")
	fmt.Printf("- name: all\n")
	fmt.Printf("  rules:\n")
	for _, h := range cfg.Hosts {
		for _, t := range h.Tests {
			outputOne(&h, &t)
		}
	}
}

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Printf("%s: version %s\n", os.Args[0], versionString)
		os.Exit(0)
	}
	if *flagLicense {
		fmt.Printf("%s: version %s\n\nThis software is published under the terms of the GPL version 2.\nA copy is at %s.\n",
			os.Args[0], versionString, GPLv2)
		os.Exit(0)
	}

	input, err := ioutil.ReadFile(*flagConfig)
	if err != nil {
		log.Fatal(err)
	}
	err = yaml.Unmarshal(input, &cfg)
	if err != nil {
		log.Fatalf("%s: %v", *flagConfig, err)
	}
	if len(cfg.Load) > 0 {
		for _, str := range cfg.Load {
			t2 := config{}
			// gosec, i intend to load files mentioned in a config file.
			input, err = ioutil.ReadFile(str) // #nosec G304
			if err != nil {
				log.Fatal(err)
			}
			err = yaml.Unmarshal(input, &t2)
			if err != nil {
				log.Fatalf("%s: %v", str, err)
			}
			cfg.Tests = append(cfg.Tests, t2.Tests...)
			cfg.Classes = append(cfg.Classes, t2.Classes...)
			cfg.Hosts = append(cfg.Hosts, t2.Hosts...)
		}
	}

	fillTests()
	fillClasses()
	inheritClasses()
	fillHosts()

	d, err := json.MarshalIndent(&cfg, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	if false {
		fmt.Printf("--- final dump:\n%s\n", string(d))
	}

	outputIt()
}
