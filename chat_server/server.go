package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"
)

type Conn struct {
	connId uint32
	conn   net.Conn
}

type Conns map[uint32]*Conn // connId -> Conn

type ConnPool struct {
	lock       sync.RWMutex
	cnt        uint32
	user2Conns [2]Conns // uid -> Conns
}

var pool ConnPool

type Message struct {
	uid     uint16
	errCode int
	msg     string
}

const (
	OK_STATUS = iota
	ERR_READ
	ERR_PARSE_UID
	ERR_PARSE_SIZE
	ERR_READ_UID
	ERR_UID_RANGE
	ERR_PEER_NOT_LOGIN
)

func main() {
	ln, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println("listen err: ", err)
		os.Exit(1)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept err: ", err)
			continue
		}

		pool.lock.Lock()
		connId := pool.cnt
		pool.cnt++
		pool.lock.Unlock()

		var pConn = &Conn{
			connId: connId,
			conn:   conn,
		}

		c := make(chan *Message)
		go handleRecv(pConn, c)
		go handleSend(pConn, c)
	}
}

func handleRecv(conn *Conn, c chan *Message) {
	buf := make([]byte, 1024)
	var size uint16
	var uid uint16
	var msg Message

	r := bytes.NewBuffer(buf)

	// receive uid from client
	if _, err := conn.conn.Read(buf); err != nil {
		msg.errCode = ERR_READ_UID
		goto EndOfRecv
	}

	// parse uid, use big-endian
	if err := binary.Read(r, binary.BigEndian, &uid); err != nil {
		msg.errCode = ERR_PARSE_UID
		goto EndOfRecv
	}

	if uid > 1 {
		msg.errCode = ERR_UID_RANGE
		goto EndOfRecv
	}

	fmt.Println("uid = ", uid)
	msg.uid = uid
	pool.lock.Lock()
	// new user, create it
	if _, ok := pool.user2Conns[uid][conn.connId]; !ok {
		pool.user2Conns[uid] = make(Conns)
		pool.user2Conns[uid][conn.connId] = conn
	}
	pool.lock.Unlock()

	// receive msg
	for {

		if _, err := conn.conn.Read(buf); err != nil {
			msg.errCode = ERR_READ
			goto EndOfRecv
		}

		// parse msg length, use big-endian
		r = bytes.NewBuffer(buf)
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			msg.errCode = ERR_PARSE_SIZE
			goto EndOfRecv
		}

		fmt.Printf("receive %d bytes from client\n", size)
		msg.errCode = OK_STATUS
		msg.msg = string(buf[2:size])
		c <- &msg
	}

EndOfRecv:
	c <- &msg
}

func handleSend(conn *Conn, c chan *Message) {
	var conns Conns
	for {
		msg := <-c
		if msg.errCode != OK_STATUS {
			goto EndOfSend
		}

		// check peer whether is online or not
		pool.lock.RLock()
		conns = pool.user2Conns[1-msg.uid]
		if len(conns) == 0 {
			pool.lock.RUnlock()
			msg.errCode = ERR_PEER_NOT_LOGIN
			goto EndOfSend
		}
		// send msg
		for _, cnn := range conns {
			if _, err := cnn.conn.Write([]byte(msg.msg)); err != nil {
				fmt.Println("write peer client err: ", err)
			}
		}
		fmt.Println("send msg to client: ", msg.msg)
		pool.lock.Unlock()
		continue

	EndOfSend:
		errMsg := fmt.Sprintf("has something wrong! errCode is %d", msg.errCode)
		conn.conn.Write([]byte(errMsg))
		// if peer do not login, the connect should hold on
		if msg.errCode != ERR_PEER_NOT_LOGIN {
			pool.lock.Lock()
			// delete conn in conns
			if _, ok := pool.user2Conns[msg.uid][conn.connId]; ok {
				delete(pool.user2Conns[msg.uid], conn.connId)
			}
			pool.lock.Unlock()
			conn.conn.Close()
			break
		}
	}
}
