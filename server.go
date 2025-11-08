package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var reNonDigit = regexp.MustCompile(`\D`)
var hardcodedKey = ""

// Estruturas
type Resp struct {
	OK   bool        `json:"ok"`
	Info interface{} `json:"info,omitempty"`
	Err  string      `json:"error,omitempty"`
}

// JSON helper
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// CPF helpers
func isAllSame(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

func calcCheckDigit(digs string, weightStart int) int {
	sum := 0
	for i := 0; i < len(digs); i++ {
		sum += int(digs[i]-'0') * (weightStart - i)
	}
	r := sum % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

func validateCPFLocal(raw string) (bool, string) {
	if raw == "" {
		return false, "cpf vazio"
	}
	digits := reNonDigit.ReplaceAllString(raw, "")
	if len(digits) != 11 {
		return false, "cpf deve ter 11 dígitos"
	}
	if isAllSame(digits) {
		return false, "cpf inválido (todos dígitos iguais)"
	}
	base := digits[:9]
	d1 := calcCheckDigit(base, 10)
	if d1 != int(digits[9]-'0') {
		return false, "primeiro dígito verificador incorreto"
	}
	d2 := calcCheckDigit(base+string('0'+d1), 11)
	if d2 != int(digits[10]-'0') {
		return false, "segundo dígito verificador incorreto"
	}
	return true, ""
}

// Cartão helpers
func luhnCheck(card string) bool {
	digits := reNonDigit.ReplaceAllString(card, "")
	if len(digits) < 12 {
		return false
	}
	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

func extractBin(card string, length int) string {
	digits := reNonDigit.ReplaceAllString(card, "")
	if len(digits) < length {
		return ""
	}
	return digits[:length]
}

// APIs externas
func queryHandy(apiURL, apiKey, bin string, timeout time.Duration) (map[string]interface{}, error) {
	client := &http.Client{Timeout: timeout}
	u := strings.ReplaceAll(apiURL, "{bin}", bin)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GoHandyLookup/1.0")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return map[string]interface{}{
			"status":  res.StatusCode,
			"message": strings.TrimSpace(string(body)),
		}, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func viaCepLookup(cep string) (map[string]interface{}, error) {
	cepClean := strings.TrimSpace(cep)
	cepClean = reNonDigit.ReplaceAllString(cepClean, "")
	if cepClean == "" {
		return nil, fmt.Errorf("cep vazio")
	}
	u := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cepClean)
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("viacep status %s", res.Status)
	}
	var data map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}
	if v, ok := data["erro"]; ok {
		if bv, ok2 := v.(bool); ok2 && bv {
			return nil, fmt.Errorf("cep não encontrado")
		}
	}
	mapped := map[string]interface{}{
		"street":       firstString(data["logradouro"]),
		"neighborhood": firstString(data["bairro"]),
		"city":         firstString(data["localidade"]),
		"state":        firstString(data["uf"]),
		"cep":          firstString(data["cep"]),
	}
	return mapped, nil
}

func firstString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Handlers
func handleValidateCPF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Resp{OK: false, Err: "method"})
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: "json inválido"})
		return
	}
	raw, _ := m["cpf"].(string)
	ok, reason := validateCPFLocal(raw)
	if !ok {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: reason})
		return
	}
	writeJSON(w, http.StatusOK, Resp{OK: true})
}

func handleValidateCEP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Resp{OK: false, Err: "method"})
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: "json inválido"})
		return
	}
	cepRaw, _ := m["cep"].(string)
	info, err := viaCepLookup(cepRaw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Resp{OK: true, Info: info})
}

func handleValidateCard(apiURL, apiKey string, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, Resp{OK: false, Err: "method"})
			return
		}
		var m map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: "json inválido"})
			return
		}
		cardRaw, _ := m["card"].(string)
		card := strings.TrimSpace(cardRaw)
		if card == "" {
			writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: "card vazio"})
			return
		}

		valid := luhnCheck(card)
		bin := extractBin(card, 6)
		respBody := map[string]interface{}{
			"valid": valid,
			"bin":   bin,
		}

		if bin != "" {
			data, err := queryHandy(apiURL, apiKey, bin, timeout)
			if err != nil {
				respBody["bin_info_error"] = err.Error()
			} else {
				respBody["bin_info"] = data
			}
		}
		writeJSON(w, http.StatusOK, respBody)
	}
}

// Checkout simulado
func handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Resp{OK: false, Err: "method"})
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: "json inválido"})
		return
	}
	writeJSON(w, http.StatusOK, Resp{OK: true, Info: map[string]interface{}{"order_id": "TEST123456"}})
}

// Main
func main() {
	keyFlag := flag.String("key", "", "")
	apiURLFlag := flag.String("api-url", "https://data.handyapi.com/bin/{bin}", "")
	addr := flag.String("addr", ":8080", "")
	timeoutFlag := flag.Int("timeout", 12, "")
	staticDir := flag.String("static", "./static", "")
	flag.Parse()

	apiKey := ""
	if *keyFlag != "" {
		apiKey = *keyFlag
	} else if hardcodedKey != "" {
		apiKey = hardcodedKey
	} else if env := os.Getenv("HANDY_API_KEY"); env != "" {
		apiKey = env
	}
	timeout := time.Duration(*timeoutFlag) * time.Second

	http.Handle("/", http.FileServer(http.Dir(*staticDir)))
	http.HandleFunc("/api/validate-cpf", handleValidateCPF)
	http.HandleFunc("/api/validate-cep", handleValidateCEP)
	http.HandleFunc("/api/validate-card", handleValidateCard(*apiURLFlag, apiKey, timeout))
	http.HandleFunc("/api/checkout", handleCheckout)

	log.Println("listening on", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
