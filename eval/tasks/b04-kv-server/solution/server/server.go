package server

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"sync"

	"example.com/kv/proto"
	"example.com/kv/store"
)

const maxLine = 64 * 1024

func Serve(ctx context.Context, l net.Listener, s *store.Store) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	conns := map[net.Conn]bool{}
	go func() {
		<-ctx.Done()
		l.Close()
		mu.Lock()
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
	}()
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				wg.Wait()
				return nil
			}
			continue
		}
		mu.Lock()
		if ctx.Err() != nil {
			mu.Unlock()
			c.Close()
			continue
		}
		conns[c] = true
		mu.Unlock()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { mu.Lock(); delete(conns, c); mu.Unlock(); c.Close() }()
			handle(c, s)
		}()
	}
}

func handle(c net.Conn, s *store.Store) {
	r := bufio.NewReaderSize(c, 4096)
	w := bufio.NewWriter(c)
	for {
		var line []byte
		tooLong := false
		for {
			chunk, isPrefix, err := r.ReadLine()
			if err != nil {
				return
			}
			if len(line)+len(chunk) > maxLine {
				tooLong = true
			}
			if !tooLong {
				line = append(line, chunk...)
			}
			if !isPrefix {
				break
			}
		}
		if tooLong {
			w.WriteString("-ERR line too long\n")
			w.Flush()
			return
		}
		ln := strings.TrimSuffix(string(line), "\r")
		if strings.EqualFold(strings.TrimSpace(ln), "quit") {
			w.WriteString("+BYE\n")
			w.Flush()
			return
		}
		w.WriteString(proto.Exec(s, ln))
		if err := w.Flush(); err != nil {
			return
		}
	}
}
