package server

import (
	"github.com/benmills/quiz"
	"log"
	"net/http/httptest"
	"testing"

	"library/httpclient"
)

type NullWriter int
func (NullWriter) Write([]byte) (int, error) { return 0, nil }

type TestNode struct {
	*httptest.Server
	node *Server
}

func testServer() *TestNode {
	nullLogger := log.New(new(NullWriter), "", 0)
	libraryServer := New("localhost:someport", nullLogger)
	httpServer := httptest.NewServer(libraryServer.Handler())
	libraryServer.SetURL(httpServer.URL)

	return &TestNode{httpServer, libraryServer}
}

func TestAddAKey(t *testing.T) {
	test := quiz.Test(t)

	server := testServer()
	defer server.Close()

	statusCode, body := httpclient.Put(server.URL+"/data/mykey", "bar")

	test.Expect(statusCode).ToEqual(201)
	test.Expect(body).ToEqual("bar")
}

func TestStatsKeys(t *testing.T) {
	test := quiz.Test(t)

	server := testServer()
	defer server.Close()

	httpclient.Put(server.URL+"/data/mykey", "bar")
	statusCode, body := httpclient.Get(server.URL+"/stats/keys", "")

	test.Expect(statusCode).ToEqual(200)
	test.Expect(body).ToContain(`"count":1`)
	test.Expect(body).ToContain(`"data":{"mykey":"bar"}`)
}

func TestFetchKey(t *testing.T) {
	test := quiz.Test(t)

	server := testServer()
	defer server.Close()

	httpclient.Put(server.URL+"/data/mykey", "bar")
	statusCode, body := httpclient.Get(server.URL+"/data/mykey", "bar")

	test.Expect(statusCode).ToEqual(200)
	test.Expect(body).ToEqual("bar")
}

func TestFetchUnknownKey(t *testing.T) {
	test := quiz.Test(t)

	server := testServer()
	defer server.Close()

	statusCode, _ := httpclient.Get(server.URL+"/data/mykey", "bar")

	test.Expect(statusCode).ToEqual(404)
}

func TestUpdateKey(t *testing.T) {
	test := quiz.Test(t)

	server := testServer()
	defer server.Close()

	httpclient.Put(server.URL+"/data/mykey", "bar")
	httpclient.Put(server.URL+"/data/mykey", "baz")
	statusCode, body := httpclient.Get(server.URL+"/data/mykey", "")

	test.Expect(statusCode).ToEqual(200)
	test.Expect(body).ToEqual("baz")
}

func TestFetchesAcrossNodes(t *testing.T) {
	test := quiz.Test(t)

	serverA := testServer()
	defer serverA.Close()
	serverB := testServer()
	defer serverB.Close()

	httpclient.Put(serverA.URL+"/peers/join", serverB.URL)

	// "a"'s hash will be stored on serverB
	key := "a"

	var statusCode int
	var body string

	statusCode, _ = httpclient.Put(serverA.URL+"/data/"+key, "bar")
	test.Expect(statusCode).ToEqual(201)

	statusCode, body = httpclient.Get(serverB.URL+"/data/"+key, "")
	test.Expect(statusCode).ToEqual(200)
	test.Expect(body).ToEqual("bar")

	statusCode, body = httpclient.Get(serverA.URL+"/data/"+key, "")
	test.Expect(statusCode).ToEqual(200)
	test.Expect(body).ToEqual("bar")
}

func TestReplicationOnPut(t *testing.T) {
	test := quiz.Test(t)

	serverA := testServer()
	defer serverA.Close()
	serverB := testServer()
	defer serverB.Close()

	httpclient.Put(serverA.URL+"/peers/join", serverB.URL)
	httpclient.Put(serverA.URL+"/settings/n", "1")

	// "a"'s hash will be stored on serverB
	key := "a"

	httpclient.Put(serverA.URL+"/data/"+key, "bar")

	_, aHasKey := serverA.node.values["a"]
	_, bHasKey := serverB.node.values["a"]

	test.Expect(aHasKey).ToBeTrue()
	test.Expect(bHasKey).ToBeTrue()
}

func TestMoveKeys(t *testing.T) {
	test := quiz.Test(t)

	serverA := testServer()
	defer serverA.Close()
	serverB := testServer()
	defer serverB.Close()
	serverC := testServer()
	defer serverC.Close()

	httpclient.Put(serverA.URL+"/peers/join", serverB.URL)

	// this key will first be on server B then will be on server C
	key := "b"

	httpclient.Put(serverA.URL+"/data/"+key, "foo")
	httpclient.Put(serverA.URL+"/peers/join", serverC.URL)

	_, bHasKey := serverB.node.values[key]
	_, cHasKey := serverC.node.values[key]

	test.Expect(bHasKey).ToBeFalse()
	test.Expect(cHasKey).ToBeTrue()
}

func TestResizeCleansUpReplicas(t *testing.T) {
	test := quiz.Test(t)

	serverA := testServer()
	defer serverA.Close()
	serverB := testServer()
	defer serverB.Close()
	serverC := testServer()
	defer serverC.Close()
	serverD := testServer()
	defer serverD.Close()

	httpclient.Put(serverA.URL+"/peers/join", serverB.URL)
	httpclient.Put(serverA.URL+"/peers/join", serverC.URL)
	httpclient.Put(serverA.URL+"/settings/n", "1")

	// this key will be owned by c before and after d joins
	// however it's replicas will first by a then d
	key := "a"

	httpclient.Put(serverA.URL+"/data/"+key, "foo")
	httpclient.Put(serverA.URL+"/peers/join", serverD.URL)

	_, aHasKey := serverA.node.values[key]
	_, cHasKey := serverC.node.values[key]
	_, dHasKey := serverD.node.values[key]

	test.Expect(aHasKey).ToBeFalse()
	test.Expect(cHasKey).ToBeTrue()
	test.Expect(dHasKey).ToBeTrue()
}

func TestFailedWriteUpdatesPeerList(t *testing.T) {
	test := quiz.Test(t)

	serverA := testServer()
	defer serverA.Close()
	serverB := testServer()
	peerURL :=  serverB.URL

	key := "a"

	httpclient.Put(serverA.URL+"/peers/join", serverB.URL)
	serverB.Close()
	httpclient.Put(serverA.URL+"/data/"+key, "foo")

	_, body := httpclient.Get(serverA.URL+"/peers", "")
	test.Expect(body).ToContain("dead:"+peerURL)
}
