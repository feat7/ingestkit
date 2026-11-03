\getenv writer_password POSTGRES_WRITER_PASSWORD
SELECT 'CREATE ROLE ingestkit_writer LOGIN' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='ingestkit_writer') \gexec
SELECT format('ALTER ROLE ingestkit_writer WITH PASSWORD %L NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION', :'writer_password') \gexec
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA ingestkit FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE ingestkit TO ingestkit_writer;
GRANT USAGE ON SCHEMA ingestkit TO ingestkit_writer;
GRANT SELECT, INSERT ON ingestkit.events TO ingestkit_writer;
