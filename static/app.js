// Auto-validate app.js
const CPF_API = "/api/validate-cpf";
const CEP_API = "/api/validate-cep";
const CARD_API = "/api/validate-card";
const CHECKOUT_API = "/api/checkout";

const el = id => document.getElementById(id);
const fmt = s => s ? s.replace(/\D/g, "") : "";

function setInfo(targetEl, html) { targetEl.innerHTML = html; }
function badge(text, ok) {
  const cl = ok ? 'status-ok' : 'status-fail';
  return `<span class="status-badge ${cl}">${escapeHtml(text)}</span>`;
}
function escapeHtml(str){
  if(!str) return "";
  return String(str).replace(/[&<>"']/g, m=> ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]));
}

function makeFieldValidator(validateFn, delay = 700) {
  let timer = null;
  let controller = null;
  return function(value) {
    if (timer) clearTimeout(timer);
    if (controller) {
      try { controller.abort(); } catch(e) {}
      controller = null;
    }
    if (!value || String(value).trim() === "") return;
    validateFn.pending && validateFn.pending();
    timer = setTimeout(() => {
      controller = new AbortController();
      validateFn(value, controller.signal).finally(() => { controller = null; });
    }, delay);
  };
}

function luhnOk(card) {
  const digits = fmt(card);
  if (digits.length < 12) return false;
  let sum = 0, alt = false;
  for (let i = digits.length - 1; i >= 0; i--) {
    let d = parseInt(digits[i], 10);
    if (alt) { d *= 2; if (d > 9) d -= 9; }
    sum += d; alt = !alt;
  }
  return sum % 10 === 0;
}

// --- validation functions ---
async function validateCPFBackend(raw, signal) {
  const outEl = el("cpfResult");
  try {
    const res = await fetch(CPF_API, {
      method: "POST",
      headers: {"Content-Type":"application/json"},
      body: JSON.stringify({cpf: raw}),
      signal
    });
    const j = await res.json();
    if (res.ok && j.ok) setInfo(outEl, `<span style="color:var(--ok);font-weight:700">CPF válido</span>`);
    else setInfo(outEl, `<span style="color:var(--bad);font-weight:700">${escapeHtml(j.error||"CPF inválido")}</span>`);
  } catch (err) {
    if (err.name==='AbortError') return;
    setInfo(outEl, `<span style="color:var(--bad);">Erro ao validar CPF</span>`);
    console.error("CPF validate error:", err);
  }
}

async function validateCEPBackend(raw, signal) {
  const outEl = el("cepResult");
  try {
    const res = await fetch(CEP_API, {
      method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify({cep:raw}), signal
    });
    const j = await res.json();
    if(res.ok && j.ok){
      const info=j.info||{};
      el("street").value=info.street||el("street").value||"";
      setInfo(outEl, `${escapeHtml(info.city||"-")} ${escapeHtml(info.state||"")}`);
    } else {
      setInfo(outEl, `<span style="color:var(--bad);font-weight:700">${escapeHtml(j.error||"CEP inválido")}</span>`);
    }
  } catch(e) {
    if(e.name==='AbortError') return;
    setInfo(outEl, `<span style="color:var(--bad);">Erro ao consultar CEP</span>`);
    console.error("CEP validate error:", e);
  }
}

async function validateCardBackend(raw, signal) {
  const outEl = el("cardResult");
  const localOk = luhnOk(raw);
  let localHtml = `<div class="row-field"><div class="label">BIN</div><div class="value">—</div></div>`;
  localHtml += `<div class="row-field"><div class="label">Validação local (Luhn)</div><div class="value">${localOk?badge("OK",true):badge("INVÁLIDO",false)}</div></div>`;
  setInfo(outEl, localHtml);

  try {
    const res = await fetch(CARD_API, {
      method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify({card:raw}), signal
    });
    const j = await res.json();
    const info=j.bin_info||j||{};
    const issuer=info.Issuer||info.IssuerName||info.bank?.name||info.bank||"-";
    const status=info.Status||info.status||"-";
    const type=info.Type||info.type||"-";
    const scheme=info.Scheme||info.scheme||info.Brand||"-";
    const tier=info.CardTier||info.brand||info.level||info.Brand||"-";
    const valid=j.valid===true||info.Luhn===true||localOk===true;
    const statusClass = valid?'status-ok':'status-fail';
    const html=`
      <div class="row-field"><div class="label">BIN</div><div class="value">${escapeHtml(j.bin||"-")}</div></div>
      <div class="row-field"><div class="label">Issuer</div><div class="value">${escapeHtml(issuer)}</div></div>
      <div class="row-field"><div class="label">Status</div><div class="value"><span class="status-badge ${statusClass}">${escapeHtml(status)}</span></div></div>
      <div class="row-field"><div class="label">Tipo</div><div class="value">${escapeHtml(type)}</div></div>
      <div class="row-field"><div class="label">Bandeira</div><div class="value">${escapeHtml(scheme)}</div></div>
      <div class="row-field"><div class="label">Nível</div><div class="value">${escapeHtml(tier)}</div></div>
      <div class="small-muted">Resultado instantâneo após digitação.</div>
    `;
    setInfo(outEl, html);
  } catch(e) {
    if(e.name==='AbortError') return;
    setInfo(outEl, `<div class="row-field"><div class="label">Validação local</div><div class="value">${localOk?badge("OK",true):badge("INVÁLIDO",false)}</div></div><div style="color:#ef4444;margin-top:8px">Erro ao consultar BIN</div>`);
    console.error("Card validate error:", e);
  }
}

// --- attach debounced validators ---
const cpfField = el("cpf");
const cepField = el("cep");
const cardField = el("cardNumber");

const cpfValidator = makeFieldValidator(async (val,signal)=>{ setInfo(el("cpfResult"),"Validando..."); await validateCPFBackend(val,signal); },700);
const cepValidator = makeFieldValidator(async (val,signal)=>{ setInfo(el("cepResult"),"Buscando..."); await validateCEPBackend(val,signal); },700);
const cardValidator = makeFieldValidator(async (val,signal)=>{ setInfo(el("cardResult"),"Validando cartão..."); await validateCardBackend(val,signal); },600);

function attachAutoValidate(fieldEl, validator) {
  if(!fieldEl) return;
  fieldEl.addEventListener("focus", ()=>{});
  fieldEl.addEventListener("blur", ()=>{ validator(fieldEl.value); });
  fieldEl.addEventListener("input", ()=>{ validator(fieldEl.value); });
}

attachAutoValidate(cpfField, cpfValidator);
attachAutoValidate(cepField, cepValidator);
attachAutoValidate(cardField, cardValidator);

// finalize order
el("btnPay")?.addEventListener("click", async ()=>{
  const payload = {
    name: el("name").value.trim(),
    cpf: el("cpf").value.trim(),
    cep: el("cep").value.trim(),
    address: el("street").value.trim(),
    card: el("cardNumber").value.trim(),
    exp: el("exp").value.trim(),
    cvv: el("cvv").value.trim()
  };
  setInfo(el("summary"),"Enviando...");
  try{
    const r = await fetch(CHECKOUT_API,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
    const j = await r.json();
    if(r.ok) setInfo(el("summary"),`Pedido simulado: APROVADO\nID: ${j.info?.order_id||"-"}`);
    else setInfo(el("summary"),`Pedido simulado: RECUSADO\n${j.error||"Motivo desconhecido"}`);
  }catch(e){ setInfo(el("summary"),"Erro ao finalizar pedido"); }
});
