package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type HTTPClient struct {
	*http.Client
}

func (client HTTPClient) getPageText(requestURL string) (string, error) {

	response, err := client.Get(requestURL)
	if err != nil {
		log.Printf("[%s] %s\n", requestURL, err)
		return "", err
	}

	if response.StatusCode != http.StatusOK {
		log.Printf("[%s] Wrong http response code=%d received\n", requestURL, response.StatusCode)
		return "", errors.New("Wrong http response code received")
	}

	pageText, err := ioutil.ReadAll(response.Body)
	response.Body.Close()

	if err != nil {
		log.Printf("[%s] %s", requestURL, err)
		return "", err
	}

	return string(pageText), nil
}

func (client *HTTPClient) findAndPrintLinesCount(requestURL string, whatToFind string, results chan<- int, taskGroup *sync.WaitGroup) {
	defer taskGroup.Done()

	pageText, err := client.getPageText(requestURL)
	if err != nil {
		return
	}

	// search is case-sensitive
	totalFound := strings.Count(pageText, whatToFind)

	fmt.Printf("Count for %s: %d\n", requestURL, totalFound)
	results <- totalFound
}

func startDispatcher(tasks <-chan string, total chan int, client *HTTPClient, textToFind string) {
	var tasksGroup sync.WaitGroup

	totalFound := 0
	tasksClosed := false
	results := make(chan int)

	for {
		select {
		case requestURL, ok := <-tasks:
			if ok {
				tasksGroup.Add(1)
				go client.findAndPrintLinesCount(requestURL, textToFind, results, &tasksGroup)
			} else if !tasksClosed {
				tasksClosed = true
				// waiting for all goroutines to complete
				// and then close results channel
				go func() {
					tasksGroup.Wait()
					close(results)
				}()
			}

		case count, ok := <-results:
			if !ok {
				total <- totalFound
				return
			}
			totalFound += count
		}
	}
}

func main() {
	const (
		requestTimeout      = time.Second * 10
		maximumWorkersCount = 5
		textToFind          = "Go"
	)

	tasks := make(chan string, maximumWorkersCount)
	total := make(chan int)

	// http client tuning: reasonable timeout added
	// we are not allowed to use global variables, so things are getting worse
	httpClient := &HTTPClient{&http.Client{Timeout: requestTimeout}}

	go startDispatcher(tasks, total, httpClient, textToFind)

	stdinScanner := bufio.NewScanner(os.Stdin)

	for stdinScanner.Scan() {

		requestURL := stdinScanner.Text()

		if _, err := url.ParseRequestURI(requestURL); err != nil {
			log.Printf("[%s] %s", requestURL, err)
			continue
		}

		// pause after reading StdIn, since we cannot handle more urls right now
		tasks <- requestURL
	}

	// no more tasks, it's time to receive results
	close(tasks)

	fmt.Println("Total:", <-total)
}
