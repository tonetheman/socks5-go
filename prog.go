package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
)

type SocksProxy struct {
	port     int
	globalId int
	debug    bool
}

func handleConnect(conn net.Conn, p *SocksProxy) {
	me := p.globalId
	p.globalId++
	log.Printf("%d: starting handleConnect now\n", me)

	var singleByte []byte = make([]byte, 1)
	count, err := conn.Read(singleByte)
	if err != nil || count != 1 {
		log.Fatalf("%d: unable to read from net for proto version: %v\n", me, err)
		return
	}
	protoVersion := singleByte[0]
	if protoVersion != 5 {
		log.Fatalf("%d: wrong protocol version", me)
		return
	}
	count, err = conn.Read(singleByte)
	if err != nil || count != 1 {
		log.Fatalf("%d: unable to read from net for numMethods: %v\n", me, err)
		return
	}
	numMethods := singleByte[0]
	if numMethods < 1 || numMethods > 255 {
		log.Fatalf("%d: incorrect number of methods: %d %v\n", me, numMethods, err)
		return
	}

	methodData := make([]uint, int(numMethods))
	for i := 0; i < int(numMethods); i++ {
		count, err := conn.Read(singleByte)
		if err != nil || count != 1 {
			log.Fatalf("%d: error reading a method in a loop: %v\n", me, err)
			return
		}
		methodData[i] = uint(singleByte[0])
	}

	// we are protocol version 5
	// and we do not take any auth method
	methodResponse := []byte{5, 0}

	conn.Write(methodResponse)

	// since we have no auth the client should
	// send the request details next
	count, err = conn.Read(singleByte)
	if err != nil || count != 1 {
		log.Fatalf("%d: error reading request details: %v\n", me, err)
		return
	}
	protoVersion = singleByte[0]
	if protoVersion != 5 {
		log.Fatalf("%d: invalid protoVersion: %d\n", me, protoVersion)
	}
	count, err = conn.Read(singleByte)
	if err != nil {
		log.Fatalf("%d: error reading request command: %v\n", me, err)
	}
	protoCommand := singleByte[0]

	count, err = conn.Read(singleByte)
	if err != nil {
		log.Fatalf("%d: error reading reserved: %v\n", me, err)
		return
	}
	protoReserved := singleByte[0]
	if protoReserved != 0 {
		log.Fatalf("%d: reserved not zero: %d\n", me, protoReserved)
		return
	}

	count, err = conn.Read(singleByte)
	if err != nil {
		log.Fatalf("%d: error reading atype: %v\n", me, err)
		return
	}
	protoAtype := singleByte[0]
	var protoIpAddress []byte
	var protoDomainString string
	if protoAtype == 1 {
		// ip v4 address

		protoIpAddress = make([]byte, 4)
		n, err := conn.Read(protoIpAddress)
		if err != nil {
			log.Fatalf("%d: could not read the ipv4 bytes: %v\n", me, err)
			return
		}
		log.Printf("%d: ipv4 addr %d %v\n", me, n, protoIpAddress)
	} else if protoAtype == 3 {
		// domain name
		count, err = conn.Read(singleByte)
		if err != nil {
			log.Fatalf("%d: could not read domain len: %v\n", me, err)
			return
		}
		protoDomainLen := singleByte[0]

		protoDomainName := make([]byte, protoDomainLen)
		n, err := conn.Read(protoDomainName)
		if err != nil {
			log.Fatalf("%d: could not read domain name: %v\n", me, err)
			return
		}
		protoDomainString = string(protoDomainName)
		log.Printf("%d: domain name is: %d %s\n", me, n, protoDomainName)
	} else if protoAtype == 4 {
		// ipv6 address
		log.Printf("%d: not handling ipv6 address\n", me)
	} else {
		log.Printf("%d: unknown atype: %d\n", me, protoAtype)
		return
	}
	// read the dest port
	protoDestPort := make([]byte, 2)
	n, err := conn.Read(protoDestPort)
	if err != nil {
		log.Printf("%d: could not read port: %v\n", me, err)
		return
	}
	iPort := binary.BigEndian.Uint16(protoDestPort)
	log.Printf("%d: port is %d %d %d\n", me, n, protoDestPort, iPort)

	if protoCommand == 1 {
		// this is a connect
		log.Printf("%d: making outbound connection\n", me)
		var cs string
		if protoAtype == 1 {
			cs = fmt.Sprintf("%d.%d.%d.%d:%d",
				protoIpAddress[0], protoIpAddress[1],
				protoIpAddress[2], protoIpAddress[3], iPort)
		} else if protoAtype == 3 {
			cs = fmt.Sprintf("%s:%d",
				protoDomainString, iPort)
		}
		log.Printf("%d: cs is %s\n", me, cs)
		outNewConn, err := net.Dial("tcp4", cs)
		otherConnId := p.globalId
		p.globalId++

		if err != nil {
			log.Fatalf("%d: unable to get outbound connection: %v\n", me, err)
			return
		} else {
			log.Printf("%d: got outbound connection: %v\n", err, outNewConn)
		}

		// RETURN FROM CONNECT HERE
		connectResponse := make([]byte, 10)
		// proto version 5
		connectResponse[0] = 5
		connectResponse[1] = 0 // success
		connectResponse[2] = 0 // reserved
		connectResponse[3] = 1 // address type 1 ipv4
		connectResponse[4] = 0
		connectResponse[5] = 0
		connectResponse[6] = 0
		connectResponse[7] = 0
		connectResponse[8] = 0 // port
		connectResponse[9] = 0
		log.Printf("%d: sending response to connect now\n", me)
		count, err = conn.Write(connectResponse)
		if err != nil {
			log.Fatalf("%d: err on write to client: %v\n", me, err)
			return
		}
		log.Printf("%d: wrote this number of bytes back to client: %d\n", me, count)
		log.Printf("%d: entering for loop for connection now...\n", me)

		go pCopy(me, conn, outNewConn, p)

		go pCopy(otherConnId, outNewConn, conn, p)
		log.Printf("%d: got out of connection forloop!!!", me)
	} else if protoCommand == 2 {
		// this is a bind
		log.Println("GOT A BIND COMMAND WHAT TO DO?")
		return
	} else if protoCommand == 3 {
		// this is a UDP associate
		log.Println("GOT A UDP COMMAND WHAT TO DO?")
		return
	} else {
		log.Println("err on request detail")
		return
	}
	log.Println("returning now")
	return

}

func pb(buffer []byte, len int, p *SocksProxy) {
	if !p.debug {
		return
	}
	msg := fmt.Sprintf("in pb now with this many to print %d\n", len)
	os.Stdout.WriteString(msg)
	for i := 0; i < len; i++ {
		msg := fmt.Sprintf("%d ", buffer[i])
		os.Stdout.WriteString(msg)
	}
	os.Stdout.WriteString("\n")
}

func cout(msg string) {
	os.Stdout.WriteString(msg)
}

func pCopy(id int, src net.Conn, dst net.Conn, p *SocksProxy) {
	msg := fmt.Sprintf("%d: start of tonyCopy\n", id)
	cout(msg)
	var buffer []byte = make([]byte, 512)
	var brokeForRead bool = false
	var brokeForWrite bool = false
	for {
		// read from src
		msg := fmt.Sprintf("%d: before read\n", id)
		cout(msg)
		count, err := src.Read(buffer)
		if err != nil {
			brokeForRead = true
			fmt.Println(id, err)
			break
		}
		// push to dst
		// this will buffer the stdout writes so to make sure ordering is correct
		// us os.Stdout.WriteString
		//fmt.Println(id, "read from", src, dst, count)
		msg = fmt.Sprintf("%d : %s : %d\n", id, "read from", count)
		os.Stdout.WriteString(msg)
		pb(buffer, count, p)
		//fmt.Println(buffer[:count])
		count, err = dst.Write(buffer[:count])
		if err != nil {
			brokeForWrite = true
			fmt.Println(id, "err on write", err)
			break
		} else {

		}
		fmt.Println(id, "wrote this amount of bytes", count)
	}
	if brokeForRead {
		fmt.Println(id, "read broke")
	}
	if brokeForWrite {
		fmt.Println(id, "write broke")
	}
	src.Close()
	dst.Close()
	fmt.Println(id, "tonyCopy broke from for loop")
}

func newSocksProxyServer(_port int) *SocksProxy {
	res := SocksProxy{port: _port,
		globalId: 0}
	return &res
}

func (p *SocksProxy) ListenAndServe() {
	port := fmt.Sprintf(":%d", p.port)
	log.Println("listening on this: ", port)
	listnr, err := net.Listen("tcp4", port)
	if err != nil {
		fmt.Println("unable to listen on", port, err)
		return
	}
	defer listnr.Close()

	for {
		conn, err := listnr.Accept()
		if err != nil {
			log.Println("got an err on accept", err)
			return
		}
		go handleConnect(conn, p)
	}
}

func atoi(s string) int {
	res, _ := strconv.Atoi(s)
	return res
}

func main() {
	args := os.Args
	if len(args) == 1 {
		// they ran without any params
		fmt.Println("need a port")
		return
	}

	port := args[1]
	proxy := newSocksProxyServer(atoi(port))
	proxy.ListenAndServe()
}
