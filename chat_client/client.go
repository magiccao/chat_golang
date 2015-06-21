package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	conn, err := net.Dial("tcp", ":8000")
	if err != nil {
		fmt.Println("dial to server err: ", err)
		os.Exit(1)
	}

	wg.Add(1)
	go handleSend(conn, &wg)
	go handleRecv(conn, &wg)

	wg.Wait()
	conn.Close()
}

func handleSend(conn net.Conn, wg *sync.WaitGroup) {
	var buf []byte
	var uid uint16
	var msg string

	fmt.Printf("please input your uid: ")
	fmt.Scanf("%d", &uid)

	r := bufio.NewReader(os.Stdin)
	w := new(bytes.Buffer)
	// login
	if err := binary.Write(w, binary.BigEndian, uid); err != nil {
		fmt.Println("binary.Write udi err: ", err)
		goto EndOfSend
	}

	if _, err := conn.Write(w.Bytes()); err != nil {
		fmt.Println("register uid err: ", err)
		goto EndOfSend
	}

	for {
		msg, _ = r.ReadString('\n')

		w = new(bytes.Buffer)
		// send msg
		if err := binary.Write(w, binary.BigEndian, uint16(len(msg)+2)); err != nil {
			fmt.Println("binary.Write msg err: ", err)
			goto EndOfSend
		}

		fmt.Printf("% 0x\n", w.Bytes())
		buf = append(buf, w.Bytes()...)
		buf = append(buf, msg...)
		if _, err := conn.Write(buf); err != nil {
			fmt.Println("write to server err: ", err)
			continue
		}
	}

EndOfSend:
	wg.Done()
}

func handleRecv(conn net.Conn, wg *sync.WaitGroup) {
	buf := make([]byte, 1024)
	for {
		if _, err := conn.Read(buf); err != nil {
			if err != io.EOF {
				fmt.Println("read server err: ", err)
			}
			goto EndOfRecv
		}

		fmt.Println("Received: ", string(buf))
	}

EndOfRecv:
	wg.Done()
}
