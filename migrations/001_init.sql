CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  username VARCHAR(100) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  full_name VARCHAR(255) NOT NULL,
  role VARCHAR(20) NOT NULL DEFAULT 'collector',
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS markets (
  id SERIAL PRIMARY KEY,
  province VARCHAR(150) NOT NULL,
  district VARCHAR(150) NOT NULL,
  nks VARCHAR(150) NOT NULL,
  name VARCHAR(255) NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS commodity_categories (
  id SERIAL PRIMARY KEY,
  name VARCHAR(200) NOT NULL UNIQUE,
  type VARCHAR(100) NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS commodities (
  id SERIAL PRIMARY KEY,
  code VARCHAR(100) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  category_id INTEGER NOT NULL REFERENCES commodity_categories(id),
  brand_type VARCHAR(100),
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS units (
  id SERIAL PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  is_standard BOOLEAN NOT NULL DEFAULT FALSE,
  conversion_factor DECIMAL(18,8) NOT NULL DEFAULT 1,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_market_assignments (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id),
  market_id INTEGER NOT NULL REFERENCES markets(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, market_id)
);

CREATE TABLE IF NOT EXISTS data_entries (
  id SERIAL PRIMARY KEY,
  year INTEGER NOT NULL,
  market_id INTEGER NOT NULL REFERENCES markets(id),
  collector_id INTEGER NOT NULL REFERENCES users(id),
  category_id INTEGER NOT NULL REFERENCES commodity_categories(id),
  commodity_id INTEGER NOT NULL REFERENCES commodities(id),
  brand_type VARCHAR(200),
  local_unit_id INTEGER NOT NULL REFERENCES units(id),
  local_quantity DECIMAL(18,4) NOT NULL,
  local_weight_kg DECIMAL(18,4) NOT NULL,
  standard_unit_id INTEGER NOT NULL REFERENCES units(id),
  standard_quantity DECIMAL(18,4) NOT NULL,
  market_price DECIMAL(18,2) NOT NULL,
  minimum_price DECIMAL(18,2) NOT NULL,
  maximum_price DECIMAL(18,2) NOT NULL,
  previous_price DECIMAL(18,2) NOT NULL DEFAULT 0,
  converted_price DECIMAL(18,2) NOT NULL DEFAULT 0,
  warning_status VARCHAR(30) NOT NULL DEFAULT 'normal',
  notes TEXT,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id SERIAL PRIMARY KEY,
  entry_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL REFERENCES users(id),
  action VARCHAR(100) NOT NULL,
  before TEXT,
  after TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_data_entries_market_id ON data_entries (market_id);
CREATE INDEX IF NOT EXISTS idx_data_entries_collector_id ON data_entries (collector_id);
CREATE INDEX IF NOT EXISTS idx_data_entries_year ON data_entries (year);
CREATE INDEX IF NOT EXISTS idx_data_entries_warning_status ON data_entries (warning_status);
