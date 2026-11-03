# Self-hosted deployment

Prepare private configuration for the hostname you will use:

~~~sh
deploy/configure.sh events.example.com
~~~

The script requires a POSIX shell, standard system utilities and OpenSSL. It
creates deploy/.env with mode 0600, three independent secrets and an initial API
key mapped to tenant default. It does not install software or contact a server.
Existing configuration is preserved; re-running against the same path fails.
Keep the file private and out of source control.

An optional second argument selects a configuration directory. localhost is
accepted for local TLS verification. Public deployment will require a hostname
pointing to the host and reachable gateway ports 80/443.
