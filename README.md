# amazon-cli

`amazon-cli` provides `amzn`, a small command-line interface for Amazon.com
orders and products. It uses [Kernel](https://www.kernel.sh/) cloud browsers
and their [`browser curl`](https://www.kernel.sh/docs/browsers/curl) feature to
route requests through Chromium's network stack and inherit the cookies from an
authenticated browser profile.

## Features

- List Amazon orders by page, start index, and time filter
- Fetch details for a single order
- Search Amazon products and mark sponsored results
- Fetch details for a single product
- Reuse an authenticated Kernel browser profile
- Output human-readable tables, JSON, JSON Lines, YAML, or raw JSON
- Transform JSON output with GJSON expressions

## Installation

Install the latest version with Go:

```sh
go install github.com/rgarcia/amazon-cli/cmd/amzn@latest
```

Or build from a local checkout:

```sh
git clone https://github.com/rgarcia/amazon-cli.git
cd amazon-cli
make build
```

## Kernel Setup

You need a Kernel API key and a Kernel Managed Auth connection for Amazon.

1. Install and authenticate the Kernel CLI.

   ```sh
   npm install -g @onkernel/cli
   ```

2. Choose a Kernel profile name for Amazon, for example `amazon`, and create a
   Managed Auth connection for Amazon.

   ```sh
   kernel auth connections create --domain amazon.com --profile-name amazon
   ```

   Copy the returned connection ID.

3. Start the login flow.

   ```sh
   kernel auth connections login <connection-id>
   ```

4. Open the hosted URL printed by the command and complete the Amazon login
   flow. Kernel saves the authenticated session to the profile for future
   browser sessions.

## CLI Setup

Initialize `amzn` configuration:

```sh
amzn config init
```

During setup, provide either a Kernel browser profile ID or profile name. One of
these is required so `amzn` can create browser sessions with your Amazon login
state.

The config file is stored at:

```text
~/.config/amzn/config.yaml
```

CLI configuration is read from this file and command-line flags only.

Browser session state is cached under the standard local state directory:

```text
~/.local/state/amzn/browsers.json
```

## Usage

List the first page of orders for a year:

```sh
amzn orders list --time-filter year-2026
```

Amazon order history is page-oriented, so pagination is exposed directly:

```sh
amzn orders list --time-filter year-2026 --page 2
amzn orders list --time-filter year-2026 --start-index 20
```

Get a single order:

```sh
amzn orders get 111-1111111-1111111
```

By default, single-order details are shown in a human-readable detail view.

Search products:

```sh
amzn product search "distilled water"
```

Get a single product by ASIN:

```sh
amzn product get B087Z5WDJ2
```

By default, product search prints a table with a sponsored column, and product
details are shown in a human-readable detail view.

Use structured output:

```sh
amzn --output json orders list --time-filter year-2026
amzn --output json orders get 111-1111111-1111111
amzn --output json product search "wireless mouse"
amzn --output yaml product get B087Z5WDJ2
amzn --output yaml orders get 111-1111111-1111111
```

Use a GJSON transform:

```sh
amzn --output raw --transform 'orders.#' orders list --time-filter year-2026
```

`amzn` reuses the cached browser for the active config profile. If Kernel
reports that browser as missing, the CLI creates a new profile-backed browser
and updates the cache.

## Development

```sh
make test
make build
```
