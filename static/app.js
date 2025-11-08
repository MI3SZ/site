const CPF_API = "/api/validate-cpf"
const CEP_API = "/api/validate-cep"
const CARD_API = "/api/validate-card"
const CHECKOUT_API = "/api/checkout"

const el = id=>document.getElementById(id)
const fmt = s=>s.replace(/\D/g,'')

function show(elm, txt){ elm.textContent = txt }

async function postJson(url, body){
  const res = await fetch(url,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)})
  return res
}

el("btnCheckCpf").addEventListener("click", async ()=>{
  const cpf = el("cpf").value.trim()
  if(!cpf){ show(el("cpfResult"),"Digite o CPF"); return }
  try{
    const r = await postJson(CPF_API,{cpf})
    const j = await r.json()
    if(r.ok) show(el("cpfResult"), `CPF válido`)
    else show(el("cpfResult"), `CPF inválido: ${j.error||"não autorizado"}`)
  }catch(e){
    show(el("cpfResult"), "Erro ao validar CPF")
  }
})

el("btnCheckCep").addEventListener("click", async ()=>{
  const cep = el("cep").value.trim()
  if(!cep){ show(el("cepResult"),"Digite o CEP"); return }
  try{
    const r = await postJson(CEP_API,{cep})
    const j = await r.json()
    if(r.ok){
      const info = j.info||{}
      el("street").value = info.street||""
      show(el("cepResult"), `${info.city||"-"} ${info.state||""}`)
    }else show(el("cepResult"), `CEP inválido`)
  }catch(e){
    show(el("cepResult"), "Erro ao consultar CEP")
  }
})

el("btnValidateCard").addEventListener("click", async ()=>{
  const card = el("cardNumber").value.trim()
  if(!card){ show(el("cardResult"),"Digite o número do cartão"); return }
  try{
    const r = await postJson(CARD_API,{card})
    const j = await r.json()
    const info = j.bin_info || j || {}
    const issuer = info.Issuer || info.bank?.name || info.Bank || "-"
    const status = info.Status || info.status || "-"
    const type = info.Type || info.type || "-"
    const scheme = info.Scheme || info.scheme || info.Brand || "-"
    const tier = info.CardTier || info.brand || info.level || info.Brand || "-"
    const valid = j.valid === true || j.valid === "true" || info.Luhn === true

    const container = el("cardResult")
    const statusClass = valid ? "status-ok" : "status-fail"
    container.innerHTML = `
      <div class="row-field"><div class="label">BIN</div><div class="value">${j.bin||"-"}</div></div>
      <div class="row-field"><div class="label">Issuer</div><div class="value">${escapeHtml(issuer)}</div></div>
      <div class="row-field"><div class="label">Status</div><div class="value"><span class="status-badge ${statusClass}">${escapeHtml(status)}</span></div></div>
      <div class="row-field"><div class="label">Tipo</div><div class="value">${escapeHtml(type)}</div></div>
      <div class="row-field"><div class="label">Bandeira</div><div class="value">${escapeHtml(scheme)}</div></div>
      <div class="row-field"><div class="label">Nível</div><div class="value">${escapeHtml(tier)}</div></div>
      <div class="small-muted">JSON bruto disponível no console do navegador.</div>
    `
    console.log("card raw response:", j)
  }catch(e){
    show(el("cardResult"), "Erro na validação do cartão")
  }
})

el("btnPay").addEventListener("click", async ()=>{
  const payload = {
    name: el("name").value.trim(),
    cpf: el("cpf").value.trim(),
    cep: el("cep").value.trim(),
    address: el("street").value.trim(),
    card: el("cardNumber").value.trim(),
    exp: el("exp").value.trim(),
    cvv: el("cvv").value.trim()
  }
  show(el("summary"), "Enviando...")
  try{
    const r = await postJson(CHECKOUT_API, payload)
    const j = await r.json()
    if(r.ok){
      show(el("summary"), `Pedido simulado: APROVADO\nID: ${j.info?.order_id || "-"}`)
    } else {
      show(el("summary"), `Pedido simulado: RECUSADO\n${j.error || "Motivo desconhecido"}`)
    }
  }catch(e){
    show(el("summary"), "Erro ao finalizar pedido")
  }
})

function escapeHtml(str){
  if(!str) return ""
  return String(str).replace(/[&<>"']/g, function(m){ return ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]) })
}
