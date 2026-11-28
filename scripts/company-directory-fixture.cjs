// Local-only fixture for the browser regression. Never configure production to use it.
const http = require("node:http");
http.createServer((req, res) => {
  const parts = req.url.split("?")[0].split("/").filter(Boolean);
  let data;
  if (parts[0] === "cnpj" && /^\d{14}$/.test(parts[1] || "")) {
    const active = parts[1] !== "00000000000191"; // Inactive fixture, not a claim about a real company.
    data = {
      cnpj: parts[1], razao_social: "Empresa QA LTDA", nome_fantasia: "Frota QA",
      situacao_cadastral: active ? 2 : 8, descricao_situacao_cadastral: active ? "ATIVA" : "BAIXADA",
      cep: "01001000", logradouro: "Praça da Sé", numero: "10", bairro: "Sé", municipio: "São Paulo", uf: "SP",
    };
  } else if (parts[0] === "cep" && /^\d{8}$/.test(parts[1] || "")) {
    data = parts[1] === "99999999" ? { erro: true } : parts[1] === "20040002"
      ? { cep: parts[1], logradouro: "Rua São José", bairro: "Centro", localidade: "Rio de Janeiro", uf: "RJ" }
      : { cep: parts[1], logradouro: "Praça da Sé", bairro: "Sé", localidade: "São Paulo", uf: "SP" };
  } else { res.writeHead(404); res.end(); return; }
  res.writeHead(200, { "Content-Type": "application/json" }); res.end(JSON.stringify(data));
}).listen(18098, "127.0.0.1", () => console.log("Local company-directory fixture listening on 18098"));
