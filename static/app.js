// --- Constantes e Seletores ---
const el = id => document.getElementById(id);
const fmt = s => s ? s.replace(/\D/g, "") : "";
const CEP_LOOKUP_API = "/api/lookup-cep";
const CHECKOUT_API = "/api/checkout";

// --- Helpers ---
const setInfo = (targetEl, html, status) => {
  targetEl.innerHTML = html;
  targetEl.className = "info"; 
  if (status === 'ok') targetEl.classList.add('status-ok-text');
  if (status === 'fail') targetEl.classList.add('status-fail-text');
};

const clearAddressFields = () => {
    el("rua").value = "";
    el("bairro").value = "";
    el("cidade").value = "";
};

// --- Estado do Formulário ---
const formState = {
  name: false,
  cpf: false,
  cep: false,
  number: false, 
  card: false,
  exp: false,
  cvv: false,
};

// --- Funções de Máscara ---
const maskCPF = v => v.replace(/\D/g, "").replace(/(\d{3})(\d)/, "$1.$2").replace(/(\d{3})(\d)/, "$1.$2").replace(/(\d{3})(\d{1,2})/, "$1-$2").replace(/(-\d{2})\d+?$/, "$1");
const maskCEP = v => v.replace(/\D/g, "").replace(/^(\d{5})(\d)/, "$1-$2").replace(/(-\d{3})\d+?$/, "$1");
const maskCard = v => v.replace(/\D/g, "").replace(/(\d{4})/g, "$1 ").trim();
const maskExp = v => v.replace(/\D/g, "").replace(/(\d{2})(\d)/, "$1/$2").replace(/(\/\d{2})\d+?$/, "$1");

// --- Funções de Validação LOCAL (Corrigidas) ---
const localValidators = {
  // Validação de CPF completa (algoritmo)
  cpf: (cpf) => {
    cpf = fmt(cpf);
    if (cpf.length !== 11 || /^(\d)\1+$/.test(cpf)) return false; 
    let sum = 0, r;
    for (let i = 1; i <= 9; i++) sum += parseInt(cpf.substring(i - 1, i)) * (11 - i);
    r = (sum * 10) % 11;
    if (r === 10 || r === 11) r = 0;
    if (r !== parseInt(cpf.substring(9, 10))) return false;
    sum = 0;
    for (let i = 1; i <= 10; i++) sum += parseInt(cpf.substring(i - 1, i)) * (12 - i);
    r = (sum * 10) % 11;
    if (r === 10 || r === 11) r = 0;
    return r === parseInt(cpf.substring(10, 11));
  },
  // Validação de Cartão completa (algoritmo Luhn)
  card: (card) => {
    const digits = fmt(card);
    if (digits.length < 13 || digits.length > 19) return false;
    let sum = 0, alt = false;
    for (let i = digits.length - 1; i >= 0; i--) {
      let d = parseInt(digits[i], 10);
      if (alt) { d *= 2; if (d > 9) d -= 9; }
      sum += d; alt = !alt;
    }
    return sum % 10 === 0;
  },
  exp: (exp) => {
    const [month, year] = exp.split('/');
    if (!month || !year || year.length !== 2) return false;
    const expMonth = parseInt(month, 10);
    const expYear = parseInt(`20${year}`, 10);
    if (expMonth < 1 || expMonth > 12) return false;
    const now = new Date();
    const currentYear = now.getFullYear();
    const currentMonth = now.getMonth() + 1;
    return expYear > currentYear || (expYear === currentYear && expMonth >= currentMonth);
  },
};

const getCardBrand = (digits) => {
  if (digits.startsWith("4")) return "Visa";
  if (/^(5[1-5])/.test(digits)) return "Mastercard";
  if (/^(50|56|57|58)/.test(digits)) return "Elo";
  if (/^(34|37)/.test(digits)) return "American Express";
  if (/^(6)/.test(digits)) return "Discover";
  return "Desconhecida";
};

// --- Função para Checar o Formulário e Habilitar o Botão ---
const checkFormValidity = () => {
  const allValid = Object.values(formState).every(v => v === true);
  el("btnPay").disabled = !allValid;
};

// --- Validador de CEP Assíncrono ---
const makeDebouncedValidator = (validateFn, delay = 600) => {
  let timer = null;
  let controller = null;
  return (value) => {
    if (timer) clearTimeout(timer);
    if (controller) controller.abort();
    if (!value || String(value).trim() === "") {
        clearAddressFields();
        return;
    }
    
    timer = setTimeout(() => {
      controller = new AbortController();
      validateFn(value, controller.signal).finally(() => { controller = null; });
    }, delay);
  };
};

const validateCEP_API = async (cep, signal) => {
  const cleanCEP = fmt(cep);
  clearAddressFields();
  
  if (cleanCEP.length !== 8) {
    setInfo(el("cepResult"), "CEP deve ter 8 dígitos", "fail");
    formState.cep = false;
    checkFormValidity();
    return;
  }
  
  setInfo(el("cepResult"), "Buscando CEP...", "");
  
  try {
    const res = await fetch(CEP_LOOKUP_API, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ cep: cleanCEP }),
      signal
    });
    
    const data = await res.json();
    
    if (!res.ok) {
      setInfo(el("cepResult"), data.error || "Erro ao buscar CEP", "fail");
      formState.cep = false;
    } else {
      // PREENCHE OS CAMPOS SEPARADOS
      el("rua").value = data.logradouro;
      el("bairro").value = data.bairro;
      el("cidade").value = `${data.localidade} - ${data.uf}`;
      
      setInfo(el("cepResult"), "CEP encontrado e endereço preenchido.", "ok");
      formState.cep = true;
    }
  } catch (err) {
    if (err.name !== 'AbortError') {
      setInfo(el("cepResult"), "Erro na conexão", "fail");
      formState.cep = false;
    }
  }
  checkFormValidity();
};

const debouncedCEPValidator = makeDebouncedValidator(validateCEP_API);

// --- Listeners de Validação e Máscara ---
const setupInputListeners = () => {
  el("name").addEventListener("input", e => {
    formState.name = e.target.value.trim().length > 2;
    checkFormValidity();
  });

  el("cpf").addEventListener("input", e => {
    e.target.value = maskCPF(e.target.value);
    const isValid = localValidators.cpf(e.target.value);
    
    formState.cpf = isValid; 
    
    if (fmt(e.target.value).length === 11) {
        setInfo(el("cpfResult"), isValid ? "CPF válido" : "CPF inválido", isValid ? 'ok' : 'fail');
    } else {
        setInfo(el("cpfResult"), "Aguardando 11 dígitos", "");
    }
    checkFormValidity();
  });

  el("cep").addEventListener("input", e => {
    e.target.value = maskCEP(e.target.value);
    debouncedCEPValidator(e.target.value);
  });
  
  el("number").addEventListener("input", e => {
    formState.number = e.target.value.trim().length > 0;
    checkFormValidity();
  });

  el("cardNumber").addEventListener("input", e => {
    e.target.value = maskCard(e.target.value);
    const isValidLength = fmt(e.target.value).length >= 13;
    const isLuhnValid = localValidators.card(e.target.value);
    const brand = getCardBrand(fmt(e.target.value));
    
    formState.card = isValidLength && isLuhnValid;
    
    let infoText = `Bandeira: ${brand}`;
    if (isValidLength) { 
        infoText += isLuhnValid ? " - Cartão válido" : " - Cartão inválido";
    } else {
        infoText += " - Aguardando 13+ dígitos";
    }
    
    setInfo(el("cardResult"), infoText, isLuhnValid && isValidLength ? 'ok' : 'fail');
    checkFormValidity();
  });

  el("exp").addEventListener("input", e => {
    e.target.value = maskExp(e.target.value);
    formState.exp = localValidators.exp(e.target.value);
    checkFormValidity();
  });
  
  el("cvv").addEventListener("input", e => {
    formState.cvv = fmt(e.target.value).length >= 3;
    checkFormValidity();
  });
};

// --- Listener do Botão de Pagar ---
const setupPayButtonListener = () => {
  el("btnPay").addEventListener("click", async () => {
    const summaryEl = el("summary");
    summaryEl.style.color = "inherit";
    setInfo(summaryEl, "Processando pagamento...", "");
    el("btnPay").disabled = true;

    const payload = {
      card_holder: el("name").value.trim(),
      card_number: fmt(el("cardNumber").value),
      expiration_date: el("exp").value.trim(),
      cvv: el("cvv").value.trim(),
      cep: fmt(el("cep").value),
      number: el("number").value.trim(), 
    };

    try {
      const res = await fetch(CHECKOUT_API, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      const data = await res.json();

      if (res.ok && data.success) {
        summaryEl.style.color = "var(--ok)";
        setInfo(summaryEl, 
          `✅ Pagamento APROVADO com sucesso!\n\n` +
          `${data.message}\n` + 
          `Bandeira Utilizada: ${data.card_brand}\n\n` +
          `📦 Endereço Salvo:\n` +
          `${data.address_info.logradouro}, ${payload.number} - ${data.address_info.bairro}\n` +
          `${data.address_info.localidade} - ${data.address_info.uf} (CEP: ${data.address_info.cep})`
        );
      } else {
        summaryEl.style.color = "var(--bad)";
        setInfo(summaryEl, `❌ Pagamento RECUSADO:\n${data.message}`);
        el("btnPay").disabled = false;
      }

    } catch (err) {
      summaryEl.style.color = "var(--bad)";
      setInfo(summaryEl, "🚨 Erro de conexão. Não foi possível finalizar o pedido.");
      el("btnPay").disabled = false;
    }
  });
};

// --- Inicialização ---
document.addEventListener("DOMContentLoaded", () => {
  setupInputListeners();
  setupPayButtonListener();
  checkFormValidity();
});