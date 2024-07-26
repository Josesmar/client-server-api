package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const timeoutMaxResponseAPI = 10 * time.Millisecond

type CurrencyResponse struct {
	Bid string `json:"bid"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutMaxResponseAPI)
	defer cancel()

	bid, err := fetchBid(ctx)
	if err != nil {
		log.Fatalf("error getting quote: %v", err)
	}

	err = saveToFile(bid)
	if err != nil {
		log.Fatalf("error saving to file: %v", err)
	}

	fmt.Println("quote saved in quotacao.txt successfully!")
}

func fetchBid(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/cotacao", nil)
	if err != nil {
		return "", fmt.Errorf("error creating http request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("timeout when making request http error: %w", err)
		}
		return "", fmt.Errorf("error when making http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error in http response: status %d", resp.StatusCode)
	}

	var data CurrencyResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", fmt.Errorf("error when decoding JSON: %w", err)
	}

	return data.Bid, nil
}

func saveToFile(bid string) error {
	file, err := os.Create("cotacao.txt")
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "Dólar: %s\n", bid)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}
