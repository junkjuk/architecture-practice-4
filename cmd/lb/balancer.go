package main

import (
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/roman-mazur/design-practice-2-template/httptools"
	"github.com/roman-mazur/design-practice-2-template/signal"
)

var (
	port       = flag.Int("port", 8090, "load balancer port")
	timeoutSec = flag.Int("timeout-sec", 3, "request timeout time in seconds")
	https      = flag.Bool("https", false, "whether backends support HTTPs")

	traceEnabled = flag.Bool("trace", false, "whether to include tracing information into responses")
)

var (
	timeout     = time.Duration(*timeoutSec) * time.Second
	serversPool = []string{
		"127.0.0.1:8080",
		"127.0.0.1:8081",
		"127.0.0.1:8082",
	}

	aliveServersPool = []string{
		"127.0.0.1:8080",
		"127.0.0.1:8081",
		"127.0.0.1:8082",
	}
)

func scheme() string {
	if *https {
		return "https"
	}
	return "http"
}

func health(dst string) bool {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	req, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s://%s/health", scheme(), dst), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}

func hash(address string) int {
	hash := fnv.New32()
	hash.Write([]byte(address))
	hashed := int(hash.Sum32())
	return hashed
}

func getServer(address string, mutex *sync.Mutex) string {
	mutex.Lock()
	clientHash := hash(address) % len(aliveServersPool)
	serverAddr := aliveServersPool[clientHash]
	mutex.Unlock()
	return serverAddr
}

func forward(dst string, rw http.ResponseWriter, r *http.Request) error {
	ctx, _ := context.WithTimeout(r.Context(), timeout)
	fwdRequest := r.Clone(ctx)
	fwdRequest.RequestURI = ""
	fwdRequest.URL.Host = dst
	fwdRequest.URL.Scheme = scheme()
	fwdRequest.Host = dst

	resp, err := http.DefaultClient.Do(fwdRequest)
	if err == nil {
		for k, values := range resp.Header {
			for _, value := range values {
				rw.Header().Add(k, value)
			}
		}
		if *traceEnabled {
			rw.Header().Set("lb-from", dst)
		}
		log.Println("fwd", resp.StatusCode, resp.Request.URL)
		rw.WriteHeader(resp.StatusCode)
		defer resp.Body.Close()
		_, err := io.Copy(rw, resp.Body)
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
		return nil
	} else {
		log.Printf("Failed to get response from %s: %s", dst, err)
		rw.WriteHeader(http.StatusServiceUnavailable)
		return err
	}
}

func checkServers(servers []string, aliveServers []string, mutex *sync.Mutex) {
	for index, server := range servers {
		server := server
		index := index
		go func() {
			for range time.Tick(10 * time.Second) {
				mutex.Lock()
				isHealth := health(server)
				log.Println(server, isHealth)
				if isHealth {
					aliveServers[index] = server
				} else {
					aliveServers[index] = ""
				}
				mutex.Unlock()
			}
		}()
	}
}

func main() {
	flag.Parse()
	var mutex sync.Mutex

	checkServers(serversPool, aliveServersPool, &mutex)
	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		server := getServer(r.RemoteAddr, &mutex)
		forward(server, rw, r)
	}))

	log.Println("Starting load balancer...")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}
