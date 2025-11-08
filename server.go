package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var reNonDigit = regexp.MustCompile(`\D`)
var hardcodedKey = ""

type Resp struct {
	OK   bool        `json:"ok"`
	Info interface{} `json:"info,omitempty"`
	Err  string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

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

func viaCepLookup(cep string) (map[string]interface{}, error) {
	cepClean := reNonDigit.ReplaceAllString(strings.TrimSpace(cep), "")
	if cepClean == "" {
		return nil, fmt.Errorf("cep vazio")
	}
	res, err := http.Get(fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cepClean))
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
	return map[string]interface{}{
		"street":       firstString(data["logradouro"]),
		"neighborhood": firstString(data["bairro"]),
		"city":         firstString(data["localidade"]),
		"state":        firstString(data["uf"]),
		"cep":          firstString(data["cep"]),
	}, nil
}

func firstString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

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
	ok, reason := validateCPFLocal(m["cpf"].(string))
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
	info, err := viaCepLookup(m["cep"].(string))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, Resp{OK: true, Info: info})
}

func handleValidateCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Resp{OK: false, Err: "method"})
		return
	}
	var m map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Err: "json inválido"})
		return
	}
	card := m["card"].(string)
	valid := luhnCheck(card)
	writeJSON(w, http.StatusOK, map[string]interface{}{"valid": valid})
}

func handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Resp{OK: false, Err: "method"})
		return
	}
	writeJSON(w, http.StatusOK, Resp{OK: true, Info: map[string]string{"order_id": fmt.Sprintf("%d", time.Now().Unix())}})
}

func main() {
	addr := flag.String("addr", ":8080", "")
	staticDir := flag.String("static", "./static", "")
	flag.Parse()

	http.Handle("/", http.FileServer(http.Dir(*staticDir)))
	http.HandleFunc("/api/validate-cpf", handleValidateCPF)
	http.HandleFunc("/api/validate-cep", handleValidateCEP)
	http.HandleFunc("/api/validate-card", handleValidateCard)
	http.HandleFunc("/api/checkout", handleCheckout)

	log.Println("Listening on", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
