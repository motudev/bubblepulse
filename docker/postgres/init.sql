-- Creates the test database alongside the primary one (bubblepulse is
-- already created by POSTGRES_DB in docker-compose.yml before this runs).
-- Runs automatically on first container start (docker-entrypoint-initdb.d).
CREATE DATABASE bubblepulse_test;

-- Dedicated non-superuser application role. Pooled multi-tenancy relies on
-- row-level security, which superusers and BYPASSRLS roles silently skip —
-- the app refuses to start in pooled mode when connected as one.
CREATE ROLE bubblepulse_app WITH LOGIN PASSWORD 'bubblepulse_app' NOSUPERUSER NOBYPASSRLS;
GRANT ALL PRIVILEGES ON DATABASE bubblepulse TO bubblepulse_app;
GRANT ALL PRIVILEGES ON DATABASE bubblepulse_test TO bubblepulse_app;
\connect bubblepulse
GRANT ALL ON SCHEMA public TO bubblepulse_app;
CREATE EXTENSION IF NOT EXISTS vector;
\connect bubblepulse_test
GRANT ALL ON SCHEMA public TO bubblepulse_app;
CREATE EXTENSION IF NOT EXISTS vector;
