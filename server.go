package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// --- Estruturas de Dados ---

type Address struct {
	CEP         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	UF          string `json:"uf"`
	ViaCEPError bool   `json:"erro"`
}

type CheckoutRequest struct {
	CardNumber string `json:"card_number"`
	CardHolder string `json:"card_holder"`
	Expiration string `json:"expiration_date"`
	CVV        string `json:"cvv"`
	CEP        string `json:"cep"`
}

type CheckoutResponse struct {
	Success     bool    `json:"success"`
	Message     string  `json:"message"`
	CardBrand   string  `json:"card_brand,omitempty"`
	AddressInfo Address `json:"address_info,omitempty"`
}

// --- Lógica de Negócio ---

func GetCardBrand(cardNumber string) string {
	re := regexp.MustCompile(`\D`)
	digits := re.ReplaceAllString(cardNumber, "")

	if len(digits) < 4 {
		return "Desconhecida"
	}
	switch {
	case strings.HasPrefix(digits, "4"):
		return "Visa"
	case strings.HasPrefix(digits, "50") || (digits >= "5600" && digits <= "5899"):
		return "Elo"
	case digits >= "5100" && digits <= "5599":
		return "Mastercard"
	case strings.HasPrefix(digits, "34") || strings.HasPrefix(digits, "37"):
		return "American Express"
	case strings.HasPrefix(digits, "6"):
		return "Discover"
	default:
		return "Desconhecida"
	}
}

func FetchAddressFromViaCEP(cep string) (Address, error) {
	re := regexp.MustCompile(`\D`)
	cleanCEP := re.ReplaceAllString(cep, "")
	if len(cleanCEP) != 8 {
		return Address{}, fmt.Errorf("CEP inválido")
	}

	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cleanCEP)

	resp, err := http.Get(url)
	if err != nil {
		return Address{}, fmt.Errorf("CEP inválido")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Address{}, fmt.Errorf("CEP não encontrado")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Address{}, fmt.Errorf("CEP inválido")
	}

	var address Address
	if err := json.Unmarshal(body, &address); err != nil {
		return Address{}, fmt.Errorf("CEP inválido")
	}

	if address.ViaCEPError {
		return Address{}, fmt.Errorf("CEP não encontrado")
	}

	return address, nil
}

// --- Handlers ---

// NOVO HANDLER: /api/lookup-cep
// Este handler é usado para a validação em tempo real no frontend.
func LookupCEPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Método não permitido."})
		return
	}

	var req struct {
		CEP string `json:"cep"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "JSON inválido."})
		return
	}

	address, err := FetchAddressFromViaCEP(req.CEP)
	if err != nil {
		// Retorna o erro específico ("CEP inválido" ou "CEP não encontrado")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Sucesso: retorna o endereço
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(address)
}

// HANDLER DE CHECKOUT (invariável, mas ainda seguro)
func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "Método não permitido."})
		return
	}

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "Payload JSON inválido."})
		return
	}

	// 1. Revalidação do Endereço (Segurança)
	address, err := FetchAddressFromViaCEP(req.CEP)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: err.Error()})
		return
	}

	// 2. Determina a Bandeira
	cardBrand := GetCardBrand(req.CardNumber)
	if cardBrand == "Desconhecida" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "Bandeira do cartão desconhecida."})
		return
	}

	// 3. Simulação de Pagamento
	if len(req.CVV) < 3 || len(req.Expiration) < 5 {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "CVV ou Data de Validade incorretos."})
		return
	}

	// SUCESSO
	w.WriteHeader(http.StatusOK)
	response := CheckoutResponse{
		Success:     true,
		Message:     "Checkout APROVADO! ID: TEST123456",
		CardBrand:   cardBrand,
		AddressInfo: address,
	}
	json.NewEncoder(w).Encode(response)
}

//Função Principal e Setup de Rotas

func main() {
	staticDir := "./static"
	http.Handle("/", http.FileServer(http.Dir(staticDir)))

	// Registra os dois endpoints da API
	http.HandleFunc("/api/lookup-cep", LookupCEPHandler) // Novo endpoint
	http.HandleFunc("/api/checkout", CheckoutHandler)

	const port = ":8080"
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Erro ao iniciar o servidor: %v", err)
	}
}
