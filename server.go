package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os" // Necessário para acessar variáveis de ambiente
	"regexp"
	"strings"

	_ "github.com/lib/pq" // Driver PostgreSQL
)

// Variável global para a conexão com o banco de dados
var db *sql.DB

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
	Number     string `json:"number"`
}

type CheckoutResponse struct {
	Success     bool    `json:"success"`
	Message     string  `json:"message"`
	CardBrand   string  `json:"card_brand,omitempty"`
	AddressInfo Address `json:"address_info,omitempty"`
}

// --- Funções de Serviço ---

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
		return Address{}, fmt.Errorf("cep inválido")
	}

	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cleanCEP)

	resp, err := http.Get(url)
	if err != nil {
		return Address{}, fmt.Errorf("erro ao consultar viacep")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Address{}, fmt.Errorf("cep não encontrado")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Address{}, fmt.Errorf("erro ao ler resposta do cep")
	}

	var address Address
	if err := json.Unmarshal(body, &address); err != nil {
		return Address{}, fmt.Errorf("resposta do viacep inválida")
	}

	if address.ViaCEPError {
		return Address{}, fmt.Errorf("cep não encontrado")
	}

	return address, nil
}

// --- Handlers ---

func LookupCEPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "método não permitido."})
		return
	}

	var req struct {
		CEP string `json:"cep"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "json inválido."})
		return
	}

	address, err := FetchAddressFromViaCEP(req.CEP)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(address)
}

func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "método não permitido."})
		return
	}

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "payload json inválido."})
		return
	}

	// 1. Validação de Campos
	if req.Number == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "o campo 'Número' do endereço é obrigatório."})
		return
	}

	// 2. Revalidação do Endereço (Segurança)
	address, err := FetchAddressFromViaCEP(req.CEP)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: err.Error()})
		return
	}

	// 3. Determina a Bandeira
	cardBrand := GetCardBrand(req.CardNumber)
	if cardBrand == "Desconhecida" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "bandeira do cartão desconhecida."})
		return
	}

	// 4. Simulação de Pagamento (Verificações mínimas)
	if len(req.CVV) < 3 || len(req.Expiration) < 5 {
		w.WriteHeader(http.StatusPaymentRequired)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "cvv ou data de validade incorretos na simulação."})
		return
	}

	// --- LÓGICA DE PERSISTÊNCIA (Salvar no DB) ---

	if db == nil {
		log.Println("Checkout falhou: Conexão com o DB não está ativa.")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "pagamento aprovado, mas erro interno: falha ao registrar o pedido no sistema."})
		return
	}

	fullAddress := fmt.Sprintf("%s, %s - %s. %s - %s",
		address.Logradouro, req.Number, address.Bairro, address.Localidade, address.UF)

	orderStatus := "APROVADO"
	var orderID int

	sqlStatement := `
	INSERT INTO orders (card_holder, card_brand, address_line, status)
	VALUES ($1, $2, $3, $4) RETURNING id`

	err = db.QueryRow(sqlStatement, req.CardHolder, cardBrand, fullAddress, orderStatus).Scan(&orderID)

	if err != nil {
		log.Printf("ERRO DB: Falha ao salvar pedido no banco: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(CheckoutResponse{Success: false, Message: "pagamento aprovado, mas erro interno: falha ao registrar o pedido (db)."})
		return
	}

	// 5. SUCESSO
	w.WriteHeader(http.StatusOK)
	response := CheckoutResponse{
		Success:     true,
		Message:     fmt.Sprintf("Checkout APROVADO! Pedido ID: %d", orderID),
		CardBrand:   cardBrand,
		AddressInfo: address,
	}
	json.NewEncoder(w).Encode(response)
}

// --- Função Principal e Setup de Rotas ---

func main() {
	// 1. Configurar Conexão com o Banco de Dados
	databaseURL := os.Getenv("DATABASE_URL")

	// Fallback para teste local se DATABASE_URL não estiver setada
	if databaseURL == "" {
		log.Println("Aviso: DATABASE_URL não configurada. Usando string literal para ambiente local.")
		// ATENÇÃO: Use a string de conexão *completa* do Koyeb aqui
		databaseURL = "user='checkout-adm' password=******* host=ep-rapid-frost-a4q9al3j.us-east-1.pg.koyeb.app dbname='koyebdb'"
	}

	if databaseURL != "" {
		var err error
		// sql.Open pode receber a string 'user=... host=...' ou a URI 'postgres://user:pass@host/db'
		db, err = sql.Open("postgres", databaseURL)
		if err != nil {
			// Não use log.Fatalf em produção se o DB for opcional.
			log.Fatalf("Erro ao abrir a conexão com o DB (Verifique o formato da string/URI): %v", err)
		}

		err = db.Ping()
		if err != nil {
			// Este é o erro de credenciais ou DNS que vimos no Koyeb.
			log.Fatalf("Erro ao conectar com o DB (Verifique o Host, Senha e DNS): %v", err)
		}
		log.Println("Conectado ao Banco de Dados com sucesso!")
	} else {
		// Se databaseURL estiver vazia (e o fallback for removido), o DB não será usado.
		log.Println("Aviso: Não há DATABASE_URL configurada. Pedidos não serão salvos no DB.")
	}

	// 2. Setup de Rotas
	staticDir := "./static"
	http.Handle("/", http.FileServer(http.Dir(staticDir)))

	http.HandleFunc("/api/lookup-cep", LookupCEPHandler)
	http.HandleFunc("/api/checkout", CheckoutHandler)

	const port = ":8080"
	log.Printf("Servidor iniciado em http://localhost%s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Erro ao iniciar o servidor: %v", err)
	}
}
