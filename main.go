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
	domainPtr := flag.String("domain", "", "Domain name to respond to (e.g. garden.local)")
	ipPtr := flag.String("ip", "", "IP address to respond with (optional, defaults to local LAN IP)")
	flag.Parse()

	if *domainPtr == "" {
		fmt.Println("Please provide a domain name using the -domain flag")
		os.Exit(1)
	}

	ip := *ipPtr
	if ip == "" {
		var err error
		ip, err = getLocalIP()
		if err != nil {
			fmt.Println("Failed to get local IP address:", err)
			os.Exit(1)
		}
	}

	server := &dnsServer{
		domain: strings.TrimSuffix(*domainPtr, "."),
		ip:     ip,
	}
	go server.start()

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
	addr := ":53"
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
	buf := make([]byte, 512)
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("ReadFromUDP error:", err)
			continue
		}
		go s.handleRequest(conn, client, buf[:n])
	}
}

func (s *dnsServer) handleRequest(conn *net.UDPConn, client *net.UDPAddr, req []byte) {
	id := binary.BigEndian.Uint16(req[:2])
	qname, qtype, _, qEnd, err := parseQuestion(req)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if err != nil {
		log.Printf("[%s] Malformed request from %s: %v", timestamp, client, err)
		return
	}
	log.Printf("[%s] Query for %s (type %d) from %s", timestamp, qname, qtype, client)
	if strings.EqualFold(qname, s.domain) {
		switch qtype {
		case 1: // Type A
			resp := buildResponse(id, req, qEnd, s.ip)
			_, err := conn.WriteToUDP(resp, client)
			if err != nil {
				log.Printf("[%s] Error sending response to %s: %v", timestamp, client, err)
			}
		case 28: // Type AAAA (IPv6)
			resp := buildNoAnswerResponse(id, req, qEnd)
			_, err := conn.WriteToUDP(resp, client)
			if err != nil {
				log.Printf("[%s] Error sending AAAA response to %s: %v", timestamp, client, err)
			}
		}
	}
}

func parseQuestion(msg []byte) (name string, qtype, qclass uint16, end int, err error) {
	var parts []string
	i := 12
	for ; i < len(msg); {
		if msg[i] == 0 {
			i++
			break
		}
		l := int(msg[i])
		if l == 0 || i+1+l > len(msg) {
			err = fmt.Errorf("label length out of range")
			return
		}
		parts = append(parts, string(msg[i+1:i+1+l]))
		i += 1 + l
	}
	if i+4 > len(msg) {
		err = fmt.Errorf("not enough bytes for qtype/qclass")
		return
	}
	name = strings.Join(parts, ".")
	qtype = binary.BigEndian.Uint16(msg[i : i+2])
	qclass = binary.BigEndian.Uint16(msg[i+2 : i+4])
	end = i + 4
	return
}

func buildResponse(id uint16, req []byte, qEnd int, ipstr string) []byte {
	// Header + question
	resp := make([]byte, qEnd)
	copy(resp, req[:qEnd])

	// Set response flag and ANCOUNT=1
	binary.BigEndian.PutUint16(resp[2:4], 0x8180) // Standard query response, no error
	binary.BigEndian.PutUint16(resp[6:8], 1)      // ANCOUNT = 1

	// Answer section
	answer := make([]byte, 16)
	// Name pointer to question (0xC00C)
	answer[0] = 0xC0
	answer[1] = 0x0C
	// Type A
	answer[2] = 0x00
	answer[3] = 0x01
	// Class IN
	answer[4] = 0x00
	answer[5] = 0x01
	// TTL
	binary.BigEndian.PutUint32(answer[6:10], 300)
	// RDLENGTH
	answer[10] = 0x00
	answer[11] = 0x04
	// RDATA (IPv4 address)
	ip := net.ParseIP(ipstr).To4()
	if ip == nil {
		ip = net.ParseIP("127.0.0.1").To4()
	}
	copy(answer[12:16], ip)

	return append(resp, answer...)
}

func buildNoAnswerResponse(id uint16, req []byte, qEnd int) []byte {
	resp := make([]byte, qEnd)
	copy(resp, req[:qEnd])
	binary.BigEndian.PutUint16(resp[2:4], 0x8180) // Standard response, no error
	// QDCOUNT already 1, ANCOUNT=0, NSCOUNT=0, ARCOUNT=0
	return resp
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok &&
			!ipnet.IP.IsLoopback() &&
			ipnet.IP.To4() != nil {
			return ipnet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("no local IP address found")
}
