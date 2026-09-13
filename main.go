package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const projectName = "Certwatch"
const projectFocus = "TLS certificate expiration checker."

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func fileReport(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("expected a file, not a directory")
	}
	hash, err := hashFile(path)
	if err != nil {
		return err
	}
	fmt.Printf("Path: %s\nSize: %d bytes\nSHA-256: %s\n", path, info.Size(), hash)
	return nil
}

func dnsReport(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		fmt.Println(ip.String())
	}
	return nil
}

func certReport(host string) error {
	dialer := &net.Dialer{Timeout: 6 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(host, "443"), &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, cert := range conn.ConnectionState().PeerCertificates {
		fmt.Printf("Subject: %s\nIssuer: %s\nValid until: %s\n", cert.Subject.CommonName, cert.Issuer.CommonName, cert.NotAfter.Format(time.RFC3339))
		break
	}
	return nil
}

func headerReport(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return errors.New("URL must start with http:// or https://")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	for _, key := range []string{"Content-Security-Policy", "Strict-Transport-Security", "X-Frame-Options", "X-Content-Type-Options", "Referrer-Policy"} {
		value := response.Header.Get(key)
		if value == "" {
			value = "missing"
		}
		fmt.Printf("%s: %s\n", key, value)
	}
	return nil
}

func systemReport() {
	fmt.Printf("Project: %s\nFocus: %s\n", projectName, projectFocus)
	fmt.Printf("OS: %s\nArch: %s\nCPUs: %d\nGo: %s\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version())
}

func main() {
	mode := flag.String("mode", "system", "system, file, dns, cert, headers")
	path := flag.String("file", "", "file path")
	host := flag.String("host", "example.com", "host name")
	url := flag.String("url", "https://example.com", "URL")
	flag.Parse()

	var err error
	switch *mode {
	case "system":
		systemReport()
	case "file":
		err = fileReport(*path)
	case "dns":
		err = dnsReport(*host)
	case "cert":
		err = certReport(*host)
	case "headers":
		err = headerReport(*url)
	default:
		err = errors.New("unknown mode")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
