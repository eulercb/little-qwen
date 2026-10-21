package server

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/kv/store"
)

func zzhStart(t *testing.T) (string, context.CancelFunc, chan error) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, l, store.New(nil)) }()
	return l.Addr().String(), cancel, done
}

func zzhDial(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	c.SetDeadline(time.Now().Add(10 * time.Second))
	return c, bufio.NewReader(c)
}

func TestHiddenPipelineAndQuit(t *testing.T) {
	addr, cancel, done := zzhStart(t)
	defer cancel()
	c, r := zzhDial(t, addr)
	defer c.Close()
	c.Write([]byte("SET a 1\r\nINCR a\nGET a\nQUIT\nPING\n"))
	var got []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		got = append(got, line)
	}
	if strings.Join(got, "") != "+OK\n:2\n$2\n+BYE\n" {
		t.Fatalf("%q", got)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}

func TestHiddenConcurrentClientsSharedStore(t *testing.T) {
	addr, cancel, _ := zzhStart(t)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, r := zzhDial(t, addr)
			defer c.Close()
			for j := 0; j < 50; j++ {
				c.Write([]byte("INCR n\n"))
				if _, err := r.ReadString('\n'); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	c, r := zzhDial(t, addr)
	defer c.Close()
	c.Write([]byte("GET n\n"))
	if l, _ := r.ReadString('\n'); l != "$500\n" {
		t.Fatalf("%q", l)
	}
}

func TestHiddenLineTooLong(t *testing.T) {
	addr, cancel, _ := zzhStart(t)
	defer cancel()
	c, r := zzhDial(t, addr)
	defer c.Close()
	go c.Write([]byte("SET k " + strings.Repeat("x", 70*1024) + "\n"))
	l, _ := r.ReadString('\n')
	if l != "-ERR line too long\n" {
		t.Fatalf("%q", l)
	}
	if _, err := r.ReadString('\n'); err == nil {
		t.Fatal("connection should be closed")
	}
}

func TestHiddenCancelClosesIdleConnections(t *testing.T) {
	addr, cancel, done := zzhStart(t)
	c, r := zzhDial(t, addr)
	defer c.Close()
	c.Write([]byte("PING\n"))
	r.ReadString('\n')
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve blocked on idle connection")
	}
	if _, err := r.ReadString('\n'); err == nil {
		t.Fatal("idle connection not closed")
	}
}
