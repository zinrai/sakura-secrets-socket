# sakura-secrets-socket

A server that hands out secrets from [SAKURA Cloud Secret Manager](https://cloud.sakura.ad.jp/products/secrets-manager/) by name on a Unix socket, and logs every request.

## Overview

`sakura-secrets-socket serve` holds the Secret Manager credentials in its own environment and answers `GET /<name>` with the value. It implements the socket contract of [zinrai/credstore](https://github.com/zinrai/credstore).

The socket is created with mode 0600, and a connection from another uid is refused. Values are returned as Secret Manager stores them; Secret Manager removes leading and trailing whitespace, a trailing newline included.

## Usage

```bash
$ sakura-secrets-socket serve -socket "$XDG_RUNTIME_DIR/secrets.sock"
```

The server runs until SIGINT or SIGTERM and removes the socket on exit.

```bash
$ curl --unix-socket "$XDG_RUNTIME_DIR/secrets.sock" http://localhost/web/db-password
```

| Status | Meaning |
|--------|---------|
| 200 | The value |
| 400 | The name is outside the contract: segments of letters, digits, `_` and `-` joined by `/` |
| 404 | No such secret in the Vault |
| 405 | Not a GET |
| 502 | Secret Manager failed; the error is in the log |

## Log

Every request is logged to stderr as one JSON object per line, whatever its status; a value never is. The start and stop of the server, and refused connections, are logged the same way.

```json
{"time":"2026-11-10T18:30:12.40+09:00","level":"INFO","msg":"request","name":"web/db-password","status":200,"version":3}
```

An error that stops the server before it listens is printed as `Error: ...` instead.

## Configuration

Set the Vault resource ID:

```bash
$ export SAKURA_SECRETS_ID="your-vault-resource-id"
```

and either static API keys:

```bash
$ export SAKURA_ACCESS_TOKEN="your-access-token"
$ export SAKURA_ACCESS_TOKEN_SECRET="your-access-token-secret"
```

or service principal credentials:

```bash
$ export SAKURA_SERVICE_PRINCIPAL_ID="your-service-principal-id"
$ export SAKURA_SERVICE_PRINCIPAL_KEY_ID="your-key-id"
$ export SAKURA_PRIVATE_KEY_PATH="/path/to/private-key.pem"
```

## License

This project is licensed under the [MIT License](./LICENSE).
