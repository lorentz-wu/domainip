package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	probing "github.com/prometheus-community/pro-bing"
)

const (
	HOSTS_TEMPLATE = `
# DOMAINTOIP Host Start
%s

# Update time: %s
# DOMAINTOIP Host End\n
`
)

var (
	githubURLs = []string{
		"alive.github.com", "api.github.com", "assets-cdn.github.com",
		"avatars.githubusercontent.com", "avatars0.githubusercontent.com",
		"avatars1.githubusercontent.com", "avatars2.githubusercontent.com",
		"avatars3.githubusercontent.com", "avatars4.githubusercontent.com",
		"avatars5.githubusercontent.com", "camo.githubusercontent.com",
		"central.github.com", "cloud.githubusercontent.com", "codeload.github.com",
		"collector.github.com", "desktop.githubusercontent.com",
		"favicons.githubusercontent.com", "gist.github.com",
		"github-cloud.s3.amazonaws.com", "github-com.s3.amazonaws.com",
		"github-production-release-asset-2e65be.s3.amazonaws.com",
		"github-production-repository-file-5c1aeb.s3.amazonaws.com",
		"github-production-user-asset-6210df.s3.amazonaws.com", "github.blog",
		"github.com", "github.community", "github.githubassets.com",
		"github.global.ssl.fastly.net", "github.io", "github.map.fastly.net",
		"githubstatus.com", "live.github.com", "media.githubusercontent.com",
		"objects.githubusercontent.com", "pipelines.actions.githubusercontent.com",
		"raw.githubusercontent.com", "user-images.githubusercontent.com",
		"vscode.dev", "education.github.com",
	}
)

func getPage(addr string) ([]byte, error) {
	res, err := http.Get(addr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("status error")
	}
	return ioutil.ReadAll(res.Body)
}

func DomainToIP(domain string) ([]string, error) {
	site := "https://www.ipaddress.com/site/"

	res, err := http.Get(site + domain)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("status error")
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return []string{}, err
	}

	ip := []string{}

	e := doc.Find("ul.separated2").First()
	if e != nil {
		e.Find("li").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			pattern := `\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`
			regex := regexp.MustCompile(pattern)
			result := regex.FindAllString(text, -1)
			ip = append(ip, result...)
		})
	}

	return ip, nil
}

var pinger *probing.Pinger

func Ping(addr string) (int64, error) {

	pinger, err := probing.NewPinger("addr")
	if err != nil {
		return 0, err
	}

	pinger.SetPrivileged(true)
	pinger.Count = 3
	pinger.Interval = time.Duration(1 * time.Second)
	err = pinger.Run()
	if err != nil {
		return 0, err
	}
	stats := pinger.Statistics()
	pinger.Stop()
	//fmt.Println(stats)
	return int64(stats.MinRtt), nil
}

func pingIP(ips []string) string {
	var minDelay int64 = math.MaxInt64
	minDelayIP := ips[0]
	for _, ip := range ips {
		delay, err := Ping(ip)
		if err != nil {
			continue
		}
		if delay < minDelay {
			minDelay = delay
			minDelayIP = ip
		}
	}
	return minDelayIP
}

func lastIP(ips []string) string {
	return ips[len(ips)-1]
}

func randomSelect(ips []string) string {
	return ips[rand.Intn(len(ips))]
}

func generateHosts(hosts []string, bestIP func(ips []string) string) string {
	var hostnamesToIPs = make(map[string]string)
	var mutex sync.Mutex

	// Create a channel to receive domain names
	domains := make(chan string)

	// Start N worker goroutines to process domains
	const numWorkers = 32
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()

			for domain := range domains {
				ips, err := DomainToIP(domain)
				if err != nil || len(ips) == 0 {
					continue
				}

				minDelayIP := bestIP(ips)

				mutex.Lock()
				hostnamesToIPs[domain] = minDelayIP
				mutex.Unlock()
			}
		}()
	}

	// Send domains to the channel
	for _, domain := range hosts {
		domains <- domain
	}
	close(domains)

	wg.Wait()

	var content strings.Builder
	for hostname, ip := range hostnamesToIPs {
		content.WriteString(ip + strings.Repeat(" ", 20-len(ip)) + hostname + "\n")
	}

	return fmt.Sprintf(HOSTS_TEMPLATE, content.String(), time.Now())
}

func main() {
	//fmt.Println("domain to ip start")
	//DomainToIP("assets-cdn.github.com")
	r := generateHosts(githubURLs, pingIP)
	fmt.Println(r)
}
