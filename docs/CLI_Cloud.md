 # UNIFIED CLOUD MANAGEMENT PLATFORM
## Documento de Requisitos de Produto (PRD)
### Plataforma Multi-Cloud CLI + Dashboard React/TypeScript

---

| Campo | Valor |
|---|---|
| **Versão** | 1.0.0 — Março 2026 |
| **Status** | Em Desenvolvimento |
| **Equipe** | Engenharia de Plataforma |
| **Duração** | 6 Semanas (Sprint Intensivo) |
| **Stack Principal** | Go (Cobra) · React/TS · AWS SDK · Docker |

---

## Sumário

1. [Resumo Executivo](#1-resumo-executivo)
2. [Arquitetura do Sistema](#2-arquitetura-do-sistema)
3. [Fluxo de Autenticação (IAM / SSO)](#3-fluxo-de-autenticação-iam--sso)
4. [REST API — Especificação de Endpoints](#4-rest-api--especificação-de-endpoints)
5. [Especificação de Comandos CLI](#5-especificação-de-comandos-cli)
6. [Plano de Desenvolvimento — 6 Semanas](#6-plano-de-desenvolvimento--6-semanas)
7. [Critérios de Aceite — Matriz Completa](#7-critérios-de-aceite--matriz-completa)
8. [Gerenciamento de Riscos](#8-gerenciamento-de-riscos)
9. [Estrutura de Diretórios do Projeto](#9-estrutura-de-diretórios-do-projeto)
10. [Métricas de Sucesso do Produto](#10-métricas-de-sucesso-do-produto)
11. [Aprovações e Assinaturas](#11-aprovações-e-assinaturas)

---

## 1. Resumo Executivo

A **Unified Cloud Management Platform (UCMP)** é uma ferramenta profissional de linha de comando construída em Go com o framework Cobra, integrada a um dashboard React/TypeScript, cujo objetivo é centralizar o gerenciamento de recursos AWS em múltiplas regiões a partir de uma interface unificada.

O sistema resolve três dores críticas de equipes de engenharia de plataforma:

1. A fragmentação do acesso multi-região via console AWS
2. A lentidão no provisionamento manual via UI
3. A ausência de uma visão consolidada de custos e recursos em tempo real

### 1.1 Objetivos de Negócio

- Reduzir em **70%** o tempo médio de provisionamento de instâncias EC2 em múltiplas regiões
- Eliminar a dependência do console AWS para operações de rotina de DevOps
- Fornecer observabilidade centralizada de recursos com atualização em tempo real
- Padronizar o fluxo de autenticação IAM/SSO para toda a equipe de engenharia
- Estabelecer uma base extensível para futura integração com GCP e Azure

### 1.2 Escopo do Produto

O produto é composto por quatro camadas principais:

- **CLI Tool (Go + Cobra):** interface de linha de comando para todas as operações
- **Auth Module:** fluxo de autenticação IAM Key-based e AWS SSO com suporte a MFA e STS Assume Role
- **REST API Bridge (Go + Gin):** camada de integração entre CLI e dashboard
- **Dashboard (React 18 + TypeScript):** interface visual com listagem, provisionamento e monitoramento

---

## 2. Arquitetura do Sistema

### 2.1 Visão Geral dos Componentes

A arquitetura segue o padrão de separação de responsabilidades, onde o CLI atua tanto como ferramenta standalone quanto como cliente da REST API.

```
Desenvolvedor / Operador
        ↓ Terminal / Browser
        
CLI (Go + Cobra)  ←──────→  REST API (Go + Gin)  ←──────→  Dashboard (React/TS)
        ↓                              ↓
AWS SDK Go v2  ────  Múltiplas Regiões  ────  Goroutines (Fan-Out Pattern)
```

### 2.2 Stack Tecnológica Detalhada

| Componente | Tecnologia | Responsabilidade | Prioridade |
|---|---|---|---|
| CLI Core | Go + Cobra Framework | Comandos provision, list, terminate, auth | Alta |
| Auth Module | IAM Keys + SSO + STS | Suporte a múltiplos perfis e MFA | **Crítica** |
| Concurrency Engine | Goroutines + WaitGroup + Channels | Fetch paralelo de N regiões AWS | Alta |
| REST API Bridge | Go net/http + Gin | Endpoints /instances, /regions, /metrics | Alta |
| Dashboard UI | React 18 + TypeScript + Vite | Visualização em tempo real dos recursos | Alta |
| State Management | Zustand + React Query | Cache e sincronização com a API | Média |
| Containerização | Docker + Docker Compose | Imagens separadas: CLI, API, Dashboard | Alta |
| CI/CD Pipeline | GitHub Actions | Build, test e push automáticos | Média |
| Config Management | Viper (Go) + ~/.ucmp/config.yaml | Múltiplos ambientes e perfis | Alta |
| Logging & Observ. | zerolog + Prometheus metrics | Logs estruturados JSON + /metrics endpoint | Média |

### 2.3 Padrões de Concorrência — Fan-Out/Fan-In com Goroutines

O módulo de fetch de instâncias utiliza o padrão **Fan-Out** para disparar requests simultâneos a todas as regiões AWS configuradas, coletando resultados via channels com timeout controlado:

```go
// Fan-Out: dispara goroutine por região
func FetchAllRegions(regions []string, creds aws.Credentials) []Instance {
    resultCh  := make(chan []Instance, len(regions))
    errorCh   := make(chan error, len(regions))
    var wg sync.WaitGroup

    for _, region := range regions {
        wg.Add(1)
        go func(r string) {
            defer wg.Done()
            instances, err := fetchRegion(r, creds)
            if err != nil { errorCh <- err; return }
            resultCh <- instances
        }(region)
    }

    wg.Wait()
    close(resultCh)
    close(errorCh)

    // Fan-In: coleta todos os resultados
    var all []Instance
    for inst := range resultCh {
        all = append(all, inst...)
    }
    return all
}
```

> **Nota de segurança:** o `go -race` detector deve passar sem alertas. Toda leitura/escrita compartilhada fora de channels deve usar `sync.Mutex`.

---

## 3. Fluxo de Autenticação (IAM / SSO)

### 3.1 Estratégia de Autenticação

O módulo de autenticação suporta dois provedores principais, com seleção por flag no CLI e armazenamento seguro de credenciais no diretório `~/.ucmp/`:

| Provedor | Mecanismo | Fluxo | Storage |
|---|---|---|---|
| **IAM Keys** | Access Key + Secret Key | Leitura de env vars ou arquivo ~/.ucmp/credentials.yaml → validação via GetCallerIdentity STS | ~/.ucmp/credentials.yaml (chmod 600) |
| **AWS SSO** | PKCE + Device Authorization | Abre browser para SSO URL → callback local em :9999 → troca code por token → STS AssumeRoleWithWebIdentity | Token cache em ~/.ucmp/sso-cache/ |
| **MFA (TOTP)** | TOTP via STS GetSessionToken | Prompt interativo para código MFA → STS GetSessionToken → credenciais temporárias (TTL configurável) | Session token em memória + refresh automático |
| **Assume Role** | STS AssumeRole | Recebe ARN da role → STS AssumeRole → retorna credenciais temporárias com duração máxima de 12h | Credenciais temporárias com auto-refresh |

### 3.2 Estrutura de Configuração (`~/.ucmp/config.yaml`)

```yaml
# ~/.ucmp/config.yaml
default_profile: production
output_format: table   # table | json | yaml

profiles:
  production:
    provider: sso
    sso_start_url: https://myorg.awsapps.com/start
    sso_account_id: "123456789012"
    sso_role_name: PowerUserAccess
    regions: [us-east-1, us-west-2, sa-east-1, eu-west-1]

  staging:
    provider: iam
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
    regions: [us-east-1, sa-east-1]

  dev:
    provider: iam
    access_key_id: ${DEV_AWS_ACCESS_KEY_ID}
    secret_access_key: ${DEV_AWS_SECRET_ACCESS_KEY}
    mfa_serial: arn:aws:iam::123456789012:mfa/dev-user
    regions: [us-east-1]
```

---

## 4. REST API — Especificação de Endpoints

### 4.1 Endpoints da API v1

Todos os endpoints requerem header `Authorization: Bearer <token>` exceto `/api/v1/health`. A API roda na porta `:8080` por padrão e suporta TLS com certificados configuráveis.

| Método | Endpoint | Descrição | Resposta |
|---|---|---|---|
| `GET` | /api/v1/instances | Lista todas as instâncias de todas as regiões | 200 + JSON array |
| `GET` | /api/v1/instances/:region | Lista instâncias de uma região específica | 200 + filtered array |
| `POST` | /api/v1/instances/provision | Provisiona nova instância EC2 | 201 + instance object |
| `DELETE` | /api/v1/instances/:id/terminate | Termina instância por ID | 200 + status message |
| `GET` | /api/v1/regions | Lista regiões AWS disponíveis e ativas | 200 + regions array |
| `GET` | /api/v1/metrics | Métricas de CPU, memória, rede por instância | 200 + metrics map |
| `GET` | /api/v1/auth/profiles | Lista perfis IAM/SSO configurados | 200 + profiles array |
| `POST` | /api/v1/auth/assume-role | Assume role via STS com retorno de credenciais | 200 + credentials |
| `GET` | /api/v1/health | Health check da API e conectividade AWS | 200 + status object |
| `GET` | /metrics | Endpoint Prometheus para scraping de métricas | 200 + text/plain |

### 4.2 Estrutura de Resposta Padrão

**Sucesso (2xx):**
```json
{
  "status": "success",
  "data": { "..." },
  "meta": {
    "total": 42,
    "regions": ["us-east-1", "sa-east-1"],
    "duration_ms": 234
  }
}
```

**Erro (4xx/5xx):**
```json
{
  "status": "error",
  "error": {
    "code": "REGION_UNAVAIL",
    "message": "Region unreachable",
    "region": "af-south-1",
    "retry_after": 30
  }
}
```

---

## 5. Especificação de Comandos CLI

### 5.1 Tabela de Comandos — `ucmp`

O binário `ucmp` é o ponto de entrada único. Todos os subcomandos seguem o padrão: `ucmp <grupo> <ação> [flags]`.

| Comando | Flags Principais | Descrição |
|---|---|---|
| `ucmp auth login` | `--provider [iam\|sso] --profile <n>` | Autentica e armazena credenciais |
| `ucmp auth list` | *(sem flags)* | Lista perfis configurados |
| `ucmp instances list` | `--region <r> --output [table\|json\|yaml] --all-regions` | Lista instâncias EC2 |
| `ucmp instances provision` | `--type <t> --region <r> --ami <id> --count <n> --tags <k=v>` | Provisiona instâncias EC2 |
| `ucmp instances terminate` | `--id <instance-id> --region <r> --force` | Termina instância |
| `ucmp instances metrics` | `--id <id> --region <r> --period <mins>` | Exibe métricas CloudWatch |
| `ucmp regions list` | `--active-only --output [table\|json]` | Lista regiões disponíveis |
| `ucmp api serve` | `--port <p> --bind <addr> --log-level <lvl>` | Inicia o servidor REST API |
| `ucmp config set` | `--profile <n> --region <r> --output <fmt>` | Configura preferências globais |
| `ucmp config view` | `--profile <n>` | Exibe configuração atual |

### 5.2 Exemplos de Uso em Fluxo Real

```bash
# 1. Autenticar com SSO
$ ucmp auth login --provider sso --profile production
✓ Browser aberto: https://myorg.awsapps.com/start
✓ Token obtido e armazenado em ~/.ucmp/sso-cache/

# 2. Listar todas as instâncias em paralelo
$ ucmp instances list --all-regions --output table
Fetching from 4 regions concurrently... ████████ 100% (3.2s)

ID            REGION      TYPE       STATE    LAUNCHED
i-0abc123     us-east-1   t3.medium  running  2026-03-10
i-0def456     sa-east-1   t3.small   running  2026-03-15
i-0ghi789     eu-west-1   t3.large   stopped  2026-02-28

# 3. Provisionar instâncias em São Paulo
$ ucmp instances provision --type t3.small --region sa-east-1 --count 2 --tags env=prod
✓ Provisioned: i-0jkl012 (sa-east-1) | i-0mno345 (sa-east-1)

# 4. Iniciar a API para o Dashboard
$ ucmp api serve --port 8080
✓ API listening on :8080 | Dashboard: http://localhost:3000

# 5. Terminar instância com confirmação
$ ucmp instances terminate --id i-0abc123 --region us-east-1
⚠ Confirm termination of i-0abc123? [y/N]: y
✓ Instance i-0abc123 terminated successfully
```

---

## 6. Plano de Desenvolvimento — 6 Semanas

Cada semana é tratada como um sprint com objetivos claros, entregáveis verificáveis e critérios de aceite mensuráveis. O plano prevê **buffer de 20%** para imprevistos técnicos.

---

### Semana 1 — Fundação & Autenticação

**Objetivo:** Estrutura completa do projeto Go + Cobra funcional, autenticação IAM e SSO operacional, primeiros testes unitários.

| Área | Tarefa / Entregável | Responsável | Critério de Aceite |
|---|---|---|---|
| Setup | Repositório Git, go.mod, estrutura de diretórios (cmd/, internal/, pkg/) | Lead Dev | `go build ./...` sem erros |
| Cobra | Comandos root, auth, instances, regions e api scaffoldados | Lead Dev | `ucmp --help` exibe todos os grupos |
| Config | Viper integrado, leitura de ~/.ucmp/config.yaml, suporte a env vars | Dev 1 | `ucmp config view` retorna valores corretos |
| IAM Auth | Fluxo Access Key + Secret Key com validação via GetCallerIdentity | Dev 1 | `ucmp auth login --provider iam` retorna identidade ARN |
| SSO Auth | Fluxo PKCE com servidor local :9999, troca de code por token SSO | Dev 2 | `ucmp auth login --provider sso` conclui autenticação |
| STS | AssumeRole e GetSessionToken com prompt MFA interativo | Dev 2 | Credenciais temporárias geradas com TTL correto |
| Testes | Testes unitários para auth package com mocks do AWS SDK | QA | `go test ./internal/auth/... >= 70%` cobertura |
| CI Setup | GitHub Actions: lint (golangci-lint) + test + build matrix (linux/mac/win) | DevOps | Pipeline verde no PR de abertura |

---

### Semana 2 — AWS SDK & Concorrência

**Objetivo:** Integração completa com AWS EC2 SDK, implementação do padrão Fan-Out/Fan-In com goroutines, comandos `instances` totalmente funcionais.

| Área | Tarefa / Entregável | Responsável | Critério de Aceite |
|---|---|---|---|
| EC2 Client | Wrapper do aws-sdk-go-v2/ec2 com retry automático e exponential backoff | Dev 1 | Chamadas retornam dados válidos de EC2 |
| Fan-Out | FetchAllRegions com goroutines + WaitGroup + buffered channels | Lead Dev | 10 regiões fetchadas em paralelo sem race conditions (`go -race` OK) |
| Fan-In | Collector com timeout de 30s e partial results em caso de falha regional | Lead Dev | Resultado retornado mesmo com 1-2 regiões indisponíveis |
| List Cmd | `ucmp instances list` com flags --region, --all-regions, --filter, --output | Dev 2 | Saída table/json/yaml formatada corretamente |
| Provision | `ucmp instances provision` com validação de AMI, tipo e tags | Dev 1 | EC2 criada e ID retornado em < 10s |
| Terminate | `ucmp instances terminate` com confirmação interativa e --force flag | Dev 2 | Instância terminada e status confirmado via polling |
| Metrics | `ucmp instances metrics` via CloudWatch com período configurável | Dev 1 | CPU/Network/Disk exibidos em formato tabular |
| Testes E2E | Testes de integração com LocalStack (EC2 mock local) | QA | Todos os comandos testados contra LocalStack sem erros |

---

### Semana 3 — REST API Bridge

**Objetivo:** API REST totalmente funcional em Go com Gin, middleware de autenticação, CORS, logging estruturado e endpoints documentados.

| Área | Tarefa / Entregável | Responsável | Critério de Aceite |
|---|---|---|---|
| Gin Setup | Servidor Gin com roteamento v1, middleware recovery e timeout | Dev 2 | `GET /api/v1/health` retorna 200 |
| Auth Middleware | Validação de Bearer token com suporte a JWT e credenciais do Viper | Dev 1 | Requisições sem token retornam 401 com mensagem clara |
| CORS | Middleware CORS configurável por ambiente (dev/staging/prod) | Dev 2 | Dashboard React conecta sem erros de CORS |
| Instances API | Handlers para GET/POST/DELETE de instâncias com validação de input | Lead Dev | Todos os endpoints retornam estrutura padrão |
| Regions API | Handler para listagem de regiões com cache de 5min (in-memory) | Dev 1 | `GET /api/v1/regions` retorna lista consistente |
| Logging | zerolog com log level configurável, request ID em cada request | Dev 1 | Logs estruturados JSON em todas as requisições |
| Rate Limiting | Middleware de rate limiting por IP (100 req/min) com headers padrão | Dev 2 | 429 retornado após exceder limite |
| Swagger | Geração automática de docs OpenAPI 3.0 via swaggo/swag | QA | `GET /swagger/index.html` exibe documentação |

---

### Semana 4 — Dashboard React/TypeScript

**Objetivo:** Dashboard React funcional com autenticação, listagem de instâncias em tempo real, ações de provisão/terminação e visualização de métricas.

| Área | Tarefa / Entregável | Responsável | Critério de Aceite |
|---|---|---|---|
| Vite Setup | React 18 + TypeScript + Vite + Tailwind CSS + shadcn/ui configurados | Frontend Dev | `npm run dev` inicia sem erros em localhost:3000 |
| Auth Screen | Tela de login com suporte a IAM Key e SSO (redirect flow) | Frontend Dev | Login persiste token em sessionStorage com expiração |
| API Client | Axios client tipado com interceptors para auth e error handling | Frontend Dev | Chamadas à API retornam tipos TypeScript corretos |
| Zustand Store | Estado global: instâncias, regiões, user, loading, erros | Frontend Dev | Estado persiste entre navegação sem re-fetch desnecessário |
| Instances Table | Tabela com filtro, sort, paginação, refresh automático a cada 30s | Frontend Dev | Tabela exibe dados reais da API com loading skeleton |
| Provision Form | Modal de provisão com validação Zod (tipo, região, AMI, tags) | Frontend Dev | Formulário valida e envia POST para a API corretamente |
| Metrics Chart | Gráfico Recharts de CPU/Network com janela de 1h configurável | Frontend Dev | Chart atualiza ao selecionar instância na tabela |
| React Query | useQuery / useMutation para cache, refetch e optimistic updates | Frontend Dev | Mutations refletem na UI sem reload da página |

---

### Semana 5 — Docker & Deployment

**Objetivo:** Containerização completa com multi-stage builds, Docker Compose orquestrando todos os serviços, e pipeline de CD configurado.

| Área | Tarefa / Entregável | Responsável | Critério de Aceite |
|---|---|---|---|
| Dockerfile CLI | Multi-stage: `builder golang:1.22-alpine → distroless/static`; binário estático | DevOps | Imagem final < 15MB; `ucmp --version` funciona no container |
| Dockerfile API | Multi-stage: `builder → alpine`; variáveis de ambiente para config | DevOps | Imagem final < 30MB; API responde em :8080 |
| Dockerfile UI | `nginx:alpine` servindo build estático do Vite; nginx.conf com proxy para API | DevOps | Dashboard acessível em :80 e proxya /api/ para a API |
| Docker Compose | Compose com services: api, dashboard, prometheus, grafana; healthchecks | DevOps | `docker-compose up` sobe todo o stack em < 2min |
| Prometheus | Expor métricas Go via /metrics + scraping configurado no prometheus.yml | DevOps | Grafana exibe dashboard de métricas da API |
| Secrets Mgmt | docker secrets para credenciais AWS; .env.example documentado | DevOps | Nenhuma credencial hardcoded nas imagens |
| CD Pipeline | GitHub Actions: build imagens → push DockerHub → tag semântico | DevOps | Push para `main` dispara build e push automáticos |
| Smoke Tests | Script de smoke test validando todos os endpoints após docker-compose up | QA | Script retorna 0 exit code no ambiente de staging |

Exemplo de `docker-compose.yml`:

```yaml
version: "3.9"
services:
  api:
    build: ./backend
    ports: ["8080:8080"]
    environment:
      - UCMP_PROFILE=${UCMP_PROFILE:-staging}
      - LOG_LEVEL=info
    secrets: [aws_credentials]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  dashboard:
    build: ./dashboard
    ports: ["3000:80"]
    depends_on:
      api:
        condition: service_healthy

  prometheus:
    image: prom/prometheus:latest
    volumes: ["./docker/prometheus.yml:/etc/prometheus/prometheus.yml"]
    ports: ["9090:9090"]

  grafana:
    image: grafana/grafana:latest
    ports: ["3001:3000"]
    depends_on: [prometheus]

secrets:
  aws_credentials:
    file: ./secrets/aws_credentials.json
```

---

### Semana 6 — QA, Segurança & Lançamento

**Objetivo:** Cobertura de testes >= 70%, auditoria de segurança, documentação completa e release v1.0.0 com artefatos publicados.

| Área | Tarefa / Entregável | Responsável | Critério de Aceite |
|---|---|---|---|
| Unit Tests Go | Cobertura >= 70% em auth, instances, api packages com table-driven tests | QA | `go test -coverprofile=cover.out >= 70%` |
| E2E Playwright | Testes ponta a ponta: auth → list → provision → terminate no browser | QA | `npx playwright test` retorna 0 falhas |
| Security Audit | govulncheck + nancy (CVE check) + Trivy nas imagens Docker | DevOps | Zero CVEs críticos nos relatórios de scan |
| Load Testing | k6: 50 usuários simultâneos por 5min contra /api/v1/instances | QA | p95 latência < 500ms; zero erros 5xx |
| Documentação | README completo, ARCHITECTURE.md, CONTRIBUTING.md, exemplos de uso | Lead Dev | Novo dev consegue rodar o projeto em < 15min seguindo o README |
| CHANGELOG | Semver changelog com todas as features, breaking changes e fixes | Lead Dev | CHANGELOG.md gerado e validado |
| Release v1.0.0 | Tag Git + release GitHub com binários pré-compilados (linux/mac/win/arm64) | Lead Dev | Binários disponíveis para download na página de releases |
| Retrospectiva | Métricas do sprint, dívida técnica identificada, roadmap v1.1 | Time | Documento de retrospectiva publicado na wiki interna |

---

## 7. Critérios de Aceite — Matriz Completa

Todos os critérios abaixo devem ser verificados antes da aprovação do release v1.0.0. Qualquer critério com status **reprovado bloqueia o merge para main**.

| ID | Feature | Critério de Aceite | Prazo |
|---|---|---|---|
| CLI-01 | Autenticação IAM | `ucmp auth login --provider iam` retorna sucesso e armazena token | Semana 1 |
| CLI-02 | Autenticação SSO | `ucmp auth login --provider sso` abre browser e conclui PKCE flow | Semana 1 |
| CLI-03 | Listar Instâncias | `ucmp instances list --all-regions` retorna JSON em < 5s para 10 regiões | Semana 2 |
| CLI-04 | Provisionar | `ucmp instances provision` cria instância EC2 e retorna ID e estado | Semana 2 |
| CLI-05 | Terminar | `ucmp instances terminate` derruba instância e confirma no CloudWatch | Semana 2 |
| CLI-06 | Concorrência | Fetch de 10 regiões simultaneamente sem race conditions (`go -race` OK) | Semana 2 |
| API-01 | Health Check | `GET /api/v1/health` retorna 200 com status AWS conectado | Semana 3 |
| API-02 | CRUD Instâncias | Todos os endpoints REST respondem corretamente com auth header | Semana 3 |
| API-03 | Métricas Prom. | `GET /metrics` expõe contadores válidos para Prometheus | Semana 4 |
| DASH-01 | Dashboard Conectado | Dashboard exibe instâncias de todas as regiões com refresh em 30s | Semana 4 |
| DASH-02 | Ações via UI | Botões Provision/Terminate no dashboard disparam chamadas à API | Semana 4 |
| DOC-01 | Docker Compose | `docker-compose up` inicia API + Dashboard sem erros em ambiente limpo | Semana 5 |
| DOC-02 | Multi-stage Build | Imagem final da API < 30MB; CLI binário estático < 15MB | Semana 5 |
| QA-01 | Test Coverage | Cobertura de testes unitários >= 70% em todos os módulos Go | Semana 6 |
| QA-02 | E2E Tests | Playwright testa fluxo completo: auth → list → provision → terminate | Semana 6 |

---

## 8. Gerenciamento de Riscos

| Risco | Impacto | Probabilidade | Mitigação |
|---|---|---|---|
| Rate Limiting AWS API | Alto | Médio | Exponential backoff + request throttling no SDK |
| Vazamento de Credenciais | **Crítico** | Baixo | Nunca logar credenciais; usar keychain/env vars |
| Concorrência (Race Conditions) | Alto | Médio | Uso de `sync.Mutex` e channels tipados |
| Divergência de Regiões | Médio | Alto | Normalização de schema por região antes de retornar |
| Overrun de Custo AWS | Alto | Médio | Alertas de billing + limites de instâncias por comando |
| Dashboard CORS em Prod | Médio | Alto | Configurar middleware CORS restritivo na API Go |
| Atrasos no Plano de 6 Sem. | Alto | Médio | Buffer de 20% no planejamento; features em MVP tiers |
| Compatibilidade Docker | Baixo | Alto | Testar em Linux/macOS/Windows antes de cada release |

---

## 9. Estrutura de Diretórios do Projeto

### Backend — Go (CLI + API)

```
ucmp/
├── cmd/
│   ├── root.go
│   ├── auth/
│   │   ├── login.go
│   │   └── list.go
│   ├── instances/
│   │   ├── list.go
│   │   ├── provision.go
│   │   └── terminate.go
│   ├── regions/
│   │   └── list.go
│   └── api/
│       └── serve.go
├── internal/
│   ├── auth/
│   │   ├── iam.go
│   │   ├── sso.go
│   │   ├── sts.go
│   │   └── auth_test.go
│   ├── aws/
│   │   ├── ec2.go
│   │   ├── cloudwatch.go
│   │   └── regions.go
│   ├── concurrency/
│   │   ├── fanout.go
│   │   └── fanout_test.go
│   └── api/
│       ├── server.go
│       ├── handlers/
│       │   ├── instances.go
│       │   ├── regions.go
│       │   ├── metrics.go
│       │   └── health.go
│       └── middleware/
│           ├── auth.go
│           ├── cors.go
│           ├── logger.go
│           └── ratelimit.go
├── pkg/
│   ├── config/
│   │   └── config.go
│   └── output/
│       └── formatter.go
├── docker/
│   ├── Dockerfile.cli
│   ├── Dockerfile.api
│   └── prometheus.yml
├── docker-compose.yml
├── go.mod
├── go.sum
└── Makefile
```

### Frontend — React/TypeScript

```
dashboard/
├── src/
│   ├── api/
│   │   ├── client.ts
│   │   └── endpoints.ts
│   ├── components/
│   │   ├── InstancesTable/
│   │   │   ├── InstancesTable.tsx
│   │   │   └── columns.tsx
│   │   ├── ProvisionModal/
│   │   │   ├── ProvisionModal.tsx
│   │   │   └── schema.ts
│   │   ├── MetricsChart/
│   │   │   └── MetricsChart.tsx
│   │   └── RegionSelector/
│   │       └── RegionSelector.tsx
│   ├── pages/
│   │   ├── Login.tsx
│   │   ├── Dashboard.tsx
│   │   └── Metrics.tsx
│   ├── store/
│   │   ├── instances.ts
│   │   └── auth.ts
│   ├── hooks/
│   │   ├── useInstances.ts
│   │   └── useMetrics.ts
│   └── types/
│       └── aws.ts
├── e2e/
│   └── dashboard.spec.ts
├── public/
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
└── package.json
```

---

## 10. Métricas de Sucesso do Produto

| Métrica | Baseline Atual | Meta v1.0 | Método de Medição |
|---|---|---|---|
| Tempo de provisionamento | ~8min (console AWS) | **< 45 segundos** | CLI timer + CloudWatch |
| Tempo de listagem multi-região | ~12min (manual) | **< 5 segundos** | `ucmp instances list --all-regions` benchmark |
| Cobertura de testes Go | 0% | **>= 70%** | `go test -cover` output |
| Latência API p95 | N/A | **< 500ms** | k6 load test report |
| Tamanho imagem Docker CLI | N/A | **< 15MB** | `docker image ls` |
| Onboarding de novo dev | ~2 dias | **< 15 minutos** | Tempo medido por desenvolvedor novo |
| Erros críticos em produção | N/A | **0 na semana 1** | Alertas Prometheus/Grafana |

---

## 11. Aprovações e Assinaturas

| Papel | Nome | Data | Status |
|---|---|---|---|
| Product Manager | _________________ | __/__/____ | ⏳ Pendente |
| Tech Lead / Arquiteto | _________________ | __/__/____ | ⏳ Pendente |
| Engineering Manager | _________________ | __/__/____ | ⏳ Pendente |
| Security Review | _________________ | __/__/____ | ⏳ Pendente |

---

*Unified Cloud Management Platform — PRD v1.0 — Documento Confidencial*
*Engenharia de Plataforma © 2026 — Todos os direitos reservados*
