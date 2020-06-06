package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
)

func handleConnect(conn net.Conn) {
	os.Stdout.WriteString("starting handleConnect now\n")
	//defer conn.Close()
	var singleByte []byte = make([]byte, 1)

	count, err := conn.Read(singleByte)
	if err != nil || count != 1 {
		fmt.Println("unable to read from net for proto version", err)
		return
	}
	protoVersion := singleByte[0]
	if protoVersion != 5 {
		fmt.Println("wrong protocol version", protoVersion)
		return
	}
	count, err = conn.Read(singleByte)
	if err != nil || count != 1 {
		fmt.Println("unalbe to read from net for numMethods", err)
		return
	}
	numMethods := singleByte[0]
	if numMethods < 1 || numMethods > 255 {
		fmt.Println("incorrect num of methods", numMethods, err)
		return
	}

	methodData := make([]uint, int(numMethods))
	for i := 0; i < int(numMethods); i++ {
		count, err := conn.Read(singleByte)
		if err != nil || count != 1 {
			fmt.Println("err reading a method", err)
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
		fmt.Println("err reading request details", err)
		return
	}
	protoVersion = singleByte[0]
	if protoVersion != 5 {
		fmt.Println("invalid proroversion", protoVersion)
	}
	count, err = conn.Read(singleByte)
	if err != nil {
		fmt.Println("err reading request command", err)
	}
	protoCommand := singleByte[0]

	count, err = conn.Read(singleByte)
	if err != nil {
		fmt.Println("weird in reserved", err)
		return
	}
	protoReserved := singleByte[0]
	if protoReserved != 0 {
		fmt.Println("reserved not zero", protoReserved)
		return
	}

	count, err = conn.Read(singleByte)
	if err != nil {
		fmt.Println("unable to read atype", err)
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
			fmt.Println("could not read the ipv4 4 bytes")
			return
		}
		fmt.Println("ip v4 addr", protoIpAddress, n)

	} else if protoAtype == 3 {
		// domain name
		count, err = conn.Read(singleByte)
		if err != nil {
			fmt.Println("could not read domain len", err)
			return
		}
		protoDomainLen := singleByte[0]

		protoDomainName := make([]byte, protoDomainLen)
		n, err := conn.Read(protoDomainName)
		if err != nil {
			fmt.Println("could not read domain name")
			return
		}
		protoDomainString = string(protoDomainName)
		fmt.Println("read this domain", n, protoDomainName)

	} else if protoAtype == 4 {
		// ipv6 address
	} else {
		fmt.Println("unknown atype", protoAtype)
		return
	}
	// read the dest port
	protoDestPort := make([]byte, 2)
	n, err := conn.Read(protoDestPort)
	if err != nil {
		fmt.Println("could not read port", err)
		return
	}
	iPort := binary.BigEndian.Uint16(protoDestPort)
	fmt.Println("port is", n, protoDestPort, iPort)

	if protoCommand == 1 {
		// this is a connect
		fmt.Println("making outbound connection...")
		var cs string
		if protoAtype == 1 {
			cs = fmt.Sprintf("%d.%d.%d.%d:%d",
				protoIpAddress[0], protoIpAddress[1],
				protoIpAddress[2], protoIpAddress[3], iPort)
		} else if protoAtype == 3 {
			cs = fmt.Sprintf("%s:%d",
				protoDomainString, iPort)
		}
		fmt.Println("cs is", cs)
		//var outNetConn net.Conn
		outNewConn, err := net.Dial("tcp4", cs)
		if err != nil {
			fmt.Println("unable to get an outbound net connection", err)
			return
		} else {
			fmt.Println("got outbound connection!", outNewConn)
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
		fmt.Println("sending response to connect now")
		count, err = conn.Write(connectResponse)
		if err != nil {
			fmt.Println("err on write to client", err)
			return
		}
		fmt.Println("wrote this number of bytes back to client", count)

		fmt.Println("entering forloop for connection now...")

		go tonyCopy(1, conn, outNewConn)
		go tonyCopy(2, outNewConn, conn)

		// tried this it did not work
		//go relay(conn, outNewConn)
		//go relay(outNewConn, conn)

		fmt.Println("got out of connection forloop!!!")

	} else if protoCommand == 2 {
		// this is a bind
		fmt.Println("GOT A BIND COMMAND WHAT TO DO?")
		return
	} else if protoCommand == 3 {
		// this is a UDP associate
		fmt.Println("GOT A UDP COMMAND WHAT TO DO?")
		return
	} else {
		fmt.Println("err on request detail", protoCommand)
		return
	}
	fmt.Println("returning now")
	return

	// need to connect tcp wise to the port requested and the IP address requested
	// or domain

	// need to move any other traffic along down that socket

}

func pb(buffer []byte, len int) {
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

func tonyCopy(id int, src net.Conn, dst net.Conn) {
	msg := fmt.Sprintf("%d: start of tonyCopy\n", id)
	cout(msg)
	var buffer []byte = make([]byte, 512)
	for {
		// read from src
		msg := fmt.Sprintf("%d: before read\n", id)
		cout(msg)
		count, err := src.Read(buffer)
		if err != nil {
			fmt.Println(id, err)
			break
		}
		// push to dst
		// this will buffer the stdout writes so to make sure ordering is correct
		// us os.Stdout.WriteString
		//fmt.Println(id, "read from", src, dst, count)
		msg = fmt.Sprintf("%d : %s : %d\n", id, "read from", count)
		os.Stdout.WriteString(msg)
		pb(buffer, count)
		//fmt.Println(buffer[:count])
		count, err = dst.Write(buffer[:count])
		if err != nil {
			fmt.Println(id, "err on write", err)
			break
		} else {

		}
		fmt.Println(id, "wrote this amount of bytes", count)
	}
	fmt.Println(id, "tonyCopy broke from for loop")
}

func main() {
	args := os.Args
	if len(args) == 1 {
		// they ran without any params
		fmt.Println("need a port")
		return
	}

	port := ":" + args[1]
	listnr, err := net.Listen("tcp4", port)
	if err != nil {
		fmt.Println("unable to listen on", port, err)
		return
	}
	defer listnr.Close()

	for {
		conn, err := listnr.Accept()
		if err != nil {
			fmt.Println("got an err on accept", err)
			return
		}
		go handleConnect(conn)
	}
}
