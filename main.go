package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func removeKeyword(input []string, keyword string) []string {
	result := []string{}
	for _, v := range input {
		if !strings.Contains(v, keyword) {
			result = append(result, v)
		}
	}
	return result
}

func removeDuplicate(args []string) []string {
	results := make([]string, 0, len(args))
	encountered := map[string]bool{}
	for i := 0; i < len(args); i++ {
		if !encountered[args[i]] {
			encountered[args[i]] = true
			results = append(results, args[i])
		}
	}
	return results
}

func GetPeers(hostname string, c chan []string) {
	url := fmt.Sprintf("https://%s/api/v1/instance/peers", hostname)
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 3 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		// fmt.Println(err)
		c <- []string{}
		wg.Done()
		return
	}
	defer resp.Body.Close()

	// statuscode
	if resp.StatusCode != 200 {
		// fmt.Println("not 200")
		c <- []string{}
		wg.Done()
		return
	}

	byteArray, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		c <- []string{}
		wg.Done()
		return
	}

	// length too short
	if len(byteArray) <= 1 {
		c <- []string{}
		wg.Done()
		return
	}

	rawStr := strings.TrimSpace(strings.Replace((string(byteArray[1 : len(byteArray)-1])), "\"", "", -1))
	// contain HTML code in responce
	if strings.Contains(rawStr, "<") {
		c <- []string{}
		wg.Done()
		return
	}

	// contain space in responce
	if strings.Contains(rawStr, " ") {
		fmt.Println(url)
		fmt.Println(rawStr)
		c <- []string{}
		wg.Done()
		return
	}

	peerSlice := strings.Split(rawStr, ",")
	peerSlice = removeKeyword(peerSlice, "activitypub-troll")

	c <- peerSlice
	wg.Done()
}

var wg sync.WaitGroup

func main() {
	startPoint := "mastodon.motcha.tech"

	// depth1: go GetPeers(startPoint)
	ch := make(chan []string)
	wg.Add(1)
	go GetPeers(startPoint, ch)
	PeerDepthOne := <-ch

	// depth2: for _,peer := range PeerDepth1 { go GetPeers(peer) }
	PeerDepthTwo := []string{""}
	count := len(PeerDepthOne)
	// count := 30
	wg.Add(count)

	// goroutine run
	for i, peer := range PeerDepthOne {
		if i > count {
			break
		}
		//fmt.Printf("goroutine: %d/%d\n", i+1, count)
		go GetPeers(peer, ch)
	}

	// receive result
	for i := 0; i < count; i++ {
		newps := <-ch
		//fmt.Printf("receive_goroutine: %d/%d\n", i+1, count)
		if len(newps) > 10 {
			PeerDepthTwo = append(PeerDepthTwo, newps...)
			// fmt.Println(len(PeerDepthTwo))
		}
	}
	wg.Wait()

	// merge Depth 1, 2, sort it
	Peer := append(PeerDepthOne, PeerDepthTwo...)
	Peer = removeDuplicate(Peer)
	sort.Strings(Peer)

	f, _ := os.Create("Peers.list")
	f.Write([]byte(strings.Join(Peer, "\n")))
	f.Close()
}
