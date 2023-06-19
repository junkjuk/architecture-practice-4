package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func (s *TestSuite) TestBalancer(t *testing.T) {
	var mutex sync.Mutex
	address1 := getServer("172.168.110.1:80", &mutex)
	address2 := getServer("192.168.100.10:8081", &mutex)
	address3 := getServer("127.0.0.1:8080", &mutex)

	assert.Equal(t, "127.0.0.1:8080", address1)
	assert.Equal(t, "127.0.0.1:8081", address2)
	assert.Equal(t, "127.0.0.1:8082", address3)
}

func (s *TestSuite) TestHealth(t *testing.T) {
	result := make([]string, len(s.serversPool))
	var mutex sync.Mutex

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	parsedURL1, _ := url.Parse(server1.URL)
	hostURL1 := parsedURL1.Host

	parsedURL2, _ := url.Parse(server2.URL)
	hostURL2 := parsedURL2.Host

	servers := []string{
		hostURL1,
		hostURL2,
		"server3:8080",
	}

	checkServers(servers, result, &mutex)
	time.Sleep(12 * time.Second)

	assert.Equal(t, hostURL1, result[0])
	assert.Equal(t, hostURL2, result[1])
	assert.Equal(t, "", result[2])
}

func Test(t *testing.T) {
	suite := new(TestSuite)
	suite.SetupSuite()
	t.Run("TestBalancer", suite.TestBalancer)
	t.Run("TestHealth", suite.TestHealth)
}

type TestSuite struct {
	serversPool []string
}

func (s *TestSuite) SetupSuite() {
	s.serversPool = []string{
		"server1:8080",
		"server2:80",
		"server3:80",
	}
}
