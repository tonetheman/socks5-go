package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
)

func handleConnect(conn net.Conn) {
	fmt.Println("handling connect!")
	defer conn.Close()
	reader := bufio.NewReader(conn)
	protoVersion, err := reader.ReadByte()
	if err != nil || protoVersion != 5 {
		fmt.Println("wrong protocol version", protoVersion, err)
		return
	}
	numMethods, err := reader.ReadByte()
	if err != nil || numMethods < 1 || numMethods > 255 {
		fmt.Println("incorrect num of methods", numMethods, err)
	}

	methodData := make([]uint, numMethods)
	for i := 0; i < int(numMethods); i++ {
		tmp, err := reader.ReadByte()
		if err != nil {
			fmt.Println("err reading a method", err)
			return
		}
		methodData[i] = uint(tmp)
	}
	//fmt.Println("methods", methodData)

	// we are protocol version 5
	// and we do not take any auth method
	methodResponse := []byte{5, 0}

	conn.Write(methodResponse)

	// since we have no auth the client should
	// send the request details next
	protoVersion, err = reader.ReadByte()
	if err != nil || protoVersion != 5 {
		fmt.Println("err reading request details", protoVersion, err)
		return
	}
	protoCommand, err := reader.ReadByte()
	if err != nil {
		fmt.Println("err reading request command", err)
	}
	protoReserved, err := reader.ReadByte()
	if err != nil || protoReserved != 0 {
		fmt.Println("weird in reserved", protoReserved, err)
		return
	}
	protoAtype, err := reader.ReadByte()
	if err != nil {
		fmt.Println("unable to read atype", err)
		return
	}
	var protoIpAddress []byte
	var protoDomainString string
	if protoAtype == 1 {
		// ip v4 address

		protoIpAddress = make([]byte, 4)
		n, err := reader.Read(protoIpAddress)
		if err != nil {
			fmt.Println("could not read the ipv4 4 bytes")
			return
		}
		fmt.Println("ip v4 addr", protoIpAddress, n)

	} else if protoAtype == 3 {
		// domain name
		protoDomainLen, err := reader.ReadByte()
		if err != nil {
			fmt.Println("could not read domain len", err)
			return
		}
		protoDomainName := make([]byte, protoDomainLen)
		n, err := reader.Read(protoDomainName)
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
	n, err := reader.Read(protoDestPort)
	if err != nil {
		fmt.Println("could not read port", err)
		return
	}
	//portBuffer := bytes.NewReader(protoDestPort)
	//var iPort int = 0
	//err = binary.Read(portBuffer, binary.BigEndian, &iPort)
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
		conn.Write(connectResponse)

		/*
			//nope
			complete := make(chan bool, 2)
			ch1 := make(chan bool, 1)
			ch2 := make(chan bool, 1)
			copyBytes(conn, outNewConn, complete, ch1, ch2)
			copyBytes(outNewConn, conn, complete, ch2, ch1)
			<-complete
			<-complete
		*/

		for {
			go io.Copy(conn, outNewConn)
			go io.Copy(outNewConn, conn)

			// tried this it did not work
			//go relay(conn, outNewConn)
			//go relay(outNewConn, conn)
		}

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

	for {
	}

}

func copyBytes(src net.Conn, dst net.Conn,
	complete chan bool, done chan bool, otherDone chan bool) {
	var err error = nil
	var bytes []byte = make([]byte, 256)
	var read int = 0
	for {
		select {
		case <-otherDone:
			complete <- true
			return
		default:
			read, err = src.Read(bytes)
			if err != nil {
				complete <- true
				done <- true
				return
			}
			_, err = dst.Write(bytes[:read])
			if err != nil {
				complete <- true
				done <- true
				break
			}
		}
	}
}

// nope
/*
func relay(src net.Conn, dst net.Conn) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
	return
}
*/

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
