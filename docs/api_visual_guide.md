# OPM Stats API: Visual Guide

This guide provides a visual overview of how the OPM Stats API functions, from data ingestion to statistics retrieval and automated documentation.

## 1. High-Level Architecture

The OPM Stats API acts as the central hub for the entire ecosystem, connecting real-time game events with persistent storage and the web frontend.

```mermaid
graph TD
    subgraph "Game Environment"
        GS1["Game Server A"]
        GS2["Game Server B"]
    end

    subgraph "OPM Stats API (Go)"
        H["Handlers (REST Endpoints)"]
        L["Service Logic"]
        M["Models (Typed Data)"]
    end

    subgraph "Storage Layer"
        CH[("ClickHouse (Telemetry)")]
        PG[("PostgreSQL (Relational)")]
        RD[("Redis (Real-time/Cache)")]
    end

    subgraph "Frontend"
        SMF["SMF Web Portal (PHP)"]
        SC["Scalar (OpenAPI Docs)"]
    end

    %% Ingestion Flow
    GS1 -->|JSON Events| H
    GS2 -->|JSON Events| H
    H --> L
    L -->|Bulk Insert| CH
    L -->|Metadata| PG

    %% Query Flow
    SMF -->|API Request| H
    H --> L
    L -->|Aggregate| CH
    L -->|Fetch| PG
    L -->|Stats| RD
    L --> M
    M -->|JSON Response| H
    H -->|Data| SMF

    %% Docs Flow
    M -.->|Swag Annotations| SC
```

---

## 2. Data Ingestion Flow (Sequenced)

When a player is killed or an objective is captured, the game engine sends a burst of data.

```mermaid
sequenceDiagram
    participant Game as OpenMOHAA Engine
    participant API as API Handler
    participant Logic as Ingestion Logic
    participant CH as ClickHouse
    participant Redis as Redis (Pulse)

    Game->>API: POST /api/v1/ingest/event (Raw JSON)
    API->>API: Validate Auth Token
    API->>Logic: Process Raw Event
    
    Logic->>CH: Insert into raw_events table
    Logic->>Redis: Update Online Status / Pulse
    
    Note over Logic, CH: Events are stored as raw JSON<br/>for maximum flexibility.
    
    API-->>Game: 202 Accepted
```

---

## 3. Statistics Retrieval & Aggregation

Requests for complex stats (like the War Room or Leaderboards) involve cross-database aggregation.

```mermaid
sequenceDiagram
    participant UI as Web Frontend (SMF)
    participant API as API Handler
    participant Logic as Advanced Stats Service
    participant CH as ClickHouse
    participant PG as PostgreSQL

    UI->>API: GET /api/v1/stats/player/{guid}/war-room
    API->>Logic: GetDeepStats(guid)
    
    par Query Telemetry
        Logic->>CH: Aggregated Combat Metrics (KDR, Accuracy)
    and Query Metadata
        Logic->>PG: Player Nicknames, Rank, Clan Info
    end
    
    Logic->>Logic: Calculate Signatures (e.g. "Sniper", "Rusher")
    Logic->>API: Return models.DeepStats{}
    API-->>UI: JSON Response
```

---

## 4. Documentation & Schema System

We use a "Code-First" approach to documentation ensures the spec is always in sync with the types.

1.  **Definitions**: Structs are defined in `internal/models`.
2.  **Annotations**: Handlers in `internal/handlers` are decorated with `// @Summary`, `// @Success`, etc.
3.  **Generation**: Running `./generate_docs.sh` invokes `swag init`.
4.  **Presentation**: The generated `web/static/swagger.yaml` is consumed by **Scalar** for an interactive UI.

---

## 5. Key Components Reference

| Component | Responsibility |
| :--- | :--- |
| **Handlers** | Route matching, request validation, JSON encoding/decoding. |
| **Logic** | Business rules, complex SQL/ClickHouse queries, achievement calculations. |
| **Models** | Source of truth for API contracts. Shared between logic and handlers. |
| **Interfaces** | Decouples components for easier testing and modularity. |
