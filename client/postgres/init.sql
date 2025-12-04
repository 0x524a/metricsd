-- Initialize test database with sample tables

-- Create extension for monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Sample tables for the test generator
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    customer_name VARCHAR(100) NOT NULL,
    product VARCHAR(100) NOT NULL,
    quantity INTEGER NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
    id SERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB,
    processed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS metrics_log (
    id SERIAL PRIMARY KEY,
    metric_name VARCHAR(100) NOT NULL,
    value DECIMAL(20, 4) NOT NULL,
    labels JSONB,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_processed ON events(processed);

-- Insert some initial data
INSERT INTO orders (customer_name, product, quantity, price, status) VALUES
    ('Alice', 'Widget A', 5, 29.99, 'completed'),
    ('Bob', 'Widget B', 3, 49.99, 'pending'),
    ('Charlie', 'Gadget X', 1, 199.99, 'processing');

