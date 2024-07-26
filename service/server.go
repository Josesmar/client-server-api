package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	timeoutMaxCallTheDollarQuoteAPI = 200 * time.Millisecond
	timeoutMaxPersistDataBase       = 10 * time.Millisecond
)

type CurrencyResponse struct {
	Bid string `json:"bid"`
}

type APIResponse struct {
	USDBRL CurrencyResponse `json:"USDBRL"`
}

var db *sql.DB

func main() {
	setupDB()
	defer db.Close()

	http.HandleFunc("/cotacao", handleCotacao)
	port := ":8080"
	fmt.Printf("server listening on the port %s...\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func setupDB() {
	var err error
	db, err = sql.Open("sqlite3", "./currencies.db")
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}

	err = createCurrencyTable()
	if err != nil {
		log.Fatalf("error creating table: %v\n", err)
	}
}

func createCurrencyTable() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS currency (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        bid REAL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`)
	return err
}

func handleCotacao(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeoutMaxCallTheDollarQuoteAPI)
	defer cancel()

	bid, err := fetchBid(ctx)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("timeout when making request dollar quote api")
		}
		http.Error(w, fmt.Sprintf("error when searching for quote: %v", err), http.StatusInternalServerError)
		return
	}

	ctxSave, cancelSave := context.WithTimeout(context.Background(), timeoutMaxPersistDataBase)
	defer cancelSave()

	err = saveBid(ctxSave, bid)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("timeout when saving data to database")
		}
		log.Printf("error saving to database: %v", err)
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"bid": bid})
}

func fetchBid(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://economia.awesomeapi.com.br/json/last/USD-BRL", nil)
	if err != nil {
		return "", fmt.Errorf("erro ao criar requisição HTTP: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("timeout when making request http")
		}
		return "", fmt.Errorf("errr when making http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error in http response: status %d", resp.StatusCode)
	}

	var apiResponse APIResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return "", fmt.Errorf("error when decoding JSON: %w", err)
	}

	log.Printf("data received from the API: %+v", apiResponse)

	return apiResponse.USDBRL.Bid, nil
}

func saveBid(ctx context.Context, bid string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		_, err := db.Exec("INSERT INTO currency (bid) VALUES (?)", bid)
		return err
	}
}

func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "error serializing response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(response)
}
