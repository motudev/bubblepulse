# BubblePulse: Project Goals & Vision

## 1. Executive Summary
The modern workplace is plagued by meeting fatigue and information fragmentation. Standups, originally designed to unblock teams, have devolved into status-reporting ceremonies that disrupt deep work and alienate cross-functional departments. 

**BubblePulse** is an open-source, self-hosted asynchronous check-in platform designed to eliminate unnecessary meetings without sacrificing team alignment or cohesion. By capturing frictionless daily updates via familiar chat interfaces (like Slack) and translating them into a visual dependency graph, BubblePulse empowers teams to identify blockers instantly, keep stakeholders informed automatically, and protect their focus time.

---

## 2. The Core Problems We Are Solving

1.  **Flow State Disruption:** Knowledge workers (developers, designers, writers) require long stretches of uninterrupted time. A 15-minute sync in the middle of the morning destroys momentum.
2.  **Information Asymmetry:** Decisions happen in private DMs or hallway chats. When dependencies shift, the people who need to know are often the last to find out.
3.  **The "Hairball" of Cross-Team Dependencies:** In growing organizations, it becomes nearly impossible to track who is blocked by whom across different departments.
4.  **Stakeholder Anxiety:** Managers and Executives often mandate meetings because they lack visibility into specific, critical initiatives.
5.  **Forced "Agile" on Non-Dev Teams:** Traditional check-in tools force rigid, software-centric paradigms (Jira tickets, sprints) onto teams like Marketing or HR that operate differently.

---

## 3. Product Vision & Philosophy

### 3.1. Frictionless Input
Users should not have to log into a separate portal to give an update. BubblePulse meets them where they already work (Slack, Teams, Matrix). The input mechanism must be as simple as sending a direct message or filling out a 3-field modal: *Focus*, *Friction*, and *Energy*.

### 3.2. Visual Actionability (The Bubble Map)
Instead of linear text feeds that get buried, BubblePulse turns daily updates into a live, interactive node graph. 
*   **Nodes (Bubbles):** Represent individuals, sized or colored by their current capacity or energy level.
*   **Edges (Lines):** Represent dependencies or blockers. A red line instantly shows that the UI Designer is waiting on the API Developer.

### 3.3. Smart Information Routing
Information should push to those who need it, rather than requiring them to pull it. 
*   **Topic Subscriptions:** POs, CEOs, or Lead Engineers can subscribe to specific semantic topics (e.g., "Stripe Migration", "Acme Corp Deal"). When a team member mentions a related keyword or theme, the subscriber is silently pinged, eliminating the need to crash a daily standup.

### 3.4. Data Sovereignty & Privacy
Team communication and bottleneck data is highly sensitive. By being **open-source and self-hostable**, BubblePulse guarantees that enterprise operational data, team sentiment, and strategic blockers never leave the company's internal servers.

### 3.5. Protecting the "Team Feel"
Async tools can feel isolating. BubblePulse counters this by:
*   Integrating "watercooler" prompts to maintain human connection.
*   Tracking passive sentiment (energy levels) to give HR and Leadership early warnings of burnout without exposing individual vulnerabilities.
*   Encouraging quick, targeted 5-minute coffee chats over 30-minute status roundtables.

---

## 4. Target Audience
BubblePulse is built for **any flexible or remote team**, not just software engineering.
*   **Development / DevOps:** Tracking PR reviews, API dependencies, and server migrations.
*   **Marketing / Creative:** Aligning copywriters, designers, and ad buyers on campaign assets.
*   **Sales / Field Ops:** Keeping field reps connected to HQ and unblocking compliance or product questions.
*   **Leadership / HR:** Passively monitoring team health and cross-departmental bottlenecks without micromanaging.

---

## 5. Technical Architecture & Stack

To ensure BubblePulse remains lightweight, easily deployable, and highly performant, we adhere to the following stack:
*   **Backend:** Go (Golang) - chosen for its concurrency model, fast compilation, and ability to distribute as a single, zero-dependency binary.
*   **Database:** PostgreSQL - highly reliable relational database handling users, updates, and recursive CTEs for edge connections.
*   **Frontend:** Vue.js 3 (Composition API) with Vite - providing a reactive, lightweight dashboard. Graphing capabilities powered by libraries like D3.js or Cytoscape.js.
*   **Integrations:** Slack API (Webhook/Event-driven) as the primary initial communication layer.

---

## 6. Strategic Roadmap

### Phase 1: The Core Loop (MVP)
*   **Goal:** Establish the foundation of async check-ins and visual unblocking.
*   **Features:** Slack integration (slash commands and modal inputs), Go REST API, basic Vue dashboard with the "Bubble Map" (nodes and edges), and PostgreSQL schema deployment.

### Phase 2: Smart Routing & Subscriptions
*   **Goal:** Solve stakeholder anxiety and automate dependency mapping.
*   **Features:** PO/CEO topic subscriptions via exact keyword matching and lightweight local LLM semantic analysis. Automated edge creation based on natural language parsing of user updates.

### Phase 3: Team Health & Analytics
*   **Goal:** Provide long-term value through organizational insights.
*   **Features:** Async retro check-ins (anonymized sentiment tracking), Calendar API integrations (automatic capacity updates), and long-term Information Flow Analysis (identifying chronic bottlenecks between departments).

---

## 7. Success Metrics
How do we know BubblePulse is working?
1.  **Reduced Meeting Load:** Average hours spent in internal status meetings drops by >50%.
2.  **High Adoption Rate:** >85% daily completion rate due to frictionless Slack input.
3.  **Faster Unblocking:** Time-to-resolution for stated "blockers" decreases, measured via the resolution of graph edges in the system.
4.  **Stable Team Health:** Sustained or improved team sentiment scores over a 6-month period.