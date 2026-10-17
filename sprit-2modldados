# GeevheitTire: Modelo de Dados (ER em Mermaid)

Convenções: chaves primárias UUID v7; `company_id` em todas as tabelas de negócio (RLS ativo); `created_at`, `updated_at`, `deleted_at` (soft delete) em todas, exceto onde indicado. Valores monetários em `numeric(12,2)`, sulco em `numeric(4,1)` (mm).

## 1. Tenant, usuários e SaaS

```mermaid
erDiagram
    companies ||--o{ users : tem
    companies ||--o{ branches : tem
    companies ||--|| subscriptions : assina
    plans ||--o{ subscriptions : define
    subscriptions ||--o{ invoices : gera
    companies ||--o{ api_keys : possui
    companies ||--o{ feature_flags : habilita
    companies ||--o{ audit_log : registra
    companies ||--o{ usage_counters : mede

    companies {
        uuid id PK
        text name
        text cnpj
        text logo_url
        jsonb brand_colors
        jsonb settings "limites de sulco, vidas max, regras"
        text status "trial|active|past_due|blocked"
        timestamptz trial_ends_at
    }
    users {
        uuid id PK
        uuid company_id FK
        uuid branch_id FK
        text name
        text email
        text password_hash
        text role "owner|manager|tire_tech|driver|inspector"
        bool totp_enabled
        timestamptz last_login_at
    }
    branches {
        uuid id PK
        uuid company_id FK
        text name
        text city
    }
    plans {
        uuid id PK
        text code "starter|pro|enterprise"
        int max_vehicles
        int max_users
        int max_branches
        jsonb features
        numeric price_month
    }
    subscriptions {
        uuid id PK
        uuid company_id FK
        uuid plan_id FK
        text status
        text asaas_customer_id
        text asaas_subscription_id
        date current_period_end
    }
    invoices {
        uuid id PK
        uuid subscription_id FK
        numeric amount
        text status
        text payment_method "pix|boleto|card"
        date due_date
        timestamptz paid_at
        text external_id "idempotencia webhook"
    }
    api_keys {
        uuid id PK
        uuid company_id FK
        text key_hash
        text name
        text scopes
        timestamptz last_used_at
        timestamptz revoked_at
    }
    feature_flags {
        uuid id PK
        uuid company_id FK
        text flag
        bool enabled
    }
    usage_counters {
        uuid id PK
        uuid company_id FK
        text metric "vehicles|users|ai_calls"
        date period
        int value
    }
    audit_log {
        uuid id PK
        uuid company_id FK
        uuid user_id FK
        text entity
        uuid entity_id
        text action
        jsonb before
        jsonb after
        timestamptz at
    }
```

## 2. Veículos, eixos e acoplamento

```mermaid
erDiagram
    companies ||--o{ vehicles : possui
    branches ||--o{ vehicles : aloca
    axle_templates ||--o{ axle_template_positions : contem
    axle_templates ||--o{ vehicles : configura
    vehicles ||--o{ vehicle_positions : tem
    axle_template_positions ||--o{ vehicle_positions : origina
    vehicles ||--o{ vehicle_couplings : cavalo
    vehicles ||--o{ vehicle_couplings : carreta
    vehicles ||--o{ odometer_readings : registra

    vehicles {
        uuid id PK
        uuid company_id FK
        uuid branch_id FK
        uuid axle_template_id FK
        text plate
        text chassis
        text renavam
        text kind "cavalo|carreta|truck|toco|tres_quartos|vuc|van|utilitario"
        text brand
        text model
        int year
        int current_km
        numeric engine_hours
        text status "ativo|inativo|manutencao"
        text cost_center
        text driver_name
        text origin "manual|imported|integrated"
        text external_id
    }
    axle_templates {
        uuid id PK
        uuid company_id FK "null = template global"
        text name
        text kind
        int axles
        int spares
        numeric recommended_psi
        numeric min_tread_mm
        text default_size
        numeric reference_cpk
        jsonb rotation_rules
    }
    axle_template_positions {
        uuid id PK
        uuid axle_template_id FK
        text code "1EE|1EI|2DI|STEP1"
        int axle_number
        text side "E|D|STEP"
        text inner_outer "E|I|NA"
        text role "direcional|tracao|livre|step"
        int sort_order
    }
    vehicle_positions {
        uuid id PK
        uuid vehicle_id FK
        uuid template_position_id FK
        text code
    }
    vehicle_couplings {
        uuid id PK
        uuid company_id FK
        uuid tractor_id FK
        uuid trailer_id FK
        timestamptz coupled_at
        timestamptz uncoupled_at
        int km_in
        int km_out
    }
    odometer_readings {
        uuid id PK
        uuid vehicle_id FK
        int km
        text source "manual|inspection|integration"
        timestamptz read_at
    }
```

## 3. Pneus, ciclo de vida, passagens, inspeções e custo

```mermaid
erDiagram
    companies ||--o{ tires : possui
    suppliers ||--o{ tires : fornece
    tires ||--o{ tire_lives : tem
    tires ||--o{ tire_assignments : passa
    vehicles ||--o{ tire_assignments : recebe
    tires ||--o{ tire_events : historico
    tires ||--o{ inspections : medido
    inspections ||--o{ inspection_photos : anexa
    tires ||--o{ maintenances : recebe
    retread_orders ||--o{ retread_order_items : contem
    tires ||--o{ retread_order_items : enviado
    suppliers ||--o{ retread_orders : executa
    tires ||--o{ alerts : gera
    stock_transfers ||--o{ stock_transfer_items : contem
    tires ||--o{ stock_transfer_items : move

    tires {
        uuid id PK
        uuid company_id FK
        uuid supplier_id FK
        uuid branch_id FK
        text code "unico por empresa"
        text qr_code
        text brand
        text model
        text size "295/80R22.5"
        text dot "semana/ano"
        text tire_type "direcional|tracao|livre"
        text load_speed_index
        numeric initial_tread_mm
        numeric recommended_psi
        text invoice_number
        date purchase_date
        numeric acquisition_cost
        text lot
        date warranty_until
        int warranty_km
        int estimated_life_km
        text state "EM_COMPRA|EM_ESTOQUE|INSTALADO|REMOVIDO|EM_RECAPAGEM|EM_GARANTIA|DESCARTADO"
        int current_life "1=nova, 2..4 recapada"
        uuid current_assignment_id FK
    }
    tire_lives {
        uuid id PK
        uuid tire_id FK
        int life_number
        numeric cost
        int km_total
        numeric tread_start
        numeric tread_end
        numeric cpk
        timestamptz started_at
        timestamptz ended_at
    }
    tire_assignments {
        uuid id PK
        uuid company_id FK
        uuid tire_id FK
        uuid vehicle_id FK
        uuid tractor_id FK "cavalo que puxou, se carreta"
        text position
        int life_number
        int km_in
        int km_out
        numeric tread_in
        numeric tread_out
        timestamptz installed_at
        timestamptz removed_at
        text reason
        uuid user_id FK
    }
    tire_events {
        uuid id PK
        uuid company_id FK
        uuid tire_id FK
        text type "compra|entrada|instalacao|rodizio|remocao|recapagem|manutencao|descarte|garantia"
        text from_state
        text to_state
        int vehicle_km
        text notes
        uuid user_id FK
        timestamptz at
        jsonb attachments
    }
    inspections {
        uuid id PK
        uuid company_id FK
        uuid tire_id FK
        uuid vehicle_id FK
        uuid user_id FK
        uuid client_id "idempotencia offline"
        numeric tread_mm
        numeric pressure_psi
        numeric temperature_c
        int vehicle_km
        text status "ok|alerta|critico"
        int level "1|2|3"
        text recommendation
        bool structural_damage
        timestamptz inspected_at
    }
    inspection_photos {
        uuid id PK
        uuid inspection_id FK
        text storage_key
        jsonb ai_analysis
    }
    maintenances {
        uuid id PK
        uuid company_id FK
        uuid tire_id FK
        text type "calibragem|reparo|balanceamento|alinhamento"
        numeric cost
        uuid user_id FK
        timestamptz done_at
    }
    suppliers {
        uuid id PK
        uuid company_id FK
        text name
        text kind "fornecedor|recapadora"
        text cnpj
        numeric rating
    }
    retread_orders {
        uuid id PK
        uuid company_id FK
        uuid supplier_id FK
        text status "enviado|parcial|concluido"
        date sent_at
        date returned_at
        numeric total_cost
    }
    retread_order_items {
        uuid id PK
        uuid retread_order_id FK
        uuid tire_id FK
        text band_type
        text result "aprovado|reprovado_carcaca"
        numeric cost
        date warranty_until
    }
    stock_transfers {
        uuid id PK
        uuid company_id FK
        uuid from_branch_id FK
        uuid to_branch_id FK
        text status
        timestamptz at
    }
    stock_transfer_items {
        uuid id PK
        uuid stock_transfer_id FK
        uuid tire_id FK
    }
    alerts {
        uuid id PK
        uuid company_id FK
        uuid tire_id FK
        uuid vehicle_id FK
        text type "sulco|pressao|temperatura|dot|vencimento|anomalia"
        text severity "info|alerta|critico"
        text message
        bool read
        timestamptz created_at
    }
```

## 4. Importação, integrações e IA

```mermaid
erDiagram
    companies ||--o{ import_jobs : executa
    companies ||--o{ integrations : configura
    integrations ||--o{ sync_logs : registra
    companies ||--o{ ai_insights : recebe
    companies ||--o{ ai_usage : consome
    companies ||--o{ webhook_endpoints : configura
    webhook_endpoints ||--o{ webhook_deliveries : envia

    import_jobs {
        uuid id PK
        uuid company_id FK
        text entity "vehicles|tires"
        text file_key
        jsonb column_mapping
        text mode "create|create_update"
        text status "preview|processing|done|failed"
        int total_rows
        int ok_rows
        int error_rows
        text errors_file_key
    }
    integrations {
        uuid id PK
        uuid company_id FK
        text provider "generic_rest|erp_x|tracker_y"
        bytea credentials_encrypted
        jsonb field_mapping
        text conflict_rule "manual_wins|integration_wins"
        timestamptz last_sync_at
        text status
    }
    sync_logs {
        uuid id PK
        uuid integration_id FK
        timestamptz started_at
        int created
        int updated
        int errors
        text details
    }
    ai_insights {
        uuid id PK
        uuid company_id FK
        text type "previsao|irregular|foto|rodizio|anomalia|compra|resumo"
        text entity "tire|vehicle|company"
        uuid entity_id
        text severity
        text explanation
        text suggested_action
        numeric confidence
        text status "novo|aceito|ignorado|ajustado"
        text prompt_version
        timestamptz created_at
    }
    ai_usage {
        uuid id PK
        uuid company_id FK
        text feature
        int tokens_in
        int tokens_out
        numeric est_cost
        timestamptz at
    }
    webhook_endpoints {
        uuid id PK
        uuid company_id FK
        text url
        text secret
        text events
        bool active
    }
    webhook_deliveries {
        uuid id PK
        uuid endpoint_id FK
        text event
        int attempts
        int last_status
        timestamptz delivered_at
    }
```

## 5. Regras de integridade importantes

1. **Uma passagem aberta por pneu:** índice único parcial em `tire_assignments(tire_id) WHERE removed_at IS NULL`.
2. **Uma posição ocupada por vez:** índice único parcial em `tire_assignments(vehicle_id, position) WHERE removed_at IS NULL`.
3. **Código do pneu único por empresa:** `UNIQUE (company_id, code) WHERE deleted_at IS NULL`.
4. **Placa única por empresa:** `UNIQUE (company_id, plate) WHERE deleted_at IS NULL`.
5. **Odômetro nunca diminui:** validação na aplicação e `CHECK` via trigger em `odometer_readings`; leitura menor gera alerta de inconsistência, não é gravada como km atual.
6. **Transições de estado:** a aplicação valida a tabela de transições; `tire_events` guarda `from_state` e `to_state` para auditoria.
7. **Idempotência offline:** `UNIQUE (company_id, client_id)` em `inspections`.
8. **RLS:** toda tabela com `company_id` usa `USING (company_id = current_setting('app.company_id')::uuid)`; teste automático tenta ler dados de outra empresa e deve falhar.
9. **CPK:** `(acquisition_cost + custo de recapagens + manutenções) / km_total`, calculado por `tire_lives` e acumulado por pneu (job noturno e sob demanda).
10. **Índices recomendados:** `inspections(tire_id, inspected_at desc)`, `tire_events(tire_id, at desc)`, `alerts(company_id, read, created_at desc)`, `tires(company_id, state)`.

## 6. Como pedir as migrations à IA

Cole este arquivo e use: "Gere as migrations SQL (golang-migrate) deste modelo em ordem de dependência, com índices, constraints, RLS e seeds dos templates de eixo. Divida em arquivos por módulo e rode um teste de isolamento entre empresas."