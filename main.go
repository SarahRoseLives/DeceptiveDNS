package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// Parse command line arguments
	domainPtr := flag.String("domain", "", "Domain name to respond to")
	ipPtr := flag.String("ip", "", "IP address to respond with (optional)")
	flag.Parse()

	// Validate command line arguments
	if *domainPtr == "" {
		fmt.Println("Please provide a domain name using the -domain flag")
		os.Exit(1)
	}

	var ip string
	if *ipPtr != "" {
		ip = *ipPtr
	} else {
		// Get local IP address
		localIP, err := getLocalIP()
		if err != nil {
			fmt.Println("Failed to get local IP address:", err)
			os.Exit(1)
		}
		ip = localIP
	}

	// Set up DNS server
	server := &dnsServer{
		domain: *domainPtr,
		ip:     ip,
	}
	go server.start()

	// Wait for interruption (Ctrl+C) to close the server
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-interrupt

	fmt.Println("DNS server stopped.")
}

type dnsServer struct {
	domain string
	ip     string
}

func (s *dnsServer) start() {
	addr := ":53" // Default DNS port
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Printf("DNS server listening on %s...\n", addr)

	for {
		buf := make([]byte, 1024)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("Error reading from UDP connection:", err)
			continue
		}

		req := buf[:n]
		go s.handleRequest(conn, addr, req)
	}
}

func (s *dnsServer) handleRequest(conn *net.UDPConn, addr *net.UDPAddr, req []byte) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in handleRequest:", r)
		}
	}()

	// Parse DNS request
	var msg dnsMsg
	if err := msg.unpack(req); err != nil {
		log.Println("Error unpacking DNS message:", err)
		return
	}

	// Check if the request is for the domain we're listening to
	if strings.EqualFold(msg.Question.Name, s.domain) {
		// Send DNS response
		resp := dnsMsg{
			ID:       msg.ID,
			Flags:    dnsFlagsResponse,
			Question: msg.Question,
			Answers: []dnsResourceRecord{
				{
					Name:  s.domain,
					Type:  dnsTypeA,
					Class: dnsClassIN,
					TTL:   3600, // TTL in seconds
					Data:  net.ParseIP(s.ip),
				},
			},
		}

		respBytes, err := resp.pack()
		if err != nil {
			log.Println("Error packing DNS response:", err)
			return
		}

		if _, err := conn.WriteToUDP(respBytes, addr); err != nil {
			log.Println("Error sending DNS response:", err)
			return
		}

		// Log the request
		log.Printf("[%s] Request for %s from %s\n", time.Now().Format("2006-01-02 15:04:05"), s.domain, addr)
	}
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}

	return "", fmt.Errorf("no local IP address found")
}

type dnsMsg struct {
	ID       uint16
	Flags    uint16
	Question dnsQuestion
	Answers  []dnsResourceRecord
}

type dnsQuestion struct {
	Name  string
	Type  uint16
	Class uint16
}

type dnsResourceRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	Data  net.IP
}

func (msg *dnsMsg) unpack(data []byte) error {
	// Ensure data is at least 12 bytes long (DNS header)
	if len(data) < 12 {
		return fmt.Errorf("invalid DNS message: message too short")
	}

	// Unpack DNS header
	msg.ID = binary.BigEndian.Uint16(data[:2])
	msg.Flags = binary.BigEndian.Uint16(data[2:4])

	// Unpack DNS question section
	qStart := 12
	for qStart < len(data) && data[qStart] != 0 {
		qStart++
	}
	if qStart+4 > len(data) {
		return fmt.Errorf("invalid DNS message: malformed question section")
	}
	msg.Question.Name = string(data[12:qStart])
	msg.Question.Type = binary.BigEndian.Uint16(data[qStart+1 : qStart+3])
	msg.Question.Class = binary.BigEndian.Uint16(data[qStart+3 : qStart+5])

	// Other sections are not supported in this example

	return nil
}

func (msg *dnsMsg) pack() ([]byte, error) {
	// Pack DNS header
	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[:2], msg.ID)
	binary.BigEndian.PutUint16(header[2:4], msg.Flags)

	// Pack DNS question section
	qName := strings.Split(msg.Question.Name, ".")
	qNameBytes := make([]byte, 0)
	for _, label := range qName {
		qNameBytes = append(qNameBytes, byte(len(label)))
		qNameBytes = append(qNameBytes, []byte(label)...)
	}
	qNameBytes = append(qNameBytes, 0) // Null-terminate domain name
	qType := make([]byte, 2)
	binary.BigEndian.PutUint16(qType, msg.Question.Type)
	qClass := make([]byte, 2)
	binary.BigEndian.PutUint16(qClass, msg.Question.Class)

	return append(header, append(qNameBytes, append(qType, qClass...)...)...), nil
}

const (
	dnsTypeA         = 1
	dnsClassIN       = 1
	dnsFlagsResponse = 0x8180 // Response flag
)
