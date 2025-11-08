const CPF_API = "/api/validate-cpf";
const CEP_API = "/api/validate-cep";
const CARD_API = "/api/validate-card";
const CHECKOUT_API = "/api/checkout";

const el = id => document.getElementById(id);

let cpfOk=false, cepOk=false, cardOk=false;

function setInfo(targetEl, html){ targetEl.innerHTML = html; }
function badge(text, ok){ return `<span class="status-badge ${ok?'status-ok':'status-fail'}">${text}</span>`; }

async function validateCPF(){ 
  const val=el("cpf").value.trim();
  if(!val) return (cpfOk=false);
  try{
    const res=await fetch(CPF_API,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({cpf:val})});
    const j=await res.json();
    if(res.ok && j.ok){ setInfo(el("cpfResult"),badge("CPF válido",true)); cpfOk=true; }
    else{ setInfo(el("cpfResult"),badge(j.error||"CPF inválido",false)); cpfOk=false; }
  }catch(e){ setInfo(el("cpfResult"),badge("Erro",false)); cpfOk=false; }
  togglePay();
}

async function validateCEP(){
  const val=el("cep").value.trim();
  if(!val) return (cepOk=false);
  try{
    const res=await fetch(CEP_API,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({cep:val})});
    const j=await res.json();
    if(res.ok && j.ok){
      el("street").value=j.info.street||"";
      setInfo(el("cepResult"),`${j.info.city||"-"} ${j.info.state||"-"}`);
      cepOk=true;
    }else{ setInfo(el("cepResult"),badge(j.error||"CEP inválido",false)); cepOk=false; }
  }catch(e){ setInfo(el("cepResult"),badge("Erro",false)); cepOk=false; }
  togglePay();
}

async function validateCard(){
  const val=el("cardNumber").value.trim();
  if(!val) return (cardOk=false);
  try{
    const res=await fetch(CARD_API,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({card:val})});
    const j=await res.json();
    if(j.valid){ setInfo(el("cardResult"),badge("Cartão válido",true)); cardOk=true; }
    else{ setInfo(el("cardResult"),badge("Cartão inválido",false)); cardOk=false; }
  }catch(e){ setInfo(el("cardResult"),badge("Erro",false)); cardOk=false; }
  togglePay();
}

function togglePay(){ el("btnPay").disabled = !(cpfOk && cepOk && cardOk); }

// attach events
el("cpf")?.addEventListener("input",validateCPF);
el("cep")?.addEventListener("input",validateCEP);
el("cardNumber")?.addEventListener("input",validateCard);

el("btnPay")?.addEventListener("click", async()=>{
  const payload={
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
    const r=await fetch(CHECKOUT_API,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
    const j=await r.json();
    if(r.ok && j.ok) setInfo(el("summary"),`Pedido aprovado!\nID: ${j.info.order_id}`);
    else setInfo(el("summary"),`Pedido recusado\n${j.error||"Erro desconhecido"}`);
  }catch(e){ setInfo(el("summary"),"Erro ao finalizar pedido"); }
});
