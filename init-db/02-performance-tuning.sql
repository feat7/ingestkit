-- PostgreSQL Performance Tuning for High-Throughput Ingestion
-- This runs after 01-init.sql when container starts
-- Optimized for PostgreSQL 18 with high write workload

-- Enable parallel workers for table scans and index builds
ALTER DATABASE ingestkit SET max_parallel_workers_per_gather = 4;
ALTER DATABASE ingestkit SET max_parallel_workers = 8;
ALTER DATABASE ingestkit SET max_parallel_maintenance_workers = 4;

-- Optimize for write-heavy workload
ALTER DATABASE ingestkit SET synchronous_commit = 'off'; -- Fast writes, slight durability trade-off
ALTER DATABASE ingestkit SET wal_compression = 'on';     -- Compress WAL for better throughput
ALTER DATABASE ingestkit SET checkpoint_completion_target = 0.9;

-- Autovacuum tuning for high-throughput tables
ALTER DATABASE ingestkit SET autovacuum_naptime = '10s';              -- Check more frequently
ALTER DATABASE ingestkit SET autovacuum_vacuum_scale_factor = 0.05;   -- Vacuum at 5% dead tuples
ALTER DATABASE ingestkit SET autovacuum_analyze_scale_factor = 0.02;  -- Analyze at 2% changes
ALTER DATABASE ingestkit SET autovacuum_vacuum_cost_delay = 2;        -- Faster vacuum
ALTER DATABASE ingestkit SET autovacuum_vacuum_cost_limit = 1000;     -- Higher vacuum throughput

-- Enable JIT compilation for complex queries (PostgreSQL 18 improvement)
ALTER DATABASE ingestkit SET jit = 'on';
ALTER DATABASE ingestkit SET jit_above_cost = 100000;

-- Optimize planner for COPY operations
ALTER DATABASE ingestkit SET effective_io_concurrency = 200;  -- For SSD storage
ALTER DATABASE ingestkit SET random_page_cost = 1.1;          -- SSD optimization

-- Logging (minimal for performance)
ALTER DATABASE ingestkit SET log_statement = 'none';
ALTER DATABASE ingestkit SET log_duration = 'off';
ALTER DATABASE ingestkit SET log_checkpoints = 'on';          -- Keep checkpoint logs for monitoring

-- Connection settings
ALTER DATABASE ingestkit SET idle_in_transaction_session_timeout = '5min';
ALTER DATABASE ingestkit SET statement_timeout = '30s';

-- Success message
DO $$
BEGIN
    RAISE NOTICE '✓ PostgreSQL performance tuning applied';
    RAISE NOTICE '  • Parallel workers: 8 max, 4 per gather';
    RAISE NOTICE '  • Synchronous commit: OFF (high performance)';
    RAISE NOTICE '  • WAL compression: ON';
    RAISE NOTICE '  • Autovacuum: Optimized for high write throughput';
    RAISE NOTICE '  • JIT compilation: Enabled';
    RAISE NOTICE '  • I/O: Optimized for SSD';
END $$;
