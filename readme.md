# DeceptiveDNS

DeceptiveDNS is a simple Go application that sets up a DNS server to respond to DNS queries for a specific domain with a specified IP address. This can be useful for testing or demonstration purposes, such as redirecting requests for a domain to a local development server or blackholing requests for a specific domain.

## How it works

When the DeceptiveDNS application is run, it starts a DNS server listening on port 53 (the default DNS port). You can specify the domain name to respond to using the `-domain` flag and optionally the IP address to respond with using the `-ip` flag. If the IP address is not provided, the server responds with the local IP address of the machine where the application is running.

When a DNS query is received for the specified domain, the server responds with the configured IP address. All other DNS queries are ignored.

The application logs each received request to the console, including the timestamp, domain name, and client IP address.

## Usage

### Prerequisites

- Go programming language installed on your system.
- Administrative privileges to run the application, as it listens on port 53 which is a privileged port.

### Installation

1. Clone this repository to your local machine:

    ```bash
    git clone https://github.com/your_username/DeceptiveDNS.git
    ```

2. Navigate to the project directory:

    ```bash
    cd DeceptiveDNS
    ```

3. Build the application:

    ```bash
    go build
    ```

### Running the application

Run the built executable with the desired domain name and optionally the IP address:

```bash
./DeceptiveDNS -domain example.com -ip 192.168.1.100
```

Replace `example.com` with the domain name you want to respond to, and `192.168.1.100` with the IP address you want to respond with. If you omit the `-ip` flag, the server will respond with the local IP address of your machine.

### Stopping the server

To stop the DNS server, press `Ctrl+C` in the terminal where the application is running.

## License

This project is licensed under the Creative Commons Public Domain - see the [LICENSE](LICENSE) file for details.
